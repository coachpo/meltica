package ws

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

type binanceEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

func (w *WS) parsePublicMessage(msg *core.Message, raw []byte) error {
	payload := raw
	var envelope binanceEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && len(envelope.Data) > 0 {
		payload = envelope.Data
	}

	var meta struct {
		Event  string `json:"e"`
		Symbol string `json:"s"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil
	}

	symbol := w.canonicalSymbol(meta.Symbol)
	stream := envelope.Stream

	// Handle partial depth streams (no event type, different payload structure)
	if meta.Event == "" && stream != "" {
		// Extract channel from stream name (e.g., "btcusdt@depth20" -> "depth20")
		parts := strings.Split(stream, "@")
		if len(parts) >= 2 {
			channel := parts[1]
			if strings.HasPrefix(channel, "depth") {
				return w.parsePartialDepthStream(msg, payload, symbol, stream)
			}
		}
	}

	switch meta.Event {
	case "aggTrade":
		return w.parseTradeEvent(msg, payload, symbol, stream)
	case "bookTicker":
		return w.parseBookTicker(msg, payload, symbol, stream)
	case "depthUpdate":
		return w.parseDepthUpdate(msg, payload, symbol, stream)
	default:
		return w.parseByHeuristics(msg, payload, symbol, stream)
	}
}

func (w *WS) parseTradeEvent(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string `json:"s"`
		Price  string `json:"p"`
		Qty    string `json:"q"`
		Time   int64  `json:"T"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	sym := symbol
	if sym == "" {
		sym = w.canonicalSymbol(rec.Symbol)
	}
	if sym == "" {
		sym = rec.Symbol
	}
	msg.Topic = topicFromEvent("trade", sym)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "trade"
	price, _ := parseDecimalToRat(rec.Price)
	qty, _ := parseDecimalToRat(rec.Qty)
	msg.Parsed = &corews.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w *WS) parseBookTicker(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string `json:"s"`
		Bid    string `json:"b"`
		Ask    string `json:"a"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	sym := symbol
	if sym == "" {
		sym = w.canonicalSymbol(rec.Symbol)
	}
	if sym == "" {
		sym = rec.Symbol
	}
	msg.Topic = topicFromEvent("bookTicker", sym)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "ticker"
	bid, _ := parseDecimalToRat(rec.Bid)
	ask, _ := parseDecimalToRat(rec.Ask)
	msg.Parsed = &corews.TickerEvent{Symbol: sym, Bid: bid, Ask: ask, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w *WS) parseDepthUpdate(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol        string     `json:"s"`
		Time          int64      `json:"E"`
		FirstUpdateID int64      `json:"U"`
		LastUpdateID  int64      `json:"u"`
		Bids          [][]string `json:"b"`
		Asks          [][]string `json:"a"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	sym := symbol
	if sym == "" {
		sym = w.canonicalSymbol(rec.Symbol)
	}
	if sym == "" {
		sym = rec.Symbol
	}

	// Use order book management with Binance guidelines
	orderBook := w.orderBooks.GetOrCreateOrderBook(sym)
	updateTime := time.UnixMilli(rec.Time)

	// Process the depth update according to Binance guidelines
	err := orderBook.ProcessBookUpdate(rec.FirstUpdateID, rec.LastUpdateID, rec.Bids, rec.Asks, updateTime)
	if err != nil {
		// If there's a sequence mismatch or other error, we need to request a snapshot
		go w.handleOrderBookError(context.Background(), sym)
		return nil
	}

	// Check if we need to request a snapshot
	if orderBook.IsSnapshotPending() {
		go w.requestSnapshot(context.Background(), sym)
		return nil
	}

	// Get the complete order book snapshot
	completeSnapshot := orderBook.GetSnapshot()

	// Use the correct topic mapping for depthUpdate events
	msg.Topic = corews.BookTopic(sym)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "book"
	msg.Parsed = &completeSnapshot
	return nil
}

func (w *WS) parseByHeuristics(msg *core.Message, payload []byte, symbol, stream string) error {
	var probe map[string]any
	if err := json.Unmarshal(payload, &probe); err != nil {
		return nil
	}
	if hasString(probe, "p") && hasString(probe, "q") {
		return w.parseTradeEvent(msg, payload, symbol, stream)
	}
	if hasString(probe, "b") && hasString(probe, "a") {
		// Distinguish between bookTicker strings and depth arrays
		if _, ok := probe["b"].([]any); ok {
			return w.parseDepthUpdate(msg, payload, symbol, stream)
		}
		return w.parseBookTicker(msg, payload, symbol, stream)
	}
	return nil
}

func depthLevelsFromPairs(pairs [][]string) []corews.DepthLevel {
	levels := make([]corews.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		price, _ := parseDecimalToRat(pair[0])
		qty, _ := parseDecimalToRat(pair[1])
		levels = append(levels, corews.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}

// parsePartialDepthStream handles partial depth stream payloads (depth20)
func (w *WS) parsePartialDepthStream(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}

	// Parse depth levels
	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)

	// Create book event
	bookEvent := corews.BookEvent{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}

	msg.Topic = corews.BookTopic(symbol)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "book"
	msg.Parsed = &bookEvent
	return nil
}

func hasString(m map[string]any, key string) bool {
	v, ok := m[key]
	if !ok {
		return false
	}
	_, isString := v.(string)
	return isString
}

// handleOrderBookError handles order book synchronization errors
func (w *WS) handleOrderBookError(ctx context.Context, symbol string) {
	// Mark the order book as needing a snapshot
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	orderBook.mu.Lock()
	orderBook.SnapshotPending = true
	orderBook.mu.Unlock()

	// Request a fresh snapshot
	w.requestSnapshot(ctx, symbol)
}

// requestSnapshot requests a fresh snapshot from the Binance API
func (w *WS) requestSnapshot(ctx context.Context, symbol string) {
	// Retry logic with exponential backoff
	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		book, err := w.orderBooks.GetSnapshotFromAPI(ctx, symbol)
		if err == nil && book != nil {
			// Successfully got snapshot, process any buffered events
			book.mu.Lock()
			if len(book.Buffer) > 0 {
				book.processBufferedEvents()
			}
			book.mu.Unlock()
			return
		}

		// If we've exhausted retries, give up
		if attempt == maxRetries-1 {
			return
		}

		// Exponential backoff
		delay := baseDelay * time.Duration(1<<attempt)
		time.Sleep(delay)
	}
}
