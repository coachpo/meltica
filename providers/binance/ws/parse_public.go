package ws

import (
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
		Time   int64  `json:"E"`
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
			// If symbol is not set, try to extract it from the stream name
			if symbol == "" {
				binanceSymbol := parts[0]
				if binanceSymbol != "" {
					canonical := w.canonicalSymbol(strings.ToUpper(binanceSymbol))
					if canonical != "" {
						symbol = canonical
					} else {
						symbol = strings.ToUpper(binanceSymbol)
					}
				}
			}
			if strings.HasPrefix(channel, "depth") {
				return w.parsePartialDepthStream(msg, payload, symbol, stream)
			}
		}
	}

	switch meta.Event {
	case "trade":
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

	// Parse depth levels directly without local order book management
	updateTime := time.UnixMilli(rec.Time)
	bids := make([]corews.DepthLevel, 0, len(rec.Bids))
	asks := make([]corews.DepthLevel, 0, len(rec.Asks))

	// Parse bids
	for _, bid := range rec.Bids {
		if len(bid) < 2 {
			continue
		}
		if price, ok := parseDecimalToRat(bid[0]); ok {
			if qty, ok := parseDecimalToRat(bid[1]); ok && qty.Sign() > 0 {
				bids = append(bids, corews.DepthLevel{
					Price: price,
					Qty:   qty,
				})
			}
		}
	}

	// Parse asks
	for _, ask := range rec.Asks {
		if len(ask) < 2 {
			continue
		}
		if price, ok := parseDecimalToRat(ask[0]); ok {
			if qty, ok := parseDecimalToRat(ask[1]); ok && qty.Sign() > 0 {
				asks = append(asks, corews.DepthLevel{
					Price: price,
					Qty:   qty,
				})
			}
		}
	}

	// Create book event with partial depth data
	bookEvent := corews.BookEvent{
		Symbol: sym,
		Bids:   bids,
		Asks:   asks,
		Time:   updateTime,
	}

	msg.Topic = corews.BookTopic(sym)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "book"
	msg.Parsed = &bookEvent
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

	// If symbol is not set, try to extract it from the stream name
	if symbol == "" && stream != "" {
		parts := strings.Split(stream, "@")
		if len(parts) > 0 {
			binanceSymbol := parts[0]
			if binanceSymbol != "" {
				if canonical := w.canonicalSymbol(strings.ToUpper(binanceSymbol)); canonical != "" {
					symbol = canonical
				} else {
					symbol = strings.ToUpper(binanceSymbol)
				}
			}
		}
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
