package routing

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
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
		panic(fmt.Sprintf("binance ws: missing symbol in message; stream=%s, event=%s", stream, meta.Event))
	}
	symbol := w.WSCanonicalSymbol(meta.Symbol)
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

func (w *WSRouter) parseTradeEvent(msg *RoutedMessage, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string `json:"s"`
		Price  string `json:"p"`
		Qty    string `json:"q"`
		Time   int64  `json:"T"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	if symbol == "" {
		panic(fmt.Sprintf("binance ws trade: missing symbol; stream=%s", stream))
	}
	topic := topicFromChannel(BNXTradeChannel, symbol)
	msg.Topic = topic
	msg.Route = coreprovider.RouteTradeUpdate
	price, _ := parseDecimalToRat(rec.Price)
	qty, _ := parseDecimalToRat(rec.Qty)
	msg.Parsed = &coreprovider.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

func (w *WSRouter) parseBookTicker(msg *RoutedMessage, payload []byte, symbol, stream string) error {
	var rec struct {
		Symbol string `json:"s"`
		Bid    string `json:"b"`
		Ask    string `json:"a"`
		Time   int64  `json:"E"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	if symbol == "" {
		panic(fmt.Sprintf("binance ws bookTicker: missing symbol; stream=%s", stream))
	}
	topic := topicFromChannel(BNXTickerChannel, symbol)
	msg.Topic = topic
	msg.Route = coreprovider.RouteTickerUpdate
	bid, _ := parseDecimalToRat(rec.Bid)
	ask, _ := parseDecimalToRat(rec.Ask)
	msg.Parsed = &coreprovider.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.UnixMilli(rec.Time)}
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
		return err
	}
	if rec.Event != "depthUpdate" {
		return fmt.Errorf("binance ws: unexpected event type in depth stream: %s", rec.Event)
	}
	if symbol == "" {
		panic(fmt.Sprintf("binance ws partial depth: missing symbol; stream=%s", stream))
	}

	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	eventTime := time.UnixMilli(rec.EventTime)

	orderBook.BufferEvent(rec.FirstUpdateID, rec.LastUpdateID, bids, asks, eventTime)

	if orderBook.IsInitialized() {
		success := UpdateFromBinanceDelta(orderBook, bids, asks, rec.FirstUpdateID, rec.LastUpdateID, eventTime)
		if !success {
			go w.initializeAndRecover(symbol, rec.FirstUpdateID, rec.LastUpdateID)
			return fmt.Errorf("order book out of sync for %s, restarting initialization", symbol)
		}
	}

	msg.Topic = corews.BookTopic(symbol)
	msg.Route = coreprovider.RouteBookSnapshot
	msg.Parsed = &coreprovider.BookEvent{
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
		price, _ := parseDecimalToRat(pStr)
		qty, _ := parseDecimalToRat(qStr)
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
