package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
)

func (w *WS) parsePublicMessage(msg *core.Message, payload []byte) error {
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
		msg.Event = typeVal
		return nil
	}
}

func (w *WS) parseTicker(msg *core.Message, env map[string]any) error {
	symbol := w.WSCanonicalSymbol(fmt.Sprint(env["product_id"]))
	msg.Topic = topicFromProviderName(CNBTopicTicker, symbol)
	msg.Event = coreprovider.RouteTickerUpdate
	bid := parseDecimal(fmt.Sprint(env["best_bid"]))
	ask := parseDecimal(fmt.Sprint(env["best_ask"]))
	msg.Parsed = &coreprovider.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w *WS) parseMatch(msg *core.Message, env map[string]any) error {
	symbol := w.WSCanonicalSymbol(fmt.Sprint(env["product_id"]))
	price := parseDecimal(fmt.Sprint(env["price"]))
	qty := parseDecimal(fmt.Sprint(env["size"]))
	msg.Topic = topicFromProviderName(CNBTopicTrade, symbol)
	msg.Event = coreprovider.RouteTradeUpdate
	msg.Parsed = &coreprovider.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: parseTime(fmt.Sprint(env["time"]))}
	return nil
}

func (w *WS) parseL2(msg *core.Message, env map[string]any) error {
	symbol := w.WSCanonicalSymbol(fmt.Sprint(env["product_id"]))

	// Get or create order book for this symbol
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
		lvl := core.BookDepthLevel{Price: price, Qty: qty}
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

	msg.Topic = topicFromProviderName("l2update", symbol)
	msg.Event = coreprovider.RouteBookSnapshot
	msg.Parsed = &completeSnapshot
	return nil
}

func (w *WS) parseSnapshot(msg *core.Message, env map[string]any) error {
	symbol := w.WSCanonicalSymbol(fmt.Sprint(env["product_id"]))
	if symbol == "" {
		return fmt.Errorf("coinbase ws snapshot: missing symbol")
	}

	// Get or create order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	updateTime := parseTime(fmt.Sprint(env["time"]))
	var bids, asks []core.BookDepthLevel

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

	msg.Topic = topicFromProviderName("snapshot", symbol)
	msg.Event = coreprovider.RouteBookSnapshot
	msg.Parsed = &completeSnapshot
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
		out = append(out, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return out
}
