package kraken

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/gorilla/websocket"
)

type ws struct {
	p *Provider
}

// TestOnlyParseTrades exposes parseTrades for tests.
func TestOnlyParseTrades(msg *core.Message, payload any, symbol string) error {
	return ws{}.parseTrades(msg, payload, symbol)
}

// TestOnlyParseTicker exposes parseTicker for tests.
func TestOnlyParseTicker(msg *core.Message, payload any, symbol string) error {
	return ws{}.parseTicker(msg, payload, symbol)
}

// TestOnlyParseBook exposes parseBook for tests.
func TestOnlyParseBook(msg *core.Message, payload any, symbol string) error {
	return ws{}.parseBook(msg, payload, symbol)
}

// TestOnlyParseOwnTrades exposes private trade parsing for tests.
func TestOnlyParseOwnTrades(msg *core.Message, payload any, p *Provider) error {
	return ws{p: p}.parseOwnTrades(msg, payload)
}

// TestOnlyParseOpenOrders exposes private open order parsing for tests.
func TestOnlyParseOpenOrders(msg *core.Message, payload any, p *Provider) error {
	return ws{p: p}.parseOpenOrders(msg, payload)
}

type wsSub struct {
	c    chan core.Message
	err  chan error
	conn *websocket.Conn
}

func (s *wsSub) C() <-chan core.Message { return s.c }
func (s *wsSub) Err() <-chan error      { return s.err }
func (s *wsSub) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (w ws) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	if err := w.p.ensureInstruments(ctx); err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, "wss://ws.kraken.com/v2", nil)
	if err != nil {
		return nil, err
	}
	// Build subscriptions.
	for _, topic := range topics {
		ch, canon := parseTopic(topic)
		if ch == "" || canon == "" {
			continue
		}
		native := ""
		if w.p != nil {
			native = w.p.canonToKrakenWS[canon]
			if native == "" {
				native = w.p.canonToKraken[canon]
			}
		}
		if native == "" {
			native = canon
		}
		native = strings.ReplaceAll(native, "-", "/")
		payload := map[string]any{
			"method": "subscribe",
			"params": map[string]any{
				"channel": ch,
				"symbol":  []string{native},
			},
		}
		if err := conn.WriteJSON(payload); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	sub := &wsSub{c: make(chan core.Message, 1024), err: make(chan error, 1), conn: conn}
	go w.readLoop(sub, topics)
	return sub, nil
}

func (w ws) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	if w.p.apiKey == "" || w.p.secret == "" {
		return nil, core.ErrNotSupported
	}
	token, err := w.p.getWSToken(ctx)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, "wss://ws-auth.kraken.com", nil)
	if err != nil {
		return nil, err
	}
	login := map[string]any{
		"event": "subscribe",
		"subscription": map[string]any{
			"name":  "ownTrades",
			"token": token,
		},
	}
	if err := conn.WriteJSON(login); err != nil {
		_ = conn.Close()
		return nil, err
	}
	for _, topic := range topics {
		ch, _ := parseTopic(topic)
		if ch == "openOrders" {
			payload := map[string]any{
				"event": "subscribe",
				"subscription": map[string]any{
					"name":  "openOrders",
					"token": token,
				},
			}
			if err := conn.WriteJSON(payload); err != nil {
				_ = conn.Close()
				return nil, err
			}
		}
	}
	sub := &wsSub{c: make(chan core.Message, 1024), err: make(chan error, 1), conn: conn}
	go w.readLoopPrivate(sub)
	return sub, nil
}

func (w ws) readLoop(sub *wsSub, requested []string) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now()}
		if err := w.parseMessage(&msg, data, requested); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

func (w ws) readLoopPrivate(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now()}
		if err := w.parsePrivate(&msg, data, nil); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

func (w ws) parseMessage(msg *core.Message, data []byte, requested []string) error {
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

func (w ws) parseV2Channel(msg *core.Message, channel string, env map[string]any, requested []string) error {
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
	canon := w.canonicalSymbol(symbol, requested)
	switch channel {
	case "trade":
		msg.Topic = topicFromChannelName(channel, canon)
		return w.parseTrades(msg, data, canon)
	case "ticker":
		msg.Topic = topicFromChannelName(channel, canon)
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseTicker(msg, payload, canon)
	case "book":
		msg.Topic = topicFromChannelName(channel, canon)
		var payload any
		if len(data) > 0 {
			payload = data[len(data)-1]
		}
		return w.parseBook(msg, payload, canon)
	case "level3":
		msg.Topic = core.BookTopic(canon)
		return w.parseLevel3(msg, data, canon)
	default:
		msg.Topic = topicFromChannelName(channel, canon)
		return nil
	}
}

func (w ws) parsePrivate(msg *core.Message, data []byte, topics []string) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		var arr []any
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		if len(arr) < 4 {
			return nil
		}
		channelName, _ := arr[len(arr)-2].(string)
		payload := arr[2]
		switch channelName {
		case "ownTrades":
			return w.parseOwnTrades(msg, payload)
		case "openOrders":
			return w.parseOpenOrders(msg, payload)
		}
		return nil
	}
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	msg.Topic = fmt.Sprint(env["event"])
	return nil
}

