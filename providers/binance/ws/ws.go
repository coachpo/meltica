package ws

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/gorilla/websocket"
)

const (
	publicStreamURL    = "wss://stream.binance.com:9443/stream"
	wsHandshakeTimeout = 10 * time.Second
)

// Provider interface defines what the ws package needs from the main provider
type Provider interface {
	// For symbol conversion
	CanonicalSymbol(binanceSymbol string) string
	// For private streams
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
	// For order book snapshots
	DepthSnapshot(ctx context.Context, symbol string, limit int) (corews.BookEvent, int64, error)
}

// WS implements the core.WS interface for Binance
type WS struct {
	p             Provider
	orderBooks    *OrderBookManager
	monitorCtx    context.Context
	monitorCancel context.CancelFunc
}

// New creates a new WebSocket handler for Binance
func New(p Provider) *WS {
	ctx, cancel := context.WithCancel(context.Background())
	return &WS{
		p:             p,
		orderBooks:    NewOrderBookManager(),
		monitorCtx:    ctx,
		monitorCancel: cancel,
	}
}

type wsSub struct {
	c    chan core.Message
	err  chan error
	conn *websocket.Conn
}

func (s *wsSub) C() <-chan core.Message { return s.c }
func (s *wsSub) Err() <-chan error      { return s.err }

func (s *wsSub) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		c:    make(chan core.Message, 1024),
		err:  make(chan error, 1),
		conn: conn,
	}
}

func (w *WS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	streams := w.buildStreams(topics)
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams derived from topics")
	}
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	endpoint := publicStreamURL + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(conn)

	// Start monitoring for order book symbols
	bookSymbols := w.extractBookSymbols(topics)
	if len(bookSymbols) > 0 {
		w.startMonitoring(bookSymbols)
	}

	go w.readLoop(sub)
	return sub, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (w *WS) WSNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance ws: unsupported canonical symbol %s", canonical))
}

func (w *WS) WSCanonicalSymbol(native string) string {
	return w.p.CanonicalSymbol(native)
}

func (w *WS) buildStreams(topics []string) []string {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := corews.ParseTopic(topic)
		providerChannel := mapper.ToProviderChannel(channel)
		if instrument == "" {
			streams = append(streams, topic)
			continue
		}
		if providerChannel == "" {
			providerChannel = strings.ToLower(channel)
		}
		binanceSymbol := strings.ToLower(core.CanonicalToBinance(instrument))
		streams = append(streams, binanceSymbol+"@"+providerChannel)
	}
	return streams
}

func (w *WS) readLoop(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now()}
		if err := w.parsePublicMessage(&msg, data); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

// InitializeOrderBook demonstrates the complete Binance order book initialization flow
// This would typically be called when starting to subscribe to depth streams
func (w *WS) InitializeOrderBook(ctx context.Context, symbol string) error {
	// Step 1: Get order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	// Step 2: If already initialized, no need to do anything
	if orderBook.IsInitialized() {
		return nil
	}

	// Step 3: Get depth snapshot from REST API
	// Note: In a real implementation, we would need access to the provider's REST client
	// For now, this demonstrates the flow
	snapshot, lastUpdateID, err := w.getDepthSnapshot(ctx, symbol, 5000)
	if err != nil {
		return fmt.Errorf("failed to get depth snapshot: %w", err)
	}

	// Step 4: Initialize order book with snapshot
	if err := orderBook.InitializeFromSnapshot(snapshot, lastUpdateID); err != nil {
		return fmt.Errorf("failed to initialize order book: %w", err)
	}

	return nil
}

// OrderBookSnapshot returns the current snapshot for a symbol if it has been initialized.
func (w *WS) OrderBookSnapshot(symbol string) (corews.BookEvent, bool) {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if !orderBook.IsInitialized() {
		return corews.BookEvent{}, false
	}
	return orderBook.GetSnapshot(), true
}

// getDepthSnapshot fetches the actual depth snapshot from the REST API
func (w *WS) getDepthSnapshot(ctx context.Context, symbol string, limit int) (corews.BookEvent, int64, error) {
	return w.p.DepthSnapshot(ctx, symbol, limit)
}

// initializeAndRecover implements the complete Binance order book initialization flow
// with automatic retry logic when gaps are detected
func (w *WS) initializeAndRecover(symbol string, firstUpdateID, lastUpdateID int64) {
	log.Printf("Starting automatic order book recovery for %s (gap detected: firstUpdateID=%d, lastUpdateID=%d)",
		symbol, firstUpdateID, lastUpdateID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Track first event U for validation (if available)
	firstEventU := firstUpdateID
	if firstEventU == 0 {
		// If no specific gap detected, just initialize normally
		firstEventU = -1
	}

	retryCount := 0
	maxRetries := 5

	for retryCount < maxRetries {
		retryCount++

		if err := w.InitializeOrderBook(ctx, symbol); err != nil {
			log.Printf("Failed to initialize order book for %s (attempt %d/%d): %v",
				symbol, retryCount, maxRetries, err)
			if retryCount < maxRetries {
				time.Sleep(time.Duration(retryCount) * time.Second)
				continue
			}
			log.Printf("Giving up on order book recovery for %s after %d attempts", symbol, maxRetries)
			return
		}

		// Validate snapshot against first event U if we have a specific gap
		orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
		if firstEventU > 0 && orderBook.GetLastUpdateID() < firstEventU {
			// Snapshot too old, retry
			log.Printf("Snapshot too old for %s (snapshot=%d < firstEvent=%d), retrying...",
				symbol, orderBook.GetLastUpdateID(), firstEventU)
			if retryCount < maxRetries {
				time.Sleep(time.Duration(retryCount) * time.Second)
				continue
			}
			log.Printf("Giving up on order book recovery for %s - cannot get recent enough snapshot", symbol)
			return
		}

		// Successfully initialized
		log.Printf("Successfully recovered order book for %s (snapshot update ID: %d)",
			symbol, orderBook.GetLastUpdateID())
		return
	}
}

// validateOrderBookState periodically checks if the order book is healthy
func (w *WS) validateOrderBookState(symbol string) {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if !orderBook.IsInitialized() {
		log.Printf("Order book for %s is not initialized, triggering recovery", symbol)
		go w.initializeAndRecover(symbol, 0, 0)
		return
	}

	// Check if order book hasn't been updated recently (stale)
	if time.Since(orderBook.LastUpdate) > 2*time.Minute {
		log.Printf("Order book for %s is stale (last update: %v), reinitializing",
			symbol, orderBook.LastUpdate)
		go w.initializeAndRecover(symbol, 0, 0)
	}
}

// startMonitoring begins periodic health checks for order books
func (w *WS) startMonitoring(symbols []string) {
	if len(symbols) == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute) // Check every minute
		defer ticker.Stop()

		for {
			select {
			case <-w.monitorCtx.Done():
				return
			case <-ticker.C:
				for _, symbol := range symbols {
					w.validateOrderBookState(symbol)
				}
			}
		}
	}()
}

// Close implements the core.WS interface
func (w *WS) Close() error {
	if w.monitorCancel != nil {
		w.monitorCancel()
	}
	return nil
}

// extractBookSymbols identifies symbols that have order book subscriptions
func (w *WS) extractBookSymbols(topics []string) []string {
	var symbols []string
	for _, topic := range topics {
		channel, symbol := corews.ParseTopic(topic)
		if channel == corews.TopicBook && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}
