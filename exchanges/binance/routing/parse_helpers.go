package routing

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/internal/numeric"
)

type tradeRecord struct {
	Symbol string      `json:"s"`
	Price  json.Number `json:"p"`
	Qty    json.Number `json:"q"`
	Time   int64       `json:"T"`
}

type tickerRecord struct {
	EventTime int64       `json:"E"`
	Symbol    string      `json:"s"`
	BidPrice  json.Number `json:"b"`
	BidQty    json.Number `json:"B"`
	AskPrice  json.Number `json:"a"`
	AskQty    json.Number `json:"A"`
	LastPrice json.Number `json:"c"`
	LastQty   json.Number `json:"Q"`
}

type orderbookRecord struct {
	Symbol        string          `json:"s"`
	FirstUpdateID int64           `json:"U"`
	LastUpdateID  int64           `json:"u"`
	Bids          [][]interface{} `json:"b"`
	Asks          [][]interface{} `json:"a"`
	EventTime     int64           `json:"E"`
}

// parseTradeEvent populates a RoutedMessage with trade data.
func parseTradeEvent(msg *RoutedMessage, rec *tradeRecord, symbol string) error {
	if symbol == "" {
		return internal.Exchange("ws trade: missing symbol")
	}
	msg.Topic = topicForSymbol(core.TopicTrade, symbol)
	msg.Route = corestreams.RouteTradeUpdate
	price, _ := numeric.Parse(rec.Price.String())
	qty, _ := numeric.Parse(rec.Qty.String())
	msg.Parsed = &corestreams.TradeEvent{Symbol: symbol, VenueSymbol: rec.Symbol, Price: price, Quantity: qty, Time: time.UnixMilli(rec.Time)}
	return nil
}

// parseTickerEvent populates a RoutedMessage with ticker data.
func parseTickerEvent(msg *RoutedMessage, rec *tickerRecord, symbol string) error {
	if symbol == "" {
		return internal.Exchange("ws ticker: missing symbol")
	}
	msg.Topic = topicForSymbol(core.TopicTicker, symbol)
	msg.Route = corestreams.RouteTickerUpdate
	bid, _ := numeric.Parse(rec.BidPrice.String())
	ask, _ := numeric.Parse(rec.AskPrice.String())
	eventTime := time.UnixMilli(rec.EventTime)
	msg.Parsed = &corestreams.TickerEvent{Symbol: symbol, VenueSymbol: rec.Symbol, Bid: bid, Ask: ask, Time: eventTime}
	return nil
}

