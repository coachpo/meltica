package routing

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

// parseTradeEvent populates a RoutedMessage with trade data.
func parseTradeEvent(msg *RoutedMessage, rec *struct {
	Symbol string      `json:"s"`
	Price  json.Number `json:"p"`
	Qty    json.Number `json:"q"`
	Time   int64       `json:"T"`
}, symbol string) error {
	if symbol == "" {
		return internal.Exchange("ws trade: missing symbol")
	}
	topic := topicFromChannel(BNXTradeChannel, symbol)
	msg.Topic = topic
	msg.Route = corestreams.RouteTradeUpdate
	price, _ := numeric.Parse(rec.Price.String())
	qty, _ := numeric.Parse(rec.Qty.String())
	msg.Parsed = &corestreams.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

// parseBookTickerEvent populates a RoutedMessage with ticker data.
func parseBookTickerEvent(msg *RoutedMessage, rec *struct {
	EventType string      `json:"e"`
	EventTime int64       `json:"E"`
	Symbol    string      `json:"s"`
	BidPrice  json.Number `json:"b"`
	BidQty    json.Number `json:"B"`
	AskPrice  json.Number `json:"a"`
	AskQty    json.Number `json:"A"`
	LastPrice json.Number `json:"c"`
	LastQty   json.Number `json:"Q"`
}, symbol string) error {
	if symbol == "" {
		return internal.Exchange("ws bookTicker: missing symbol")
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

// parseDepthEvent populates a RoutedMessage with depth delta data.
func parseDepthEvent(msg *RoutedMessage, rec *struct {
	Event         string          `json:"e"`
	Symbol        string          `json:"s"`
	FirstUpdateID int64           `json:"U"`
	LastUpdateID  int64           `json:"u"`
	Bids          [][]interface{} `json:"b"`
	Asks          [][]interface{} `json:"a"`
	EventTime     int64           `json:"E"`
}, symbol string) error {
	if rec.Event != "depthUpdate" {
		return internal.Exchange("ws: unexpected event type in depth stream: %s", rec.Event)
	}
	if symbol == "" {
		return internal.Exchange("ws depth: missing symbol")
	}

	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)
	eventTime := time.UnixMilli(rec.EventTime)

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

// parseOrderUpdateEvent populates a RoutedMessage with order update data.
func parseOrderUpdateEvent(msg *RoutedMessage, payload []byte) error {
	var ou struct {
		EventTime int64 `json:"E"`
		Order     struct {
			Symbol    string      `json:"s"`
			ID        int64       `json:"i"`
			Status    string      `json:"X"`
			FilledQty json.Number `json:"z"`
			AvgPrice  json.Number `json:"ap"`
			EventTime int64       `json:"T"`
		} `json:"o"`
	}
	if err := json.Unmarshal(payload, &ou); err != nil {
		return err
	}

	filled, _ := numeric.Parse(ou.Order.FilledQty.String())
	avg, _ := numeric.Parse(ou.Order.AvgPrice.String())
	msg.Route = corestreams.RouteOrderUpdate
	msg.Topic = topicFromChannel(BNXOrderChannel, ou.Order.Symbol)
	msg.Parsed = &corestreams.OrderEvent{
		Symbol:    ou.Order.Symbol,
		OrderID:   fmt.Sprintf("%d", ou.Order.ID),
		Status:    internal.MapOrderStatus(ou.Order.Status),
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      time.UnixMilli(ou.Order.EventTime),
	}
	return nil
}

// parseBalanceUpdateEvent populates a RoutedMessage with balance data.
func parseBalanceUpdateEvent(msg *RoutedMessage, payload []byte, event string) error {
	msg.Route = corestreams.RouteBalanceSnapshot

	switch event {
	case "outboundAccountPosition":
		var oap struct {
			EventTime int64 `json:"E"`
			Balances  []struct {
				Asset string      `json:"a"`
				Free  json.Number `json:"f"`
			} `json:"B"`
		}
		if err := json.Unmarshal(payload, &oap); err != nil {
			return err
		}
		var be corestreams.BalanceEvent
		for _, b := range oap.Balances {
			amt, _ := numeric.Parse(b.Free.String())
			be.Balances = append(be.Balances, core.Balance{Asset: b.Asset, Available: amt, Time: time.UnixMilli(oap.EventTime)})
		}
		msg.Topic = coretopics.UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &be
		return nil
	case "balanceUpdate":
		var bu struct {
			EventTime int64       `json:"E"`
			Asset     string      `json:"a"`
			Delta     json.Number `json:"d"`
		}
		if err := json.Unmarshal(payload, &bu); err != nil {
			return err
		}
		amt, _ := numeric.Parse(bu.Delta.String())
		msg.Topic = coretopics.UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &corestreams.BalanceEvent{
			Balances: []core.Balance{
				{Asset: bu.Asset, Available: amt, Time: time.UnixMilli(bu.EventTime)},
			},
		}
		return nil
	default:
		return nil
	}
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
