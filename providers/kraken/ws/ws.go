package ws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/gorilla/websocket"
)

const (
	publicWSURL        = "wss://ws.kraken.com/v2"
	privateWSURL       = "wss://ws-auth.kraken.com"
	wsHandshakeTimeout = 10 * time.Second
)

// Provider interface defines what the ws package needs from the main provider
type Provider interface {
	// For symbol conversion
	NativeSymbolForWS(canon string) string
	CanonicalSymbol(exch string, requested []string) string
	MapNativeToCanon(native string) string
	// For ensuring instruments are loaded
	EnsureInstruments(ctx context.Context) error
	// For private authentication
	APIKey() string
	Secret() string
	GetWSToken(ctx context.Context) (string, error)
}

// WS implements the core.WS interface for Kraken
type WS struct {
	p Provider
}

// New creates a new WebSocket handler for Kraken
func New(p Provider) *WS {
	return &WS{p: p}
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
func (w *WS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	if err := w.p.EnsureInstruments(ctx); err != nil {
		return nil, err
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, publicWSURL, nil)
	if err != nil {
		return nil, err
	}
	// Build subscriptions.
	for _, topic := range topics {
		ch, canon := corews.ParseTopic(topic)
		if ch == "" || canon == "" {
			continue
		}
		native := strings.ReplaceAll(w.p.NativeSymbolForWS(canon), "-", "/")
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
	sub := newWSSub(conn)
	go w.readLoop(sub, topics)
	return sub, nil
}

// SubscribePrivate connects to the authenticated websocket stream and subscribes to private topics.
func (w *WS) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	if w.p.APIKey() == "" || w.p.Secret() == "" {
		return nil, core.ErrNotSupported
	}
	token, err := w.p.GetWSToken(ctx)
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
			"name":  KRKTopicOwnTrades,
			"token": token,
		},
	}
	if err := conn.WriteJSON(login); err != nil {
		_ = conn.Close()
		return nil, err
	}
	for _, topic := range topics {
		ch, _ := corews.ParseTopic(topic)
		if ch != KRKTopicOpenOrders {
			continue
		}
		payload := map[string]any{
			"event": "subscribe",
			"subscription": map[string]any{
				"name":  KRKTopicOpenOrders,
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
func (w *WS) readLoop(sub *wsSub, requested []string) {
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
func (w *WS) readLoopPrivate(sub *wsSub) {
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
