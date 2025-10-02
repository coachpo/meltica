package binance

import (
	"encoding/json"
	"time"

	"github.com/coachpo/meltica/core"
)

type binanceEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

func (w ws) parsePublicMessage(msg *core.Message, raw []byte) error {
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

func (w ws) parseTradeEvent(msg *core.Message, payload []byte, symbol, stream string) error {
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
	msg.Parsed = &core.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w ws) parseBookTicker(msg *core.Message, payload []byte, symbol, stream string) error {
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
	msg.Parsed = &core.TickerEvent{Symbol: sym, Bid: bid, Ask: ask, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w ws) parseDepthUpdate(msg *core.Message, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string     `json:"s"`
		Time   int64      `json:"E"`
		Bids   [][]string `json:"b"`
		Asks   [][]string `json:"a"`
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
	msg.Topic = topicFromEvent("depthUpdate", sym)
	if msg.Topic == "" {
		msg.Topic = stream
	}
	msg.Event = "depth"
	de := core.DepthEvent{Symbol: sym, Time: time.UnixMilli(rec.Time)}
	de.Bids = append(de.Bids, depthLevelsFromPairs(rec.Bids)...)
	de.Asks = append(de.Asks, depthLevelsFromPairs(rec.Asks)...)
	msg.Parsed = &de
	return nil
}

func (w ws) parseByHeuristics(msg *core.Message, payload []byte, symbol, stream string) error {
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

func depthLevelsFromPairs(pairs [][]string) []core.DepthLevel {
	levels := make([]core.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		price, _ := parseDecimalToRat(pair[0])
		qty, _ := parseDecimalToRat(pair[1])
		levels = append(levels, core.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}