func (w ws) parseOwnTrades(msg *core.Message, payload any) error {
	trades, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for id, raw := range trades {
		rec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		evt := w.orderEventFromTrade(id, rec)
		if evt == nil {
			continue
		}
		msg.Topic = core.OrderTopic(evt.Symbol)
		msg.Event = "order"
		msg.Parsed = evt
	}
	return nil
}

func (w ws) parseOpenOrders(msg *core.Message, payload any) error {
	recMap, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	for id, raw := range recMap {
		rec, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		evt := w.orderEventFromOpenOrder(id, rec)
		if evt == nil {
			continue
		}
		msg.Topic = core.OrderTopic(evt.Symbol)
		msg.Event = "order"
		msg.Parsed = evt
	}
	return nil
}

func (w ws) orderEventFromTrade(id string, rec map[string]any) *core.OrderEvent {
	canon := w.p.mapNativeToCanon(strings.ToUpper(fmt.Sprint(rec["pair"])))
	filled := parseDecimalStr(fmt.Sprint(rec["vol"]))
	avg := parseDecimalStr(fmt.Sprint(rec["price"]))
	status := mapStatus(fmt.Sprint(rec["status"]))
	return &core.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
		Time:      parseTradesTS(fmt.Sprint(rec["time"])),
	}
}

func (w ws) orderEventFromOpenOrder(id string, rec map[string]any) *core.OrderEvent {
	descr, _ := rec["descr"].(map[string]any)
	pair := ""
	if descr != nil {
		pair = fmt.Sprint(descr["pair"])
	}
	canon := w.p.mapNativeToCanon(strings.ToUpper(pair))
	status := mapStatus(fmt.Sprint(rec["status"]))
	priceMap, _ := rec["price"].(map[string]any)
	avg := parseDecimalStr(fmt.Sprint(priceMap["last"]))
	filled := parseDecimalStr(fmt.Sprint(rec["vol_exec"]))
	return &core.OrderEvent{
		Symbol:    canon,
		OrderID:   id,
		Status:    status,
		FilledQty: filled,
		AvgPrice:  avg,
	}
}

func (w ws) parseTrades(msg *core.Message, payload any, symbol string) error {
	rows, ok := payload.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	var events []*core.TradeEvent
	for _, row := range rows {
		rec, ok := row.(map[string]any)
		if !ok {
			continue
		}
		sym := symbol
		if sym == "" {
			if raw := valueString(rec["symbol"]); raw != "" {
				sym = w.canonicalSymbol(raw, nil)
			}
		}
		price := parseDecimalStr(valueString(firstPresent(rec, "price", "px")))
		qty := parseDecimalStr(valueString(firstPresent(rec, "qty", "quantity", "volume")))
		when := parseISOTime(valueString(firstPresent(rec, "timestamp", "time")))
		if when.IsZero() {
			when = time.Now().UTC()
		}
		events = append(events, &core.TradeEvent{Symbol: sym, Price: price, Quantity: qty, Time: when})
	}
	if len(events) > 0 {
		msg.Event = "trade"
		msg.Parsed = events[len(events)-1]
	}
	return nil
}

func (w ws) parseTicker(msg *core.Message, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	bid := parseDecimalStr(valueString(firstPresent(row, "bid", "best_bid")))
	ask := parseDecimalStr(valueString(firstPresent(row, "ask", "best_ask")))
	msg.Event = "ticker"
	msg.Parsed = &core.TickerEvent{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now().UTC()}
	return nil
}

