package routing

import (
	"encoding/json"
	"strings"
	"time"

	coreprovider "github.com/coachpo/meltica/core/provider"
)

func (w *WSRouter) parsePublicMessage(msg *RoutedMessage, data []byte, requested []string) error {
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

func (w *WSRouter) parseV2Channel(msg *RoutedMessage, channel string, env map[string]any, requested []string) error {
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
	canon := w.deps.CanonicalSymbol(symbol, requested)
	msg.Topic = topicFromChannel(channel, canon)
	switch channel {
	case KrkTopicTrade:
		return w.parseTrades(msg, data, canon, requested)
	case KrkTopicTicker:
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseTicker(msg, payload, canon)
	case KrkTopicBook:
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseBook(msg, payload, canon)
	default:
		return nil
	}
}

func (w *WSRouter) parseTrades(msg *RoutedMessage, payload any, symbol string, requested []string) error {
	rows, ok := payload.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	var events []*coreprovider.TradeEvent
	for _, row := range rows {
		rec, ok := row.(map[string]any)
		if !ok {
			continue
		}
		sym := symbol
		if sym == "" {
			if raw := valueString(rec["symbol"]); raw != "" {
				sym = w.deps.CanonicalSymbol(raw, requested)
				if sym == "" {
					sym = w.deps.MapNativeToCanon(strings.ToUpper(strings.ReplaceAll(raw, "/", "")))
				}
			}
		}
		price := parseDecimalStr(valueString(firstPresent(rec, "price", "px")))
		qty := parseDecimalStr(valueString(firstPresent(rec, "qty", "quantity", "volume")))
		when := parseISOTime(valueString(firstPresent(rec, "timestamp", "time")))
		if when.IsZero() {
			when = time.Now().UTC()
		}
		events = append(events, &coreprovider.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: when})
	}
	if len(events) > 0 {
		msg.Route = coreprovider.RouteTradeUpdate
		msg.Parsed = events[len(events)-1]
	}
	return nil
}

func (w *WSRouter) parseTicker(msg *RoutedMessage, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	bid := parseDecimalStr(valueString(firstPresent(row, "bid", "best_bid")))
	ask := parseDecimalStr(valueString(firstPresent(row, "ask", "best_ask")))
	msg.Route = coreprovider.RouteTickerUpdate
	msg.Parsed = &coreprovider.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w *WSRouter) parseBook(msg *RoutedMessage, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	de := coreprovider.BookEvent{Symbol: symbol, Time: time.Now().UTC()}
	if rawBids, ok := row["bids"]; ok {
		appendDepthLevels(&de.Bids, rawBids)
	}
	if rawAsks, ok := row["asks"]; ok {
		appendDepthLevels(&de.Asks, rawAsks)
	}
	msg.Route = coreprovider.RouteBookSnapshot
	msg.Parsed = &de
	return nil
}
