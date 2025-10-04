package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/gorilla/websocket"
)

const (
	depthStreamURL         = "wss://stream.binance.com:9443/ws/btcusdt@depth@100ms"
	depthSnapshotURL       = "https://api.binance.com/api/v3/depth?symbol=BTCUSDT&limit=5000"
	displaySymbol          = "BTCUSDT"
	desiredLevelsPerSide   = 5000
	minimumRequiredPerSide = 1
	displayLevels          = 50
	renderInterval         = 200 * time.Millisecond
)

type depthEvent struct {
	EventType       string          `json:"e"`
	EventTime       int64           `json:"E"`
	Symbol          string          `json:"s"`
	FirstUpdateID   int64           `json:"U"`
	FinalUpdateID   int64           `json:"u"`
	PreviousFinalID int64           `json:"pu"`
	Bids            [][]interface{} `json:"b"`
	Asks            [][]interface{} `json:"a"`
}

type depthSnapshot struct {
	LastUpdateID int64           `json:"lastUpdateId"`
	Bids         [][]interface{} `json:"bids"`
	Asks         [][]interface{} `json:"asks"`
}

type orderBook struct {
	bids         map[string]*big.Rat
	asks         map[string]*big.Rat
	lastUpdateID int64
	lastUpdate   time.Time
}

func main() {
	if err := runValidation(); err != nil {
		log.Fatalf("❌ Binance order book validation failed: %v", err)
	}
}

func runValidation() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")
	fmt.Println("Connecting to depth stream and buffering events...")

	conn, err := connectDepthStream(ctx)
	if err != nil {
		return fmt.Errorf("open depth stream: %w", err)
	}
	defer conn.Close()

	readCtx, stopReaders := context.WithCancel(ctx)
	defer stopReaders()

	eventsCh := make(chan depthEvent, 4096)
	errCh := make(chan error, 1)
	go streamDepthEvents(readCtx, conn, eventsCh, errCh)

	buffer := make([]depthEvent, 0, 2048)
	firstEvent, err := waitForFirstEvent(ctx, eventsCh, errCh, &buffer)
	if err != nil {
		return err
	}
	fmt.Printf("Buffered first depth event range: U=%d, u=%d\n", firstEvent.FirstUpdateID, firstEvent.FinalUpdateID)

	client := &http.Client{Timeout: 10 * time.Second}

	var snapshot depthSnapshot
	for {
		if err := takeStreamError(errCh); err != nil {
			return err
		}
		drainEvents(eventsCh, &buffer)

		snapCtx, cancelSnap := context.WithTimeout(ctx, 10*time.Second)
		snapshot, err = fetchDepthSnapshot(snapCtx, client)
		cancelSnap()
		if err != nil {
			return fmt.Errorf("fetch depth snapshot: %w", err)
		}
		fmt.Printf("Fetched snapshot with lastUpdateId=%d\n", snapshot.LastUpdateID)
		if snapshot.LastUpdateID >= firstEvent.FirstUpdateID {
			break
		}
		fmt.Println("Snapshot is older than buffered range, retrying...")
		time.Sleep(200 * time.Millisecond)
	}

	drainEvents(eventsCh, &buffer)
	if err := takeStreamError(errCh); err != nil {
		return err
	}

	buffer = dropOldEvents(buffer, snapshot.LastUpdateID)
	if err := ensureBufferReady(ctx, eventsCh, errCh, &buffer, snapshot.LastUpdateID); err != nil {
		return err
	}
	fmt.Printf("Buffered %d events ready for application\n", len(buffer))

	ob := &orderBook{}
	if err := ob.loadSnapshot(snapshot); err != nil {
		return fmt.Errorf("initialize local order book: %w", err)
	}
	fmt.Printf("Snapshot depth: %d bids, %d asks (update ID %d)\n", ob.bidCount(), ob.askCount(), ob.lastID())

	if err := validateSnapshotDepth(ob); err != nil {
		return err
	}

	if err := applyBufferedEvents(ob, buffer); err != nil {
		return err
	}

	displayOrderBook(ob)

	if err := runEventLoop(ctx, ob, eventsCh, errCh, client); err != nil {
		return err
	}

	return nil
}

