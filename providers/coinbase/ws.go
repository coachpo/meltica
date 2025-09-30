package coinbase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core"
)

type ws struct {
	p *Provider
}

type wsSub struct {
	conn *websocket.Conn
	c    chan core.Message
	err  chan error
}

func (s *wsSub) C() <-chan core.Message { return s.c }
func (s *wsSub) Err() <-chan error      { return s.err }

func (s *wsSub) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

const wsEndpoint = "wss://ws-feed.exchange.coinbase.com"

func (w ws) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("coinbase: no topics provided")
	}
	if err := w.p.ensureInstruments(ctx); err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := &wsSub{conn: conn, c: make(chan core.Message, 256), err: make(chan error, 1)}
	if err := w.sendSubscriptions(conn, topics, false); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go w.readLoop(sub, topics, false)
	return sub, nil
}

func (w ws) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	if w.p.apiKey == "" || w.p.secret == "" || w.p.passphrase == "" {
		return nil, core.ErrNotSupported
	}
	if err := w.p.ensureInstruments(ctx); err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: 10 * time.Second}
	conn, _, err := dialer.DialContext(ctx, wsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := &wsSub{conn: conn, c: make(chan core.Message, 256), err: make(chan error, 1)}
	if err := w.sendSubscriptions(conn, topics, true); err != nil {
		_ = conn.Close()
		return nil, err
	}
	go w.readLoop(sub, topics, true)
	return sub, nil
}

func (w ws) sendSubscriptions(conn *websocket.Conn, topics []string, private bool) error {
	channels := map[string][]string{}
	for _, topic := range topics {
		parts := strings.Split(topic, ":")
		if len(parts) != 2 {
			continue
		}
		ch := parts[0]
		sym := parts[1]
		native := w.p.native(sym)
		if native == "" {
			native = sym
		}
		switch ch {
		case core.TopicTicker:
			channels["ticker"] = append(channels["ticker"], native)
		case core.TopicTrade:
			channels["matches"] = append(channels["matches"], native)
		case core.TopicDepth, core.TopicFullBook, core.TopicSnapshot5:
			channels["level2"] = append(channels["level2"], native)
		case core.TopicOrder:
			if private {
				channels["user"] = append(channels["user"], native)
			}
		case core.TopicBalance:
			if private {
				channels["user"] = append(channels["user"], "*")
			}
		default:
			channels[ch] = append(channels[ch], native)
		}
	}
	payload := map[string]any{
		"type":        "subscribe",
		"product_ids": uniqueFlatten(channels),
		"channels":    buildChannels(channels),
	}
	if private {
		sig, err := buildWSSignature(w.p.apiKey, w.p.secret, w.p.passphrase)
		if err != nil {
			return err
		}
		for k, v := range sig {
			payload[k] = v
		}
	}
	return conn.WriteJSON(payload)
}

func buildChannels(target map[string][]string) []any {
	if len(target) == 0 {
		return []any{"ticker", "matches", "level2"}
	}
	out := make([]any, 0, len(target))
	for ch, products := range target {
		if len(products) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"name":        ch,
			"product_ids": products,
		})
	}
	return out
}

func uniqueFlatten(ch map[string][]string) []string {
	set := map[string]struct{}{}
	for _, vals := range ch {
		for _, v := range vals {
			if v == "" {
				continue
			}
			set[v] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	return out
}

func (w ws) readLoop(sub *wsSub, topics []string, private bool) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now().UTC()}
		if err := w.parseMessage(&msg, data, topics, private); err != nil {
			sub.err <- err
			return
		}
		if msg.Topic != "" {
			sub.c <- msg
		}
	}
}

func (w ws) parseMessage(msg *core.Message, payload []byte, topics []string, private bool) error {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	typeVal, _ := env["type"].(string)
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

func (w ws) canonicalSymbol(native string) string {
	native = strings.ToUpper(native)
	if native == "" {
		return native
	}
	if w.p == nil {
		return native
	}
	w.p.mu.RLock()
	defer w.p.mu.RUnlock()
	if canon := w.p.nativeToCanon[native]; canon != "" {
		return canon
	}
	return native
}

// TestOnly helpers for unit tests

func TestOnlyParseTicker(payload []byte) (*core.TickerEvent, error) {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	msg := core.Message{}
	s := ws{p: TestOnlyNewProvider()}
	if err := s.parseTicker(&msg, env); err != nil {
		return nil, err
	}
	if ev, ok := msg.Parsed.(*core.TickerEvent); ok {
		return ev, nil
	}
	return nil, fmt.Errorf("no ticker event parsed")
}

func TestOnlyParseMatch(payload []byte) (*core.TradeEvent, error) {
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		return nil, err
	}
	msg := core.Message{}
	s := ws{p: TestOnlyNewProvider()}
	if err := s.parseMatch(&msg, env); err != nil {
		return nil, err
	}
	if ev, ok := msg.Parsed.(*core.TradeEvent); ok {
		return ev, nil
	}
	return nil, fmt.Errorf("no trade event parsed")
}
