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
	p          Provider
	orderBooks *OrderBookManager
}

// New creates a new WebSocket handler for Binance
func New(p Provider) *WS {
	return &WS{
		p:          p,
		orderBooks: NewOrderBookManager(),
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

func (w *WS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
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
	return w.p.CanonicalSymbol(native)
}

func (w *WS) buildStreams(topics []string) []string {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := corews.ParseTopic(topic)
		providerChannel := mapper.ToProviderChannel(channel)
		if instrument == "" {
			streams = append(streams, topic)
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

// InitializeOrderBook demonstrates the complete Binance order book initialization flow
// This would typically be called when starting to subscribe to depth streams
func (w *WS) InitializeOrderBook(ctx context.Context, symbol string) error {
	// Step 1: Get order book for this symbol
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)

	// Step 2: If already initialized, no need to do anything
	if orderBook.IsInitialized() {
		return nil
	}

	// Step 3: Get depth snapshot from REST API
	// Note: In a real implementation, we would need access to the provider's REST client
	// For now, this demonstrates the flow
	snapshot, lastUpdateID, err := w.getDepthSnapshot(ctx, symbol, 5000)
	if err != nil {
		return fmt.Errorf("failed to get depth snapshot: %w", err)
	}

	// Step 4: Initialize order book with snapshot
	if err := orderBook.InitializeFromSnapshot(snapshot, lastUpdateID); err != nil {
		return fmt.Errorf("failed to initialize order book: %w", err)
	}

	return nil
}

// getDepthSnapshot is a placeholder method that would fetch the actual snapshot
// In a real implementation, this would call the provider's REST API
func (w *WS) getDepthSnapshot(ctx context.Context, symbol string, limit int) (corews.BookEvent, int64, error) {
	// This is a placeholder - in reality we would call:
	// w.p.Spot().DepthSnapshot(ctx, symbol, limit)
	// But we don't have access to the spot API from the WS struct

	// For now, return an empty book event with placeholder update ID
	return corews.BookEvent{
		Symbol: symbol,
		Bids:   []corews.DepthLevel{},
		Asks:   []corews.DepthLevel{},
		Time:   time.Now(),
	}, 0, nil
}
