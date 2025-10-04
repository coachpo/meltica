package routing

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance/infra/wsinfra"
)

// WSDependencies exposes the provider hooks required by the websocket routing layer.
type WSDependencies interface {
	CanonicalSymbol(binanceSymbol string) string
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
	DepthSnapshot(ctx context.Context, symbol string, limit int) (coreprovider.BookEvent, int64, error)
}

type RoutedMessage = coreprovider.RoutedMessage

type Subscription = coreprovider.Subscription

// WSRouter manages websocket subscriptions, routing raw frames from Level 1 to Level 3.
type WSRouter struct {
	infra         *wsinfra.Client
	deps          WSDependencies
	orderBooks    *OrderBookManager
	monitorCtx    context.Context
	monitorCancel context.CancelFunc
}

// NewWSRouter creates a websocket routing layer bound to the infrastructure client.
func NewWSRouter(infra *wsinfra.Client, deps WSDependencies) *WSRouter {
	ctx, cancel := context.WithCancel(context.Background())
	return &WSRouter{
		infra:         infra,
		deps:          deps,
		orderBooks:    NewOrderBookManager(),
		monitorCtx:    ctx,
		monitorCancel: cancel,
	}
}

type wsSub struct {
	raw *wsinfra.Subscription
	c   chan RoutedMessage
	err chan error
}

func newWSSub(raw *wsinfra.Subscription) *wsSub {
	return &wsSub{
		raw: raw,
		c:   make(chan RoutedMessage, 1024),
		err: make(chan error, 1),
	}
}

func (s *wsSub) C() <-chan RoutedMessage { return s.c }
func (s *wsSub) Err() <-chan error       { return s.err }

func (s *wsSub) Close() error {
	if s.raw != nil {
		return s.raw.Close()
	}
	return nil
}

// SubscribePublic subscribes to Binance combined streams and routes messages to Level 3.
func (w *WSRouter) SubscribePublic(ctx context.Context, topics ...string) (Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("binance/wsrouter: no topics provided")
	}
	streams := w.buildStreams(topics)
	if len(streams) == 0 {
		return nil, fmt.Errorf("binance/wsrouter: no streams derived from topics")
	}
	rawSub, err := w.infra.SubscribePublic(ctx, streams)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)

	bookSymbols := w.extractBookSymbols(topics)
	if len(bookSymbols) > 0 {
		w.startMonitoring(bookSymbols)
	}

	go w.readLoop(sub)
	return sub, nil
}

// SubscribePrivate subscribes to the Binance user data stream and routes messages.
func (w *WSRouter) SubscribePrivate(ctx context.Context) (Subscription, error) {
	listenKey, err := w.deps.CreateListenKey(ctx)
	if err != nil {
		return nil, err
	}
	rawSub, err := w.infra.SubscribePrivate(ctx, listenKey)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)

	go w.readPrivateLoop(ctx, sub, listenKey)
	return &wsPrivateWrapper{sub: sub, deps: w.deps, listenKey: listenKey}, nil
}

func (w *WSRouter) readLoop(sub *wsSub) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Raw()
	errCh := sub.raw.Err()

	for {
		select {
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: coreprovider.RouteUnknown}
			if err := w.parsePublicMessage(&msg, raw.Data); err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
			}
			select {
			case sub.c <- msg:
			default:
				// Backpressure handling: drop oldest by discarding channel element
				select {
				case <-sub.c:
				default:
				}
				sub.c <- msg
			}
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				select {
				case sub.err <- err:
				default:
				}
			}
			return
		}
	}
}

func (w *WSRouter) readPrivateLoop(ctx context.Context, sub *wsSub, listenKey string) {
	defer close(sub.c)
	defer close(sub.err)

	rawCh := sub.raw.Raw()
	errCh := sub.raw.Err()

	keepAliveTicker := time.NewTicker(30 * time.Minute)
	defer keepAliveTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepAliveTicker.C:
			_ = w.deps.KeepAliveListenKey(context.Background(), listenKey)
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: coreprovider.RouteUnknown}
			if err := w.parsePrivateMessage(&msg, raw.Data); err != nil {
				select {
				case sub.err <- err:
				default:
				}
				return
			}
			sub.c <- msg
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				select {
				case sub.err <- err:
				default:
				}
			}
			return
		}
	}
}