func (w ws) parseBook(msg *core.Message, payload any, symbol string) error {
	row, ok := payload.(map[string]any)
	if !ok {
		return nil
	}
	de := core.DepthEvent{Symbol: symbol, Time: time.Now().UTC()}
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

func (w ws) parseLevel3(msg *core.Message, payload any, symbol string) error {
	rows, ok := payload.([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	last := rows[len(rows)-1]
	rec, ok := last.(map[string]any)
	if !ok {
		return nil
	}
	de := core.DepthEvent{Symbol: symbol, Time: time.Now().UTC()}
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
				lvl := core.DepthLevel{
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

func parseTopic(topic string) (channel, symbol string) {
	parts := strings.Split(topic, ":")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

func topicFromChannelName(name, symbol string) string {
	switch name {
	case "trade":
		if symbol == "" {
			return name
		}
		return core.TradeTopic(symbol)
	case "ticker":
		if symbol == "" {
			return name
		}
		return core.TickerTopic(symbol)
	case "spread", "book":
		if symbol == "" {
			return name
		}
		return core.DepthTopic(symbol)
	case "level3":
		if symbol == "" {
			return name
		}
		return core.BookTopic(symbol)
	case "ownTrades", "openOrders":
		if symbol == "" {
			return name
		}
		return core.OrderTopic(symbol)
	default:
		if symbol == "" {
			return name
		}
		return name + ":" + symbol
	}
}

func appendDepthLevels(dst *[]core.DepthLevel, raw any) {
	levels, ok := raw.([]any)
	if !ok {
		return
	}
	for _, entry := range levels {
		priceStr := ""
		qtyStr := ""
		switch lvl := entry.(type) {
		case []any:
			if len(lvl) > 0 {
				priceStr = valueString(lvl[0])
			}
			if len(lvl) > 1 {
				qtyStr = valueString(lvl[1])
			}
		case map[string]any:
			priceStr = valueString(firstPresent(lvl, "price", "px", "bid", "ask"))
			qtyStr = valueString(firstPresent(lvl, "qty", "quantity", "volume", "size"))
		default:
			continue
		}
		if priceStr == "" {
			continue
		}
		price := parseDecimalStr(priceStr)
		qty := parseDecimalStr(qtyStr)
		*dst = append(*dst, core.DepthLevel{Price: price, Qty: qty})
	}
}

func firstPresent(m map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := m[key]; ok && v != nil {
			return v
		}
	}
	return nil
}

func valueString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func (w ws) canonicalSymbol(exch string, requested []string) string {
	if exch == "" {
		return canonicalFromRequested(exch, requested)
	}
	upper := strings.ToUpper(strings.TrimSpace(exch))
	if w.p != nil {
		if w.p.nativeToCanon != nil {
			if canon := w.p.nativeToCanon[upper]; canon != "" {
				return canon
			}
			if canon := w.p.nativeToCanon[strings.ReplaceAll(upper, "/", "")]; canon != "" {
				return canon
			}
		}
		for c, native := range w.p.canonToKraken {
			if strings.EqualFold(native, upper) {
				return c
			}
		}
		for c, native := range w.p.canonToKrakenWS {
			if strings.EqualFold(native, upper) {
				return c
			}
		}
	}
	return canonicalFromRequested(upper, requested)
}

func canonicalFromRequested(exch string, requested []string) string {
	candidates := []string{exch}
	if strings.Contains(exch, "/") {
		candidates = append(candidates, strings.ReplaceAll(exch, "/", "-"))
	} else if strings.Contains(exch, "-") {
		candidates = append(candidates, strings.ReplaceAll(exch, "-", "/"))
	}
	for _, t := range requested {
		_, sym := parseTopic(t)
		if sym == "" {
			continue
		}
		for _, cand := range candidates {
			if cand != "" && strings.EqualFold(sym, cand) {
				return sym
			}
		}
		if strings.Contains(sym, "-") {
			alt := strings.ReplaceAll(sym, "-", "/")
			for _, cand := range candidates {
				if cand != "" && strings.EqualFold(alt, cand) {
					return sym
				}
			}
		}
	}
	if strings.Contains(exch, "/") {
		return strings.ReplaceAll(exch, "/", "-")
	}
	return exch
}

func parseISOTime(ts string) time.Time {
	ts = strings.TrimSpace(ts)
	if ts == "" {
		return time.Time{}
	}
	layouts := []string{time.RFC3339Nano, time.RFC3339}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t.UTC()
		}
	}
	return parseTradesTS(ts)
}

func parseTradesTS(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	f, err := strconv.ParseFloat(ts, 64)
	if err != nil {
		return time.Time{}
	}
	sec := int64(f)
	ns := int64((f - float64(sec)) * 1e9)
	return time.Unix(sec, ns).UTC()
}
