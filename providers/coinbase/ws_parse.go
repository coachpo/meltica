package coinbase

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

func (w ws) parseMessage(msg *core.Message, payload []byte, private bool) error {
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

func (w ws) parseTicker(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	msg.Topic = core.TickerTopic(symbol)
	msg.Event = "ticker"
	bid := parseDecimal(fmt.Sprint(env["best_bid"]))
	ask := parseDecimal(fmt.Sprint(env["best_ask"]))
	msg.Parsed = &core.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w ws) parseMatch(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	price := parseDecimal(fmt.Sprint(env["price"]))
	qty := parseDecimal(fmt.Sprint(env["size"]))
	msg.Topic = core.TradeTopic(symbol)
	msg.Event = "trade"
	msg.Parsed = &core.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w ws) parseL2(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	msg.Topic = core.DepthTopic(symbol)
	msg.Event = "depth"
	changes, _ := env["changes"].([]any)
	event := &core.DepthEvent{Symbol: symbol, Time: parseTime(fmt.Sprint(env["time"]))}
	for _, change := range changes {
		row, _ := change.([]any)
		if len(row) < 3 {
			continue
		}
		side := fmt.Sprint(row[0])
		price := parseDecimal(fmt.Sprint(row[1]))
		qty := parseDecimal(fmt.Sprint(row[2]))
		lvl := core.DepthLevel{Price: price, Qty: qty}
		if strings.EqualFold(side, "buy") {
			event.Bids = append(event.Bids, lvl)
		} else {
			event.Asks = append(event.Asks, lvl)
		}
	}
	msg.Parsed = event
	return nil
}

func (w ws) parseSnapshot(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	msg.Topic = core.DepthTopic(symbol)
	msg.Event = "depth"
	event := &core.DepthEvent{Symbol: symbol, Time: parseTime(fmt.Sprint(env["time"]))}
	if bids, ok := env["bids"].([]any); ok {
		event.Bids = append(event.Bids, buildLevels(bids)...)
	}
	if asks, ok := env["asks"].([]any); ok {
		event.Asks = append(event.Asks, buildLevels(asks)...)
	}
	msg.Parsed = event
	return nil
}

func buildLevels(raw []any) []core.DepthLevel {
	out := make([]core.DepthLevel, 0, len(raw))
	for _, row := range raw {
		vals, _ := row.([]any)
		if len(vals) < 2 {
			continue
		}
		price := parseDecimal(fmt.Sprint(vals[0]))
		qty := parseDecimal(fmt.Sprint(vals[1]))
		out = append(out, core.DepthLevel{Price: price, Qty: qty})
	}
	return out
}

func (w ws) parseOrderEvent(msg *core.Message, env map[string]any) error {
	symbol := w.canonicalSymbol(fmt.Sprint(env["product_id"]))
	id := fmt.Sprint(env["order_id"]) + fmt.Sprint(env["client_oid"])
	status := mapStatus(fmt.Sprint(env["type"]), fmt.Sprint(env["reason"]))
	filled := parseDecimal(fmt.Sprint(env["filled_size"]))
	avg := parseDecimal(fmt.Sprint(env["price"]))
	msg.Topic = core.OrderTopic(symbol)
	msg.Event = "order"
	msg.Parsed = &core.OrderEvent{Symbol: symbol, OrderID: id, Status: status, FilledQty: filled, AvgPrice: avg, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w ws) parseBalance(msg *core.Message, env map[string]any) error {
	accounts, _ := env["accounts"].([]any)
	balances := make([]core.Balance, 0, len(accounts))
	for _, acct := range accounts {
		row, _ := acct.(map[string]any)
		asset := strings.ToUpper(fmt.Sprint(row["currency"]))
		free := parseDecimal(fmt.Sprint(row["balance"]))
		balances = append(balances, core.Balance{Asset: asset, Available: free, Time: parseTime(fmt.Sprint(env["time"]))})
	}
	msg.Topic = core.BalanceTopic()
	msg.Event = "balance"
	msg.Parsed = &core.BalanceEvent{Balances: balances}
	return nil
}
