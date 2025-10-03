package ws

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"time"

	"strings"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/gorilla/websocket"
)

const (
	publicWSURL        = "wss://ws.okx.com:8443/ws/v5/public"
	privateWSURL       = "wss://ws.okx.com:8443/ws/v5/private"
	wsHandshakeTimeout = 10 * time.Second
)

// Provider interface defines what the ws package needs from the main provider
type Provider interface {
	// For private authentication
	APIKey() string
	Secret() string
	Passphrase() string
}

// WS implements the core.WS interface for OKX
type WS struct {
	p          Provider
	orderBooks *OrderBookManager
}

// New creates a new WebSocket handler for OKX
func New(p Provider) *WS {
	return &WS{
		p:          p,
		orderBooks: NewOrderBookManager(),
	}
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

func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		c:    make(chan core.Message, 1024),
		err:  make(chan error, 1),
		conn: conn,
	}
}

// SubscribePublic connects to the OKX public websocket and subscribes to requested topics.
func (w *WS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, publicWSURL, nil)
	if err != nil {
		return nil, err
	}
	payload := wsSubscribePayload{
		Op:   "subscribe",
		Args: w.buildSubscriptionArgs(topics),
	}
	if err := conn.WriteJSON(payload); err != nil {
		_ = conn.Close()
		return nil, err
	}
	sub := newWSSub(conn)
	go w.readLoopPublic(sub)
	return sub, nil
}

// SubscribePrivate connects to the OKX private websocket and subscribes to private topics.
func (w *WS) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, privateWSURL, nil)
	if err != nil {
		return nil, err
	}
	if err := w.writePrivateLogin(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if len(topics) > 0 {
		payload := wsSubscribePayload{
			Op:   "subscribe",
			Args: w.buildSubscriptionArgs(topics),
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

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (w *WS) WSNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx ws: unsupported canonical symbol %s", canonical))
}

func (w *WS) WSCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx ws: unsupported native symbol %s", native))
}

func (w *WS) readLoopPublic(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now()}
		if err := w.parsePublicMessage(&msg, data); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

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
		if err := w.parsePrivateMessage(&msg, data); err != nil {
			sub.err <- err
			return
		}
		sub.c <- msg
	}
}

func (w *WS) buildSubscriptionArgs(topics []string) []map[string]string {
	args := make([]map[string]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := corews.ParseTopic(topic)
		providerChannel := mapper.ToProviderChannel(channel)
		if providerChannel == "" {
			continue
		}
		arg := map[string]string{"channel": providerChannel}
		if instrument != "" {
			arg["instId"] = instrument
		}
		args = append(args, arg)
	}
	return args
}

func (w *WS) writePrivateLogin(conn *websocket.Conn) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	pre := ts + "GET" + "/users/self/verify"
	mac := hmac.New(sha256.New, []byte(w.p.Secret()))
	mac.Write([]byte(pre))
	sign := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	payload := map[string]any{
		"op": "login",
		"args": []map[string]string{{
			"apiKey":     w.p.APIKey(),
			"passphrase": w.p.Passphrase(),
			"timestamp":  ts,
			"sign":       sign,
		}},
	}
	return conn.WriteJSON(payload)
}

type wsSubscribePayload struct {
	Op   string              `json:"op"`
	Args []map[string]string `json:"args"`
}
