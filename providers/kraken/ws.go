package kraken

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/gorilla/websocket"
)

const (
	publicWSURL        = "wss://ws.kraken.com/v2"
	privateWSURL       = "wss://ws-auth.kraken.com"
	wsHandshakeTimeout = 10 * time.Second
)

type ws struct {
	p *Provider
}

// wsSub wraps a websocket connection and exposes it as a subscription.
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

// newWSSub constructs a websocket subscription wrapper around the connection.
func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		c:    make(chan core.Message, 1024),
		err:  make(chan error, 1),
		conn: conn,
	}
}

// SubscribePublic connects to the Kraken public websocket and subscribes to the requested topics.
func (w ws) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	if err := w.p.ensureInstruments(ctx); err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, publicWSURL, nil)
	if err != nil {
		return nil, err
	}
	// Build subscriptions.
	for _, topic := range topics {
		ch, canon := parseTopic(topic)
		if ch == "" || canon == "" {
			continue
		}
		channelName := normalizePublicChannel(ch)
		if channelName == "" {
			continue
		}
		native := strings.ReplaceAll(w.nativeSymbolForWS(canon), "-", "/")
		payload := map[string]any{
			"method": "subscribe",
			"params": map[string]any{
				"channel": channelName,
				"symbol":  []string{native},
			},
		}
		if err := conn.WriteJSON(payload); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}
	sub := newWSSub(conn)
	go w.readLoop(sub, topics)
	return sub, nil
}

// SubscribePrivate connects to the authenticated websocket stream and subscribes to private topics.
func (w ws) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	if w.p.apiKey == "" || w.p.secret == "" {
		return nil, core.ErrNotSupported
	}
	token, err := w.p.getWSToken(ctx)
	if err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, privateWSURL, nil)
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
		if ch != "openOrders" {
			continue
		}
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
	sub := newWSSub(conn)
	go w.readLoopPrivate(sub)
	return sub, nil
}

// readLoop processes public websocket messages until the connection closes.
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

// readLoopPrivate processes private websocket messages until the connection closes.
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
