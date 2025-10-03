package ws

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/gorilla/websocket"
)

const (
	publicStreamURL    = "wss://stream.binance.com:9443/stream"
	wsHandshakeTimeout = 10 * time.Second
)

// Provider interface defines what the ws package needs from the main provider
type Provider interface {
	// For symbol conversion
	CanonicalSymbol(binanceSymbol string) string
	// For private streams
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
}

// WS implements the core.WS interface for Binance
type WS struct {
	p Provider
}

// New creates a new WebSocket handler for Binance
func New(p Provider) *WS {
	return &WS{
		p: p,
	}
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

func newWSSub(conn *websocket.Conn) *wsSub {
	return &wsSub{
		c:    make(chan core.Message, 1024),
		err:  make(chan error, 1),
		conn: conn,
	}
}

func (w *WS) SubscribePublic(ctx context.Context, topics ...core.Topic) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topics provided")
	}
	streams := w.buildStreams(topics)
	if len(streams) == 0 {
		return nil, fmt.Errorf("no streams derived from topics")
	}
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	endpoint := publicStreamURL + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(conn)
	go w.readLoop(sub)
	return sub, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (w *WS) WSNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance ws: unsupported canonical symbol %s", canonical))
}

func (w *WS) WSCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance ws: unsupported native symbol %s", native))
}

func (w *WS) buildStreams(topics []core.Topic) []string {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := corews.ParseTopic(string(topic))
		providerChannel := mapper.ToProviderChannel(channel)
		if instrument == "" {
			streams = append(streams, string(topic))
			continue
		}
		if providerChannel == "" {
			providerChannel = strings.ToLower(channel)
		}
		binanceSymbol := strings.ToLower(core.CanonicalToBinance(instrument))
		streams = append(streams, binanceSymbol+"@"+providerChannel)
	}
	return streams
}

func (w *WS) readLoop(sub *wsSub) {
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