// parseOrderbookEvent populates a RoutedMessage with order book delta data.
func parseOrderbookEvent(msg *RoutedMessage, rec *orderbookRecord, symbol string, event string) error {
	if event != "depthUpdate" {
		return internal.Exchange("ws: unexpected event type in depth stream: %s", event)
	}
	if symbol == "" {
		return internal.Exchange("ws depth: missing symbol")
	}

	bids := depthLevelsFromPairs(rec.Bids)
	asks := depthLevelsFromPairs(rec.Asks)
	eventTime := time.UnixMilli(rec.EventTime)

	msg.Topic = OrderBook(symbol)
	msg.Route = corestreams.RouteDepthDelta
	msg.Parsed = &DepthDelta{
		Symbol:        symbol,
		VenueSymbol:   rec.Symbol,
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
	var rec struct {
		EventTime         int64       `json:"E"`
		Symbol            string      `json:"s"`
		Side              string      `json:"S"`
		OrderType         string      `json:"o"`
		TimeInForce       string      `json:"f"`
		Quantity          json.Number `json:"q"`
		Price             json.Number `json:"p"`
		OrderStatus       string      `json:"X"`
		OrderID           int64       `json:"i"`
		CumulativeQty     json.Number `json:"z"`
		LastExecutedPrice json.Number `json:"L"`
		TransactionTime   int64       `json:"T"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return err
	}
	quantity, _ := numeric.Parse(rec.Quantity.String())
	filled, _ := numeric.Parse(rec.CumulativeQty.String())
	avg, _ := numeric.Parse(rec.LastExecutedPrice.String())
	price, _ := numeric.Parse(rec.Price.String())
	order := &corestreams.OrderEvent{
		Symbol:      rec.Symbol,
		OrderID:     fmt.Sprintf("%d", rec.OrderID),
		Status:      internal.MapOrderStatus(rec.OrderStatus),
		FilledQty:   filled,
		AvgPrice:    avg,
		Quantity:    quantity,
		Price:       price,
		Side:        normalizeOrderSide(rec.Side),
		Type:        normalizeOrderType(rec.OrderType),
		TimeInForce: normalizeTimeInForce(rec.TimeInForce),
		Time:        time.UnixMilli(rec.TransactionTime),
	}
	if order.AvgPrice == nil {
		order.AvgPrice = price
	}
	if order.Quantity == nil {
		order.Quantity = quantity
	}
	msg.Route = corestreams.RouteOrderUpdate
	msg.Topic = topicForSymbol(core.TopicUserOrder, rec.Symbol)
	msg.Parsed = order
	return nil
}

// parseBalanceUpdateEvent populates a RoutedMessage with balance data.
func parseBalanceUpdateEvent(msg *RoutedMessage, payload []byte, event string) error {
	msg.Route = corestreams.RouteBalanceSnapshot

	switch event {
	case accountSnapshotEvent:
		var oap struct {
			Balances []struct {
				Asset  string      `json:"a"`
				Free   json.Number `json:"f"`
				Locked json.Number `json:"l"`
			} `json:"B"`
		}
		if err := json.Unmarshal(payload, &oap); err != nil {
			return err
		}
		var be corestreams.BalanceEvent
		eventMillis, _, err := extractInt64Field(payload, "E")
		if err != nil {
			return err
		}
		for _, b := range oap.Balances {
			avail, _ := numeric.Parse(b.Free.String())
			locked, _ := numeric.Parse(b.Locked.String())
			var total *big.Rat
			if avail != nil && locked != nil {
				total = new(big.Rat).Add(new(big.Rat).Set(avail), locked)
			} else if avail != nil {
				total = new(big.Rat).Set(avail)
			}
			be.Balances = append(be.Balances, core.Balance{Asset: strings.ToUpper(b.Asset), Available: avail, Locked: locked, Total: total, Time: time.UnixMilli(eventMillis)})
		}
		msg.Topic = UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &be
		return nil
	case "balanceUpdate":
		var bu struct {
			Asset string      `json:"a"`
			Delta json.Number `json:"d"`
		}
		if err := json.Unmarshal(payload, &bu); err != nil {
			return err
		}
		amt, _ := numeric.Parse(bu.Delta.String())
		eventMillis, _, err := extractInt64Field(payload, "E")
		if err != nil {
			return err
		}
		var eventTime time.Time
		if eventMillis > 0 {
			eventTime = time.UnixMilli(eventMillis)
		}
		msg.Topic = UserBalance()
		msg.Route = corestreams.RouteBalanceSnapshot
		msg.Parsed = &corestreams.BalanceEvent{
			Balances: []core.Balance{
				{Asset: strings.ToUpper(bu.Asset), Available: amt, Total: cloneRat(amt), Locked: new(big.Rat), Time: eventTime},
			},
		}
		return nil
	default:
		return nil
	}
}

func normalizeOrderSide(raw string) core.OrderSide {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "BUY":
		return core.SideBuy
	case "SELL":
		return core.SideSell
	default:
		return ""
	}
}

func normalizeOrderType(raw string) core.OrderType {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "MARKET":
		return core.TypeMarket
	case "LIMIT":
		return core.TypeLimit
	default:
		return core.OrderType(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func normalizeTimeInForce(raw string) core.TimeInForce {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "GTC":
		return core.GTC
	case "IOC":
		return core.IOC
	case "FOK":
		return core.FOK
	default:
		return core.TimeInForce(strings.ToLower(strings.TrimSpace(raw)))
	}
}

func cloneRat(src *big.Rat) *big.Rat {
	if src == nil {
		return nil
	}
	return new(big.Rat).Set(src)
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
