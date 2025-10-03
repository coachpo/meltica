package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

type binanceEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

// parsePublicMessage is the entry point for Binance public WS payloads.
//
// Symbol flow:
//   - Combined streams may carry the event inside an outer envelope
//     {"stream": "<binanceSymbol>@<channel>", "data": {...}}. The method
//     unwraps that envelope and immediately canonicalizes the upstream
//     symbol code via `WSCanonicalSymbol`.
//   - If the payload already includes a symbol (e.g. trade/ticker events), that
//     value wins and is canonicalized, producing the authoritative symbol that
//     gets propagated to downstream parsers.
//   - When the payload omits a symbol (e.g. partial depth snapshots), the method
//     reconstructs it from the stream name and again canonicalizes it. Unknown
//     symbols bubble up as panics through the provider’s canonicalizer.
//
// Function behavior:
//   - After the symbol pipeline settles, the function delegates to
//     event-specific handlers. Each handler receives the resolved symbol along
//     with the raw stream name so it can fallback if needed.
//   - If the event type cannot be determined, the message is intentionally
//     dropped (nil error) because the data does not map to any supported topic.
func (w *WS) parsePublicMessage(msg *core.Message, raw []byte) error {
	payload, stream := unwrapCombinedPayload(raw)
	channel := streamChannel(stream)

	var meta struct {
		Event  string `json:"e"`
		Symbol string `json:"s"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil
	}

	symbol := w.WSCanonicalSymbol(meta.Symbol)

	// Handle event-based routing first
	switch meta.Event {
	case BNXTradeChannel:
		return w.parseTradeEvent(msg, payload, symbol, stream)
	}

	// Handle channel-based routing
	switch channel {
	case BNXTickerChannel:
		return w.parseBookTicker(msg, payload, symbol, stream)
	case BNXBookDepthChannel:
		return w.parseBookStream(msg, payload, symbol, stream)
	}

	// Some feeds reuse the channel name as the event type (e.g. futures book ticker).
	switch meta.Event {
	case BNXTickerChannel:
		return w.parseBookTicker(msg, payload, symbol, stream)
	case BNXBookDepthChannel:
		return w.parseBookStream(msg, payload, symbol, stream)
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

// parseBookStream handles partial depth snapshots like
// `<binanceSymbol>@depth<levels>`.
//
// Symbol flow:
//   - These payloads never include a symbol field, so the handler rebuilds the
//     Binance symbol from the stream name, canonicalizes it, and stores the
//     canonical value back in `msg.Topic`. Unknown symbols panic via the
//     provider’s canonicalizer.
//
// Function behavior:
//   - Converts bid/ask string pairs into rational depth levels and packages them
//     into a `corews.BookEvent` stamped with `corews.TopicBook`.
//   - Uses `time.Now()` because Binance does not supply a timestamp for partial
//     depth snapshots.
func (w *WS) parseBookStream(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}

	// If symbol is not set, try to extract it from the stream name
	if symbol == "" && stream != "" {
		if native := streamSymbol(stream); native != "" {
			symbol = w.WSCanonicalSymbol(native)
		}
	}

	// Parse depth levels
	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)

	// Enforce symbol presence. If we cannot determine it, error out so callers can log/crash.
	if symbol == "" {
		return fmt.Errorf("binance ws partial depth: missing symbol; stream=%s", stream)
	}

	// Create book event
	bookEvent := corews.BookEvent{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}

	msg.Topic = corews.BookTopic(symbol)
	msg.Event = corews.TopicBook
	msg.Parsed = &bookEvent
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

func streamChannel(stream string) string {
	if stream == "" {
		return ""
	}
	parts := strings.SplitN(stream, "@", 2)
	if len(parts) < 2 {
		return ""
	}
	return parts[1]
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
