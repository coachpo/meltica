package ws

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

const (
	wsEndpoint         = "wss://ws-feed.exchange.coinbase.com"
	wsHandshakeTimeout = 10 * time.Second
	wsBufferSize       = 256
)

// Provider interface defines what the ws package needs from the main provider
type Provider interface {
	// For symbol conversion
	Native(symbol string) string
	CanonicalFromNative(native string) string
	// For ensuring instruments are loaded
	EnsureInstruments(ctx context.Context) error
	// For private authentication
	APIKey() string
	Secret() string
	Passphrase() string
}

// WS implements the core.WS interface for Coinbase
type WS struct {
	p          Provider
	orderBooks *OrderBookManager
}

// New creates a new WebSocket handler for Coinbase
func New(p Provider) *WS {
	return &WS{
		p:          p,
		orderBooks: NewOrderBookManager(),
	}
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

func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		conn: conn,
		c:    make(chan core.Message, wsBufferSize),
		err:  make(chan error, 1),
	}
}

func (w *WS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("coinbase: no topics provided")
	}
	if err := w.p.EnsureInstruments(ctx); err != nil {
		return nil, err
	}
	conn, err := w.dial(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.sendSubscriptions(conn, topics, false); err != nil {
		_ = conn.Close()
		return nil, err
	}
	sub := newWSSub(conn)
	go w.readLoop(sub, false)
	return sub, nil
}

func (w *WS) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	if w.p.APIKey() == "" || w.p.Secret() == "" || w.p.Passphrase() == "" {
		return nil, core.ErrNotSupported
	}
	if err := w.p.EnsureInstruments(ctx); err != nil {
		return nil, err
	}
	conn, err := w.dial(ctx)
	if err != nil {
		return nil, err
	}
	if err := w.sendSubscriptions(conn, topics, true); err != nil {
		_ = conn.Close()
		return nil, err
	}
	sub := newWSSub(conn)
	go w.readLoop(sub, true)
	return sub, nil
}

func (w *WS) dial(ctx context.Context) (*websocket.Conn, error) {
	dialer := websocket.Dialer{Proxy: http.ProxyFromEnvironment, HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, wsEndpoint, nil)
	return conn, err
}

func (w *WS) readLoop(sub *wsSub, private bool) {
	defer close(sub.c)
	defer close(sub.err)
	for {
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			sub.err <- err
			return
		}
		msg := core.Message{Raw: data, At: time.Now().UTC()}
		if private {
			err = w.parsePrivateMessage(&msg, data)
		} else {
			err = w.parsePublicMessage(&msg, data)
		}
		if err != nil {
			sub.err <- err
			return
		}
		if msg.Topic != "" {
			sub.c <- msg
		}
	}
}

func (w *WS) nativeProduct(symbol string) string {
	if symbol == "" {
		return symbol
	}
	if native := w.p.Native(symbol); native != "" {
		return native
	}
	return symbol
}

func (w *WS) canonicalSymbol(native string) string {
	if native == "" {
		return native
	}
	if canon := w.p.CanonicalFromNative(native); canon != "" {
		return canon
	}
	return native
}

func (w *WS) sendSubscriptions(conn *websocket.Conn, topics []string, private bool) error {
	channels := map[string][]string{}
	for _, topic := range topics {
		channel, symbol := parseTopic(topic)
		if channel == "" || symbol == "" {
			continue
		}
		native := w.nativeProduct(symbol)
		providerChannel := mapper.ToProviderChannel(channel)
		if providerChannel == "user" && !private {
			continue
		}
		if channel == corews.TopicUserBalance && private {
			channels[providerChannel] = append(channels[providerChannel], "*")
			continue
		}
		if native != "" {
			channels[providerChannel] = append(channels[providerChannel], native)
		}
	}
	payload := map[string]any{
		"type":        "subscribe",
		"product_ids": uniqueFlatten(channels),
		"channels":    buildChannels(channels),
	}
	if private {
		sig, err := buildWSSignature(w.p.APIKey(), w.p.Secret(), w.p.Passphrase())
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
	for channel, products := range target {
		if len(products) == 0 {
			continue
		}
		out = append(out, map[string]any{
			"name":        channel,
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
