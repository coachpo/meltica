package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

type binanceEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

func (w *WSRouter) parsePublicMessage(msg *RoutedMessage, raw []byte) error {
	payload, stream := unwrapCombinedPayload(raw)

	var meta struct {
		Event  string `json:"e"`
		Symbol string `json:"s"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil
	}

	if meta.Symbol == "" {
		return internal.Exchange("ws: missing symbol in event=%s stream=%s", meta.Event, stream)
	}
	symbol, err := w.deps.CanonicalSymbol(meta.Symbol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: binance ws: symbol resolution failed; stream=%s, event=%s, meta.Symbol=%s, err=%v\n", stream, meta.Event, meta.Symbol, err)
		return err
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

func (w *WSRouter) parseTradeEvent(msg *RoutedMessage, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string      `json:"s"`
		Price  json.Number `json:"p"`
		Qty    json.Number `json:"q"`
		Time   int64       `json:"T"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode trade event payload")
	}
	if symbol == "" {
		return internal.Exchange("ws trade: missing symbol stream=%s", stream)
	}
	topic := topicFromChannel(BNXTradeChannel, symbol)
	msg.Topic = topic
	msg.Route = corestreams.RouteTradeUpdate
	price, _ := numeric.Parse(rec.Price.String())
	qty, _ := numeric.Parse(rec.Qty.String())
	msg.Parsed = &corestreams.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w *WSRouter) parseBookTicker(msg *RoutedMessage, payload []byte, symbol, stream string) error {
	var rec struct {
		EventType string      `json:"e"`
		EventTime int64       `json:"E"`
		Symbol    string      `json:"s"`
		BidPrice  json.Number `json:"b"`
		BidQty    json.Number `json:"B"`
		AskPrice  json.Number `json:"a"`
		AskQty    json.Number `json:"A"`
		LastPrice json.Number `json:"c"`
		LastQty   json.Number `json:"Q"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode book ticker payload")
	}
	if symbol == "" {
		return internal.Exchange("ws bookTicker: missing symbol stream=%s", stream)
	}
	topic := topicFromChannel(BNXTickerChannel, symbol)
	msg.Topic = topic
	msg.Route = corestreams.RouteTickerUpdate
	bid, _ := numeric.Parse(rec.BidPrice.String())
	ask, _ := numeric.Parse(rec.AskPrice.String())
	eventTime := time.UnixMilli(rec.EventTime)
	msg.Parsed = &corestreams.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: eventTime}
	return nil
}

func (w *WSRouter) parseBookStream(msg *RoutedMessage, payload []byte, symbol, stream string) error {
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
		return internal.WrapExchange(err, "decode book depth payload")
	}
	if rec.Event != "depthUpdate" {
		return internal.Exchange("ws: unexpected event type in depth stream: %s", rec.Event)
	}
	if symbol == "" {
		return internal.Exchange("ws depth: missing symbol stream=%s", stream)
	}

	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)
	eventTime := time.UnixMilli(rec.EventTime)

	// Expose raw DepthDelta event for Level 3 to consume
	msg.Topic = coretopics.Book(symbol)
	msg.Route = corestreams.RouteDepthDelta
	msg.Parsed = &DepthDelta{
		Symbol:        symbol,
		FirstUpdateID: rec.FirstUpdateID,
		LastUpdateID:  rec.LastUpdateID,
		Bids:          bids,
		Asks:          asks,
		EventTime:     eventTime,
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

func depthLevelsFromPairs(pairs [][]interface{}) []core.BookDepthLevel {
	levels := make([]core.BookDepthLevel, 0, len(pairs))
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
		price, _ := numeric.Parse(pStr)
		qty, _ := numeric.Parse(qStr)
		levels = append(levels, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return levels
}

func streamSymbol(stream string) string {
	if stream == "" {
		return ""
	}
	parts := strings.SplitN(stream, "@", 2)
	if len(parts) == 0 {
		return ""
	}
	return strings.ToUpper(parts[0])
}
