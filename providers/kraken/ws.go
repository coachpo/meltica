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
	conn, _, err := dialer.DialContext(ctx, "wss://ws.kraken.com", nil)
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
		payload := map[string]any{
			"event": "subscribe",
			"pair":  []string{native},
			"subscription": map[string]any{
				"name": ch,
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
	// Kraken WS responses are arrays or objects depending on event type.
	if len(data) == 0 {
		return nil
	}
	if data[0] == '[' {
		var arr []any
		if err := json.Unmarshal(data, &arr); err != nil {
			return err
		}
		if len(arr) < 3 {
			return nil
		}
		channelName, _ := arr[len(arr)-2].(map[string]any)
		subInfo, ok := arr[len(arr)-1].(string)
		if !ok {
			return nil
		}
		canon := w.canonicalSymbol(subInfo, requested)
		if canon == "" {
			canon = subInfo
		}
		topic := topicFromChannel(channelName, canon)
		msg.Topic = topic
		eventType := channelNameName(channelName)
		switch eventType {
		case "trade":
			return w.parseTrades(msg, arr[1], canon)
		case "ticker":
			return w.parseTicker(msg, arr[1], canon)
		case "book", "spread":
			return w.parseBook(msg, arr[1], canon)
		}
		return nil
	}
	// handle system messages
	var env map[string]any
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}
	msg.Topic = fmt.Sprint(env["event"])
	return nil
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
		rec, ok := row.([]any)
		if !ok || len(rec) < 3 {
			continue
		}
		price := parseDecimalStr(fmt.Sprint(rec[0]))
		qty := parseDecimalStr(fmt.Sprint(rec[1]))
		when := parseTradesTS(fmt.Sprint(rec[2]))
		events = append(events, &core.TradeEvent{Symbol: symbol, Price: price, Quantity: qty, Time: when})
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
	bid := parseDecimalStr(extractFirst(row, "b"))
	ask := parseDecimalStr(extractFirst(row, "a"))
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
	if bids, ok := row["b"].([]any); ok {
		for _, b := range bids {
			if lvl, ok := b.([]any); ok && len(lvl) >= 2 {
				price := parseDecimalStr(fmt.Sprint(lvl[0]))
				qty := parseDecimalStr(fmt.Sprint(lvl[1]))
				de.Bids = append(de.Bids, core.DepthLevel{Price: price, Qty: qty})
			}
		}
	}
	if asks, ok := row["a"].([]any); ok {
		for _, a := range asks {
			if lvl, ok := a.([]any); ok && len(lvl) >= 2 {
				price := parseDecimalStr(fmt.Sprint(lvl[0]))
				qty := parseDecimalStr(fmt.Sprint(lvl[1]))
				de.Asks = append(de.Asks, core.DepthLevel{Price: price, Qty: qty})
			}
		}
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

func topicFromChannel(info map[string]any, symbol string) string {
	if info == nil {
		return ""
	}
	name, _ := info["name"].(string)
	switch name {
	case "trade":
		return core.TradeTopic(symbol)
	case "ticker":
		return core.TickerTopic(symbol)
	case "spread", "book":
		return core.DepthTopic(symbol)
	case "ownTrades":
		return core.OrderTopic(symbol)
	case "openOrders":
		return core.OrderTopic(symbol)
	default:
		if symbol == "" {
			return name
		}
		return name + ":" + symbol
	}
}

func channelNameName(info map[string]any) string {
	if info == nil {
		return ""
	}
	name, _ := info["name"].(string)
	return name
}

func extractFirst(row map[string]any, key string) string {
	vals, ok := row[key].([]any)
	if !ok || len(vals) == 0 {
		return ""
	}
	inner, ok := vals[0].([]any)
	if ok && len(inner) > 0 {
		return fmt.Sprint(inner[0])
	}
	return fmt.Sprint(vals[0])
}

func (w ws) canonicalSymbol(exch string, requested []string) string {
	if w.p == nil {
		return canonicalFromRequested(exch, requested)
	}
	canon := ""
	for c, native := range w.p.canonToKraken {
		if strings.EqualFold(native, exch) {
			canon = c
			break
		}
	}
	if canon != "" {
		return canon
	}
	return canonicalFromRequested(exch, requested)
}

func canonicalFromRequested(exch string, requested []string) string {
	for _, t := range requested {
		_, sym := parseTopic(t)
		if strings.EqualFold(sym, exch) {
			return sym
		}
	}
	return exch
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