func connectDepthStream(ctx context.Context) (*websocket.Conn, error) {
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, depthStreamURL, nil)
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func streamDepthEvents(ctx context.Context, conn *websocket.Conn, out chan<- depthEvent, errCh chan<- error) {
	defer close(out)
	for {
		if deadlineErr := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); deadlineErr != nil {
			sendErr(errCh, deadlineErr)
			return
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if !isNormalClose(err) {
				sendErr(errCh, err)
			}
			return
		}

		var ev depthEvent
		if decodeErr := json.Unmarshal(data, &ev); decodeErr != nil {
			sendErr(errCh, fmt.Errorf("decode depth event: %w", decodeErr))
			return
		}
		if ev.EventType != "depthUpdate" {
			continue
		}

		select {
		case out <- ev:
		case <-ctx.Done():
			return
		}
	}
}

func waitForFirstEvent(parent context.Context, eventsCh <-chan depthEvent, errCh <-chan error, buffer *[]depthEvent) (depthEvent, error) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return depthEvent{}, errors.New("timed out waiting for initial depth event")
		case err := <-errCh:
			return depthEvent{}, err
		case ev, ok := <-eventsCh:
			if !ok {
				return depthEvent{}, errors.New("depth stream closed before initial event")
			}
			*buffer = append(*buffer, ev)
			return ev, nil
		}
	}
}

func fetchDepthSnapshot(ctx context.Context, client *http.Client) (depthSnapshot, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, depthSnapshotURL, nil)
	if err != nil {
		return depthSnapshot{}, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return depthSnapshot{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return depthSnapshot{}, fmt.Errorf("snapshot request failed: status=%d body=%s", resp.StatusCode, string(body))
	}

	var snap depthSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return depthSnapshot{}, err
	}
	return snap, nil
}

func (ob *orderBook) loadSnapshot(snapshot depthSnapshot) error {
	ob.bids = make(map[string]*big.Rat, len(snapshot.Bids))
	ob.asks = make(map[string]*big.Rat, len(snapshot.Asks))
	ob.lastUpdateID = snapshot.LastUpdateID
	ob.lastUpdate = time.Now()

	if err := applyLevels(ob.bids, snapshot.Bids); err != nil {
		return err
	}
	if err := applyLevels(ob.asks, snapshot.Asks); err != nil {
		return err
	}
	return nil
}

func runEventLoop(ctx context.Context, ob *orderBook, eventsCh <-chan depthEvent, errCh <-chan error, client *http.Client) error {
	renderTicker := time.NewTicker(renderInterval)
	defer renderTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			displayOrderBook(ob)
			return nil
		case err := <-errCh:
			if err != nil {
				return err
			}
		case ev, ok := <-eventsCh:
			if !ok {
				return errors.New("depth stream closed")
			}
			applied, applyErr := ob.applyEvent(ev)
			if applyErr != nil {
				if errors.Is(applyErr, errGapDetected) {
					if recErr := recoverOrderBook(ctx, ob, ev, eventsCh, errCh, client); recErr != nil {
						return recErr
					}
					continue
				}
				return applyErr
			}
			if applied {
				displayOrderBook(ob)
			}
		case <-renderTicker.C:
			displayOrderBook(ob)
		}
	}
}

