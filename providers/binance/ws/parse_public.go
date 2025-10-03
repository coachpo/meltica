package ws

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

type binanceEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// parsePublicMessage routes Binance public WS payloads.
//
// Subscription mode:
//   - Only combined streams are supported. Incoming data is expected to be
//     wrapped as {"stream":"<streamName>", "data":{...}}.
//
// Routing rule (combined streams only):
//   - Route by substring in `stream`:
//     contains BNXBookDepthChannel (e.g. "@depth20@100ms") → parseBookStream
//     contains BNXTradeChannel      (e.g. "@trade")        → parseTradeEvent
//     contains BNXTickerChannel     (e.g. "@bookTicker")   → parseBookTicker
//     Any other streams are dropped (return nil).
//   - If `stream` is empty (non-combined payload), the message is dropped.
//
// Symbol flow:
//   - Trade/Ticker: prefer the `s` field in payload, canonicalized via
//     `WSCanonicalSymbol` (provided upfront here and again in handlers as needed).
//   - Partial depth snapshots: payload has no symbol; the handler extracts the
//     native symbol from `stream`, canonicalizes it, and errors if unresolved.
func (w *WS) parsePublicMessage(msg *core.Message, raw []byte) error {
	payload, stream := unwrapCombinedPayload(raw)

	var meta struct {
		Event  string `json:"e"`
		Symbol string `json:"s"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil
	}

	// Only canonicalize if we have a symbol
	// TODO: as far as I know all streams provide the symbol
	var symbol string
	if meta.Symbol != "" {
		symbol = w.WSCanonicalSymbol(meta.Symbol)
	} else if stream != "" {
		// Extract symbol from stream name (e.g., "btcusdt@trade" -> "BTCUSDT")
		if native := streamSymbol(stream); native != "" {
			symbol = w.WSCanonicalSymbol(native)
		}
	}

	// Strict policy: require symbol for all messages
	if symbol == "" {
		fmt.Fprintf(os.Stderr, "ERROR: binance ws: missing symbol in message; stream=%s, event=%s, meta.Symbol=%s\n", stream, meta.Event, meta.Symbol)
		return fmt.Errorf("binance ws: missing symbol in message; stream=%s, event=%s", stream, meta.Event)
	}

	if stream != "" {
		switch {
		case strings.Contains(stream, BNXBookDepthChannel):
			return w.parseBookStream(msg, payload, symbol, stream)
		case strings.Contains(stream, BNXTradeChannel):
			return w.parseTradeEvent(msg, payload, symbol, stream)
		case strings.Contains(stream, BNXTickerChannel):
			return w.parseBookTicker(msg, payload, symbol, stream)
		default:
			return nil
		}
	}

	return nil
}

// parseTradeEvent parses a single trade event.
//
// Symbol flow:
//   - Prefers the symbol supplied by `parsePublicMessage` so combined streams
//     keep sharing the same canonical identifier.
//   - When the caller cannot resolve the symbol, the function falls back to the
//     payload's raw symbol, canonicalizes it, and errors if the result is still
//     empty (prevents silent routing gaps).
//
// Function behavior:
//   - Produces a `corews.TradeEvent` populated with rational prices/quantities
//     and stamps the canonical topic via `topicFromChannel`.
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
		sym = w.WSCanonicalSymbol(rec.Symbol)
	}
	if sym == "" {
		return fmt.Errorf("binance ws trade: missing symbol; stream=%s", stream)
	}
	topic := topicFromChannel(BNXTradeChannel, sym)
	msg.Topic = topic
	msg.Event = corews.TopicTrade
	price, _ := parseDecimalToRat(rec.Price)
	qty, _ := parseDecimalToRat(rec.Qty)
	msg.Parsed = &corews.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

// parseBookTicker parses best bid/ask updates.
//
// Symbol flow:
//   - Leverages the caller-provided canonical symbol whenever possible so the
//     topic emitted here lines up with trade/other book feeds.
//   - If the caller could not determine the symbol, the handler inspects the
//     payload, canonicalizes it, and errors when resolution fails. The error is
//     surfaced so the caller can log/alert instead of routing an ambiguous
//     message.
//
// Function behavior:
//   - Emits a `corews.TickerEvent` tagged with `corews.TopicTicker` and parsed
//     rational bid/ask levels.
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
		sym = w.WSCanonicalSymbol(rec.Symbol)
	}
	if sym == "" {
		return fmt.Errorf("binance ws bookTicker: missing symbol; stream=%s", stream)
	}
	topic := topicFromChannel(BNXTickerChannel, sym)
	msg.Topic = topic
	msg.Event = corews.TopicTicker
	bid, _ := parseDecimalToRat(rec.Bid)
	ask, _ := parseDecimalToRat(rec.Ask)
	msg.Parsed = &corews.TickerEvent{Symbol: sym, Bid: bid, Ask: ask, Time: time.UnixMilli(rec.Time)}
	return nil
}

// depthLevelsFromPairs converts [price, quantity] string pairs into structured
// depth levels using rational number parsing. Invalid pairs are skipped.
func depthLevelsFromPairs(pairs [][]interface{}) []corews.DepthLevel {
	levels := make([]corews.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var pStr, qStr string
		switch v := pair[0].(type) {
		case string:
			pStr = v
		default:
			pStr = fmt.Sprint(v)
		}
		switch v := pair[1].(type) {
		case string:
			qStr = v
		default:
			qStr = fmt.Sprint(v)
		}
		price, _ := parseDecimalToRat(pStr)
		qty, _ := parseDecimalToRat(qStr)
		levels = append(levels, corews.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}

// parseBookStream handles depth updates like `<binanceSymbol>@depth@100ms`.
//
// According to Binance documentation:
// - These are incremental updates to the order book
// - They contain U (first update ID) and u (last update ID) for synchronization
// - The order book must be initialized with a snapshot before applying these updates
//
// Symbol flow:
//   - These payloads include a symbol field, but we also extract from stream name as fallback
//
// Function behavior:
//   - If the order book is not initialized, buffers the event
//   - If the order book is initialized, applies the update directly
//   - Returns an error if out of sync (needs restart with snapshot)
func (w *WS) parseBookStream(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		Event         string          `json:"e"`
		Symbol        string          `json:"s"`
		FirstUpdateID int64           `json:"U"`
		LastUpdateID  int64           `json:"u"`
		Bids          [][]interface{} `json:"b"`
		Asks          [][]interface{} `json:"a"`
		EventTime     int64           `json:"E"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}

	// Validate event type
	if rec.Event != "depthUpdate" {
		return fmt.Errorf("binance ws: unexpected event type in depth stream: %s", rec.Event)
	}

	// If symbol is not set, try to extract it from the stream name
	if symbol == "" && stream != "" {
		if native := streamSymbol(stream); native != "" {
			symbol = w.WSCanonicalSymbol(native)
		}
	}

	// Fallback to payload symbol if still not resolved
	if symbol == "" && rec.Symbol != "" {
		symbol = w.WSCanonicalSymbol(rec.Symbol)
	}

	// Parse depth levels
	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)

	// Enforce symbol presence. If we cannot determine it, error out so callers can log/crash.
	if symbol == "" {
		fmt.Fprintf(os.Stderr, "ERROR: binance ws partial depth: missing symbol; stream=%s, rec.Symbol=%s\n", stream, rec.Symbol)
		return fmt.Errorf("binance ws partial depth: missing symbol; stream=%s", stream)
	}

	// Get the order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	// Convert event time to time.Time
	eventTime := time.UnixMilli(rec.EventTime)

	// Buffer or apply the event based on initialization state
	orderBook.BufferEvent(rec.FirstUpdateID, rec.LastUpdateID, bids, asks, eventTime)

	// Update the message topic and payload
	msg.Topic = corews.BookTopic(symbol)
	msg.Event = corews.TopicBook
	msg.Parsed = &corews.BookEvent{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   eventTime,
	}

	return nil
}

func unwrapCombinedPayload(raw []byte) ([]byte, string) {
	payload := raw
	var envelope binanceEnvelope
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Stream != "" && len(envelope.Data) > 0 {
		return envelope.Data, envelope.Stream
	}
	return payload, ""
}

func streamSymbol(stream string) string {
	if stream == "" {
		return ""
	}
	parts := strings.SplitN(stream, "@", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
