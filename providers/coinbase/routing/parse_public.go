package routing

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
)

func (w *WSRouter) parsePublicMessage(msg *coreprovider.RoutedMessage, payload []byte) error {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	typeVal := fmt.Sprint(env["type"])
	switch typeVal {
	case "subscriptions", "heartbeat", "error":
		msg.Topic = typeVal
		msg.Route = coreprovider.RouteUnknown
		return nil
	case CNBTopicTicker:
		return w.parseTicker(msg, env)
	case "l2update":
		return w.parseL2(msg, env)
	case "snapshot":
		return w.parseSnapshot(msg, env)
	case "match":
		return w.parseMatch(msg, env)
	default:
		msg.Topic = typeVal
		msg.Route = coreprovider.RouteUnknown
		return nil
	}
}

func (w *WSRouter) parseTicker(msg *coreprovider.RoutedMessage, env map[string]any) error {
	symbol := w.deps.CanonicalFromNative(fmt.Sprint(env["product_id"]))
	msg.Topic = topicFromProviderName(CNBTopicTicker, symbol)
	msg.Route = coreprovider.RouteTickerUpdate
	bid := parseDecimal(fmt.Sprint(env["best_bid"]))
	ask := parseDecimal(fmt.Sprint(env["best_ask"]))
	msg.Parsed = &coreprovider.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w *WSRouter) parseMatch(msg *coreprovider.RoutedMessage, env map[string]any) error {
	symbol := w.deps.CanonicalFromNative(fmt.Sprint(env["product_id"]))
	price := parseDecimal(fmt.Sprint(env["price"]))
	qty := parseDecimal(fmt.Sprint(env["size"]))
	msg.Topic = topicFromProviderName(CNBTopicTrade, symbol)
	msg.Route = coreprovider.RouteTradeUpdate
	msg.Parsed = &coreprovider.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WSRouter) parseL2(msg *coreprovider.RoutedMessage, env map[string]any) error {
	symbol := w.deps.CanonicalFromNative(fmt.Sprint(env["product_id"]))
	if symbol == "" {
		return fmt.Errorf("coinbase ws: missing product_id in l2update")
	}

	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	changes, _ := env["changes"].([]any)
	updateTime := parseTime(fmt.Sprint(env["time"]))

	var bids, asks []core.BookDepthLevel
	for _, change := range changes {
		row, _ := change.([]any)
		if len(row) < 3 {
			continue
		}
		side := fmt.Sprint(row[0])
		price := parseDecimal(fmt.Sprint(row[1]))
		qty := parseDecimal(fmt.Sprint(row[2]))
		if price == nil || qty == nil {
			continue
		}
		lvl := core.BookDepthLevel{Price: price, Qty: qty}
		if strings.EqualFold(side, "buy") {
			bids = append(bids, lvl)
		} else {
			asks = append(asks, lvl)
		}
	}

	UpdateFromCoinbaseDelta(orderBook, bids, asks, updateTime)
	snapshot := orderBook.GetSnapshot()

	msg.Topic = topicFromProviderName("l2update", symbol)
	msg.Route = coreprovider.RouteBookSnapshot
	msg.Parsed = &snapshot
	return nil
}

func (w *WSRouter) parseSnapshot(msg *coreprovider.RoutedMessage, env map[string]any) error {
	symbol := w.deps.CanonicalFromNative(fmt.Sprint(env["product_id"]))
	if symbol == "" {
		return fmt.Errorf("coinbase ws snapshot: missing symbol")
	}

	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	updateTime := parseTime(fmt.Sprint(env["time"]))
	var bids, asks []core.BookDepthLevel

	if bidsData, ok := env["bids"].([]any); ok {
		bids = buildLevels(bidsData)
	}
	if asksData, ok := env["asks"].([]any); ok {
		asks = buildLevels(asksData)
	}

	orderBook.UpdateFromSnapshot(bids, asks, updateTime)
	snapshot := orderBook.GetSnapshot()

	msg.Topic = topicFromProviderName("snapshot", symbol)
	msg.Route = coreprovider.RouteBookSnapshot
	msg.Parsed = &snapshot
	return nil
}

func buildLevels(raw []any) []core.BookDepthLevel {
	out := make([]core.BookDepthLevel, 0, len(raw))
	for _, row := range raw {
		vals, _ := row.([]any)
		if len(vals) < 2 {
			continue
		}
		price := parseDecimal(fmt.Sprint(vals[0]))
		qty := parseDecimal(fmt.Sprint(vals[1]))
		if price == nil || qty == nil {
			continue
		}
		out = append(out, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return out
}
