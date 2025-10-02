package ws

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

func (w *WS) parseMessage(msg *core.Message, data []byte, requested []string) error {
	if len(data) == 0 {
		return nil
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	if channel := valueString(env["channel"]); channel != "" {
		return w.parseV2Channel(msg, channel, env, requested)
	}
	if method := valueString(env["method"]); method != "" {
		msg.Topic = method
		return nil
	}
	msg.Topic = valueString(env["event"])
	if msg.Topic == "" {
		msg.Topic = valueString(env["type"])
	}
	return nil
}

func (w *WS) parseV2Channel(msg *core.Message, channel string, env map[string]any, requested []string) error {
	normalized := normalizePublicChannel(channel)
	if normalized == "" {
		msg.Topic = channel
		return nil
	}
	var data []any
	switch typed := env["data"].(type) {
	case []any:
		data = typed
	case map[string]any:
		data = []any{typed}
	case nil:
		data = nil
	default:
		return nil
	}
	symbol := valueString(env["symbol"])
	if symbol == "" {
		for _, entry := range data {
			rec, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if sym := valueString(rec["symbol"]); sym != "" {
				symbol = sym
				break
			}
		}
	}
	canon := w.p.CanonicalSymbol(symbol, requested)
	msg.Topic = topicFromChannelName(normalized, canon)
	switch normalized {
	case "trade":
		return w.parseTrades(msg, data, canon)
	case "ticker":
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseTicker(msg, payload, canon)
	case "book":
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseBook(msg, payload, canon)
	case "level3":
		msg.Topic = corews.BookTopic(canon)
		return w.parseLevel3(msg, data, canon)
	default:
		return nil
	}
}

func (w *WS) parseTrades(msg *core.Message, payload any, symbol string) error {
	rows, ok := payload.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	var events []*corews.TradeEvent
	for _, row := range rows {
		rec, ok := row.(map[string]any)
		if !ok {
			continue
		}
		sym := symbol
		if sym == "" {
			if raw := valueString(rec["symbol"]); raw != "" {
				sym = w.p.CanonicalSymbol(raw, nil)
			}
		}
		price := parseDecimalStr(valueString(firstPresent(rec, "price", "px")))
		qty := parseDecimalStr(valueString(firstPresent(rec, "qty", "quantity", "volume")))
		when := parseISOTime(valueString(firstPresent(rec, "timestamp", "time")))
		if when.IsZero() {
			when = time.Now().UTC()
		}
		events = append(events, &corews.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: when})
	}
	if len(events) > 0 {
		msg.Event = "trade"
		msg.Parsed = events[len(events)-1]
	}
	return nil
}

func (w *WS) parseTicker(msg *core.Message, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	bid := parseDecimalStr(valueString(firstPresent(row, "bid", "best_bid")))
	ask := parseDecimalStr(valueString(firstPresent(row, "ask", "best_ask")))
	msg.Event = "ticker"
	msg.Parsed = &corews.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w *WS) parseBook(msg *core.Message, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	de := corews.DepthEvent{Symbol: symbol, Time: time.Now().UTC()}
	if rawBids, ok := row["bids"]; ok {
		appendDepthLevels(&de.Bids, rawBids)
	}
	if rawAsks, ok := row["asks"]; ok {
		appendDepthLevels(&de.Asks, rawAsks)
	}
	msg.Event = "depth"
	msg.Parsed = &de
	return nil
}

func (w *WS) parseLevel3(msg *core.Message, payload any, symbol string) error {
	rows, ok := payload.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	last := rows[len(rows)-1]
	rec, ok := last.(map[string]any)
	if !ok {
		return nil
	}
	de := corews.DepthEvent{Symbol: symbol, Time: time.Now().UTC()}
	if rawBids, ok := rec["bids"]; ok {
		appendDepthLevels(&de.Bids, rawBids)
	}
	if rawAsks, ok := rec["asks"]; ok {
		appendDepthLevels(&de.Asks, rawAsks)
	}
	if orders, ok := rec["orders"]; ok {
		if arr, ok := orders.([]any); ok {
			for _, ordRaw := range arr {
				ord, ok := ordRaw.(map[string]any)
				if !ok {
					continue
				}
				lvl := corews.DepthLevel{
					Price: parseDecimalStr(valueString(firstPresent(ord, "price", "px"))),
					Qty:   parseDecimalStr(valueString(firstPresent(ord, "qty", "quantity", "volume", "size"))),
				}
				side := strings.ToLower(valueString(firstPresent(ord, "side", "type")))
				switch side {
				case "buy", "bid":
					de.Bids = append(de.Bids, lvl)
				case "sell", "ask":
					de.Asks = append(de.Asks, lvl)
				}
			}
		}
	}
	if len(de.Bids) == 0 && len(de.Asks) == 0 {
		return nil
	}
	msg.Event = "depth"
	msg.Parsed = &de
	return nil
}
