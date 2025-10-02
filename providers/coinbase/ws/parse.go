package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

func (w *WS) parseMessage(msg *core.Message, payload []byte, private bool) error {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	typeVal := fmt.Sprint(env["type"])
	switch typeVal {
	case "subscriptions", "heartbeat", "error":
		msg.Topic = typeVal
		msg.Event = typeVal
		return nil
	case "ticker":
		return w.parseTicker(msg, env)
	case "l2update":
		return w.parseL2(msg, env)
	case "snapshot":
		return w.parseSnapshot(msg, env)
	case "match":
		return w.parseMatch(msg, env)
	case "received", "open", "done", "change", "activate":
		if !private {
			return nil
		}
		return w.parseOrderEvent(msg, env)
	case "wallet", "profile":
		if !private {
			return nil
		}
		return w.parseBalance(msg, env)
	default:
		msg.Topic = mapper.ToProtocolTopic(typeVal)
		msg.Event = typeVal
		return nil
	}
}

func (w *WS) parseTicker(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	msg.Topic = corews.TickerTopic(symbol)
	msg.Event = "ticker"
	bid := parseDecimal(fmt.Sprint(env["best_bid"]))
	ask := parseDecimal(fmt.Sprint(env["best_ask"]))
	msg.Parsed = &corews.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w *WS) parseMatch(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	price := parseDecimal(fmt.Sprint(env["price"]))
	qty := parseDecimal(fmt.Sprint(env["size"]))
	msg.Topic = corews.TradeTopic(symbol)
	msg.Event = "trade"
	msg.Parsed = &corews.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WS) parseL2(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))

	// Get or create order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	changes, _ := env["changes"].([]any)
	updateTime := parseTime(fmt.Sprint(env["time"]))

	var bids, asks []corews.DepthLevel
	for _, change := range changes {
		row, _ := change.([]any)
		if len(row) < 3 {
			continue
		}
		side := fmt.Sprint(row[0])
		price := parseDecimal(fmt.Sprint(row[1]))
		qty := parseDecimal(fmt.Sprint(row[2]))
		lvl := corews.DepthLevel{Price: price, Qty: qty}
		if strings.EqualFold(side, "buy") {
			bids = append(bids, lvl)
		} else {
			asks = append(asks, lvl)
		}
	}

	// Apply delta update to order book
	UpdateFromCoinbaseDelta(orderBook, bids, asks, updateTime)

	// Get the complete order book snapshot
	completeSnapshot := orderBook.GetSnapshot()

	msg.Topic = corews.BookTopic(symbol)
	msg.Event = "book"
	msg.Parsed = &completeSnapshot
	return nil
}

func (w *WS) parseSnapshot(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))

	// Get or create order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	updateTime := parseTime(fmt.Sprint(env["time"]))
	var bids, asks []corews.DepthLevel

	if bidsData, ok := env["bids"].([]any); ok {
		bids = buildLevels(bidsData)
	}
	if asksData, ok := env["asks"].([]any); ok {
		asks = buildLevels(asksData)
	}

	// Update order book from snapshot
	orderBook.UpdateFromSnapshot(bids, asks, updateTime)

	// Get the complete order book snapshot
	completeSnapshot := orderBook.GetSnapshot()

	msg.Topic = corews.BookTopic(symbol)
	msg.Event = "book"
	msg.Parsed = &completeSnapshot
	return nil
}

func buildLevels(raw []any) []corews.DepthLevel {
	out := make([]corews.DepthLevel, 0, len(raw))
	for _, row := range raw {
		vals, _ := row.([]any)
		if len(vals) < 2 {
			continue
		}
		price := parseDecimal(fmt.Sprint(vals[0]))
		qty := parseDecimal(fmt.Sprint(vals[1]))
		out = append(out, corews.DepthLevel{Price: price, Qty: qty})
	}
	return out
}

func (w *WS) parseOrderEvent(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	id := fmt.Sprint(env["order_id"]) + fmt.Sprint(env["client_oid"])
	status := mapStatus(fmt.Sprint(env["type"]), fmt.Sprint(env["reason"]))
	filled := parseDecimal(fmt.Sprint(env["filled_size"]))
	avg := parseDecimal(fmt.Sprint(env["price"]))
	msg.Topic = corews.UserOrderTopic(symbol)
	msg.Event = "order"
	msg.Parsed = &corews.OrderEvent{Symbol: symbol, OrderID: id, Status: status, FilledQty: filled, AvgPrice: avg, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WS) parseBalance(msg *core.Message, env map[string]any) error {
	accounts, _ := env["accounts"].([]any)
	balances := make([]corews.Balance, 0, len(accounts))
	for _, acct := range accounts {
		row, _ := acct.(map[string]any)
		asset := strings.ToUpper(fmt.Sprint(row["currency"]))
		free := parseDecimal(fmt.Sprint(row["balance"]))
		balances = append(balances, corews.Balance{Asset: asset, Available: free, Time: parseTime(fmt.Sprint(env["time"]))})
	}
	msg.Topic = corews.UserBalanceTopic()
	msg.Event = "balance"
	msg.Parsed = &corews.BalanceEvent{Balances: balances}
	return nil
}