func recoverOrderBook(ctx context.Context, ob *orderBook, firstGap depthEvent, eventsCh <-chan depthEvent, errCh <-chan error, client *http.Client) error {
	log.Println("Gap detected, resynchronizing order book...")
	buffer := make([]depthEvent, 0, 4096)
	buffer = append(buffer, firstGap)
	firstRangeStart := firstGap.FirstUpdateID

	for {
		if err := takeStreamError(errCh); err != nil {
			return err
		}
		drainEvents(eventsCh, &buffer)

		snapCtx, cancelSnap := context.WithTimeout(ctx, 10*time.Second)
		snapshot, err := fetchDepthSnapshot(snapCtx, client)
		cancelSnap()
		if err != nil {
			return fmt.Errorf("recover: fetch depth snapshot: %w", err)
		}

		if snapshot.LastUpdateID < firstRangeStart {
			log.Printf("Snapshot too old during recovery (snapshot=%d < gap start=%d), retrying...", snapshot.LastUpdateID, firstRangeStart)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		if err := ob.loadSnapshot(snapshot); err != nil {
			return fmt.Errorf("recover: load snapshot: %w", err)
		}

		if err := validateSnapshotDepth(ob); err != nil {
			log.Printf("Snapshot depth issue during recovery: %v", err)
			time.Sleep(200 * time.Millisecond)
			continue
		}

		buffer = dropOldEvents(buffer, snapshot.LastUpdateID)
		if err := ensureBufferReady(ctx, eventsCh, errCh, &buffer, snapshot.LastUpdateID); err != nil {
			return fmt.Errorf("recover: ensure buffer ready: %w", err)
		}

		if err := applyBufferedEvents(ob, buffer); err != nil {
			return fmt.Errorf("recover: apply buffered events: %w", err)
		}

		displayOrderBook(ob)
		log.Println("Order book resynchronized successfully")
		return nil
	}
}

func displayOrderBook(ob *orderBook) {
	bids := topLevels(ob.bids, displayLevels, true)
	asks := topLevels(ob.asks, displayLevels, false)

	fmt.Print("\033[H\033[2J")
	fmt.Printf("Binance %s Depth | UpdateID %d | Bids %d | Asks %d\n", displaySymbol, ob.lastID(), ob.bidCount(), ob.askCount())
	if !ob.lastUpdate.IsZero() {
		fmt.Printf("Last update: %s\n", ob.lastUpdate.Format(time.RFC3339Nano))
	}
	fmt.Printf("%-18s %-18s | %-18s %-18s\n", "BidQty", "BidPrice", "AskPrice", "AskQty")

	rows := len(bids)
	if len(asks) > rows {
		rows = len(asks)
	}
	if rows < displayLevels {
		rows = displayLevels
	}

	for i := 0; i < rows; i++ {
		bidQty, bidPrice := "-", "-"
		askPrice, askQty := "-", "-"
		if i < len(bids) {
			bidQty = formatRat(bids[i].qty, 6)
			bidPrice = formatRat(bids[i].price, 8)
		}
		if i < len(asks) {
			askPrice = formatRat(asks[i].price, 8)
			askQty = formatRat(asks[i].qty, 6)
		}
		fmt.Printf("%-18s %-18s | %-18s %-18s\n", bidQty, bidPrice, askPrice, askQty)
	}
}

type bookLevel struct {
	price *big.Rat
	qty   *big.Rat
}

func topLevels(book map[string]*big.Rat, limit int, desc bool) []bookLevel {
	levels := make([]bookLevel, 0, len(book))
	for priceStr, qty := range book {
		price, ok := new(big.Rat).SetString(priceStr)
		if !ok {
			continue
		}
		levels = append(levels, bookLevel{
			price: price,
			qty:   new(big.Rat).Set(qty),
		})
	}
	if len(levels) == 0 {
		return levels
	}
	sort.Slice(levels, func(i, j int) bool {
		cmp := levels[i].price.Cmp(levels[j].price)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if len(levels) > limit {
		levels = levels[:limit]
	}
	return levels
}

func formatRat(r *big.Rat, precision int) string {
	if r == nil {
		return "0"
	}
	return r.FloatString(precision)
}

func applyBufferedEvents(ob *orderBook, events []depthEvent) error {
	for idx, ev := range events {
		applied, err := ob.applyEvent(ev)
		if err != nil {
			return fmt.Errorf("buffered event %d: %w", idx, err)
		}
		if !applied {
			continue
		}
	}
	return nil
}

func applyLevels(book map[string]*big.Rat, levels [][]interface{}) error {
	for _, raw := range levels {
		if len(raw) < 2 {
			continue
		}
		priceStr := fmt.Sprint(raw[0])
		qtyStr := fmt.Sprint(raw[1])

		qty, err := parseDecimal(qtyStr)
		if err != nil {
			return fmt.Errorf("parse quantity %q: %w", qtyStr, err)
		}

		if qty.Sign() == 0 {
			delete(book, priceStr)
			continue
		}

		book[priceStr] = qty
	}
	return nil
}

func parseDecimal(value string) (*big.Rat, error) {
	r := new(big.Rat)
	if _, ok := r.SetString(value); !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	return r, nil
}

var errGapDetected = errors.New("order book gap detected")

func (ob *orderBook) applyEvent(ev depthEvent) (bool, error) {
	if ev.FinalUpdateID <= ob.lastUpdateID {
		return false, nil
	}
	if ev.FirstUpdateID > ob.lastUpdateID+1 {
		return false, fmt.Errorf("%w: local=%d, event=[%d,%d]", errGapDetected, ob.lastUpdateID, ev.FirstUpdateID, ev.FinalUpdateID)
	}
	if err := applyLevels(ob.bids, ev.Bids); err != nil {
		return false, err
	}
	if err := applyLevels(ob.asks, ev.Asks); err != nil {
		return false, err
	}
	ob.lastUpdateID = ev.FinalUpdateID
	if ev.EventTime > 0 {
		ob.lastUpdate = time.UnixMilli(ev.EventTime)
	} else {
		ob.lastUpdate = time.Now()
	}
	return true, nil
}

func validateSnapshotDepth(ob *orderBook) error {
	bids := ob.bidCount()
	asks := ob.askCount()
	if bids < minimumRequiredPerSide || asks < minimumRequiredPerSide {
		return fmt.Errorf("depth snapshot missing required side: bids=%d asks=%d", bids, asks)
	}
	if bids < desiredLevelsPerSide || asks < desiredLevelsPerSide {
		fmt.Printf("Snapshot depth below Binance limit but acceptable: bids=%d asks=%d\n", bids, asks)
	} else {
		fmt.Printf("Snapshot depth validated: bids=%d asks=%d\n", bids, asks)
	}
	return nil
}

func (ob *orderBook) bidCount() int { return len(ob.bids) }
func (ob *orderBook) askCount() int { return len(ob.asks) }
func (ob *orderBook) lastID() int64 { return ob.lastUpdateID }

func dropOldEvents(events []depthEvent, lastUpdateID int64) []depthEvent {
	idx := 0
	for idx < len(events) && events[idx].FinalUpdateID <= lastUpdateID {
		idx++
	}
	if idx == 0 {
		return events
	}
	trimmed := make([]depthEvent, len(events)-idx)
	copy(trimmed, events[idx:])
	return trimmed
}

func ensureBufferReady(parent context.Context, eventsCh <-chan depthEvent, errCh <-chan error, buffer *[]depthEvent, snapshotLastID int64) error {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	target := snapshotLastID + 1
	for {
		if bufferHasTarget(*buffer, target) {
			return nil
		}
		select {
		case <-ctx.Done():
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("timed out waiting for buffered event covering snapshot id %d", snapshotLastID)
			}
			return ctx.Err()
		case err := <-errCh:
			return err
		case ev, ok := <-eventsCh:
			if !ok {
				return errors.New("depth stream closed before applying buffered events")
			}
			*buffer = append(*buffer, ev)
			*buffer = dropOldEvents(*buffer, snapshotLastID)
		}
	}
}

func bufferHasTarget(events []depthEvent, target int64) bool {
	if len(events) == 0 {
		return false
	}
	first := events[0]
	return target >= first.FirstUpdateID && target <= first.FinalUpdateID
}

func drainEvents(eventsCh <-chan depthEvent, buffer *[]depthEvent) {
	for {
		select {
		case ev, ok := <-eventsCh:
			if !ok {
				return
			}
			*buffer = append(*buffer, ev)
		default:
			return
		}
	}
}

func takeStreamError(errCh <-chan error) error {
	select {
	case err := <-errCh:
		if err == nil {
			return nil
		}
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	default:
		return nil
	}
}

func sendErr(ch chan<- error, err error) {
	select {
	case ch <- err:
	default:
	}
}

func isNormalClose(err error) bool {
	if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
		return true
	}
	return false
}