// Close terminates background monitoring goroutines.
func (w *WSRouter) Close() error {
	if w.monitorCancel != nil {
		w.monitorCancel()
	}
	return nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (w *WSRouter) WSNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance ws: unsupported canonical symbol %s", canonical))
}

func (w *WSRouter) WSCanonicalSymbol(native string) string {
	return w.deps.CanonicalSymbol(native)
}

func (w *WSRouter) buildStreams(topics []string) []string {
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

// OrderBookSnapshot returns the current snapshot for a symbol if it has been initialized.
func (w *WSRouter) OrderBookSnapshot(symbol string) (coreprovider.BookEvent, bool) {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if !orderBook.IsInitialized() {
		return coreprovider.BookEvent{}, false
	}
	return orderBook.GetSnapshot(), true
}

// InitializeOrderBook demonstrates the complete Binance order book initialization flow.
func (w *WSRouter) InitializeOrderBook(ctx context.Context, symbol string) error {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if orderBook.IsInitialized() {
		return nil
	}
	snapshot, lastUpdateID, err := w.deps.DepthSnapshot(ctx, symbol, 5000)
	if err != nil {
		return fmt.Errorf("failed to get depth snapshot: %w", err)
	}
	if err := orderBook.InitializeFromSnapshot(snapshot, lastUpdateID); err != nil {
		return fmt.Errorf("failed to initialize order book: %w", err)
	}
	return nil
}

func (w *WSRouter) initializeAndRecover(symbol string, firstUpdateID, lastUpdateID int64) {
	log.Printf("Starting automatic order book recovery for %s (gap detected: first=%d, last=%d)", symbol, firstUpdateID, lastUpdateID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	firstEventU := firstUpdateID
	if firstEventU == 0 {
		firstEventU = -1
	}

	const maxRetries = 5
	var retryCount int

	for retryCount < maxRetries {
		retryCount++

		if err := w.InitializeOrderBook(ctx, symbol); err != nil {
			log.Printf("Failed to initialize order book for %s (attempt %d/%d): %v", symbol, retryCount, maxRetries, err)
			if retryCount < maxRetries {
				time.Sleep(time.Duration(retryCount) * time.Second)
				continue
			}
			log.Printf("Giving up on order book recovery for %s after %d attempts", symbol, maxRetries)
			return
		}

		orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
		if firstEventU > 0 && orderBook.GetLastUpdateID() < firstEventU {
			log.Printf("Snapshot too old for %s (snapshot=%d < firstEvent=%d), retrying...", symbol, orderBook.GetLastUpdateID(), firstEventU)
			if retryCount < maxRetries {
				time.Sleep(time.Duration(retryCount) * time.Second)
				continue
			}
			log.Printf("Giving up on order book recovery for %s - cannot get recent enough snapshot", symbol)
			return
		}

		log.Printf("Successfully recovered order book for %s (snapshot update ID: %d)", symbol, orderBook.GetLastUpdateID())
		return
	}
}

func (w *WSRouter) validateOrderBookState(symbol string) {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if !orderBook.IsInitialized() {
		log.Printf("Order book for %s is not initialized, triggering recovery", symbol)
		go w.initializeAndRecover(symbol, 0, 0)
		return
	}

	if time.Since(orderBook.LastUpdate) > 2*time.Minute {
		log.Printf("Order book for %s is stale (last update: %v), reinitializing", symbol, orderBook.LastUpdate)
		go w.initializeAndRecover(symbol, 0, 0)
	}
}

func (w *WSRouter) startMonitoring(symbols []string) {
	if len(symbols) == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-w.monitorCtx.Done():
				return
			case <-ticker.C:
				for _, symbol := range symbols {
					w.validateOrderBookState(symbol)
				}
			}
		}
	}()
}

func (w *WSRouter) extractBookSymbols(topics []string) []string {
	var symbols []string
	for _, topic := range topics {
		channel, symbol := corews.ParseTopic(topic)
		if channel == corews.TopicBook && symbol != "" {
			symbols = append(symbols, symbol)
		}
	}
	return symbols
}

type wsPrivateWrapper struct {
	sub       *wsSub
	deps      WSDependencies
	listenKey string
}

func (w *wsPrivateWrapper) C() <-chan RoutedMessage { return w.sub.C() }
func (w *wsPrivateWrapper) Err() <-chan error       { return w.sub.Err() }

func (w *wsPrivateWrapper) Close() error {
	w.deps.CloseListenKey(context.Background(), w.listenKey)
	return w.sub.Close()
}
