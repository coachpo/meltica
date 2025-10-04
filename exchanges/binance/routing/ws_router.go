package routing

import (
	"context"
	"log"
	"strings"
	"time"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance/internal"
)

// WSDependencies exposes the exchange hooks required by the websocket routing layer.
type WSDependencies interface {
	CanonicalSymbol(binanceSymbol string) (string, error)
	NativeSymbol(canonical string) (string, error)
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
	DepthSnapshot(ctx context.Context, symbol string, limit int) (coreexchange.BookEvent, int64, error)
}

type RoutedMessage = coreexchange.RoutedMessage

type Subscription = coreexchange.Subscription

// WSRouter manages websocket subscriptions, routing raw frames from Level 1 to Level 3.
type WSRouter struct {
	infra         coreexchange.StreamClient
	deps          WSDependencies
	orderBooks    *OrderBookManager
	monitorCtx    context.Context
	monitorCancel context.CancelFunc
}

// NewWSRouter creates a websocket routing layer bound to the infrastructure client.
func NewWSRouter(infra coreexchange.StreamClient, deps WSDependencies) *WSRouter {
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
	raw coreexchange.StreamSubscription
	c   chan RoutedMessage
	err chan error
}

func newWSSub(raw coreexchange.StreamSubscription) *wsSub {
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
		return nil, internal.Invalid("wsrouter: no topics provided")
	}
	streams, err := w.buildStreams(topics)
	if err != nil {
		return nil, err
	}
	if len(streams) == 0 {
		return nil, internal.Invalid("wsrouter: no streams derived from topics")
	}
	topicsForInfra := make([]coreexchange.StreamTopic, len(streams))
	for i, stream := range streams {
		topicsForInfra[i] = coreexchange.StreamTopic{Scope: coreexchange.StreamScopePublic, Name: stream}
	}
	rawSub, err := w.infra.Subscribe(ctx, topicsForInfra...)
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
	topic := coreexchange.StreamTopic{Scope: coreexchange.StreamScopePrivate, Name: listenKey}
	rawSub, err := w.infra.Subscribe(ctx, topic)
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

	rawCh := sub.raw.Messages()
	errCh := sub.raw.Errors()

	for {
		select {
		case raw, ok := <-rawCh:
			if !ok {
				return
			}
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: coreexchange.RouteUnknown}
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

	rawCh := sub.raw.Messages()
	errCh := sub.raw.Errors()

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
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: coreexchange.RouteUnknown}
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
	if w.infra != nil {
		return w.infra.Close()
	}
	return nil
}

// Symbol conversion helpers consult the exchange-provided registry.
func (w *WSRouter) WSNativeSymbol(canonical string) (string, error) {
	return w.deps.NativeSymbol(canonical)
}

func (w *WSRouter) WSCanonicalSymbol(native string) (string, error) {
	return w.deps.CanonicalSymbol(native)
}

func (w *WSRouter) buildStreams(topics []string) ([]string, error) {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument := coretopics.Parse(topic)
		exchangeChannel := mapper.ToExchange(channel)
		if instrument == "" {
			streams = append(streams, topic)
			continue
		}
		if exchangeChannel == "" {
			exchangeChannel = strings.ToLower(channel)
		}
		native, err := w.deps.NativeSymbol(instrument)
		if err != nil {
			return nil, err
		}
		streams = append(streams, strings.ToLower(native)+"@"+exchangeChannel)
	}
	return streams, nil
}

// OrderBookSnapshot returns the current snapshot for a symbol if it has been initialized.
func (w *WSRouter) OrderBookSnapshot(symbol string) (coreexchange.BookEvent, bool) {
	orderBook := w.orderBooks.GetOrCreateOrderBook(symbol)
	if !orderBook.IsInitialized() {
		return coreexchange.BookEvent{}, false
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
		return internal.WrapExchange(err, "failed to get depth snapshot")
	}
	if err := orderBook.InitializeFromSnapshot(snapshot, lastUpdateID); err != nil {
		return internal.WrapExchange(err, "failed to initialize order book")
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
		channel, symbol := coretopics.Parse(topic)
		if channel == coretopics.TopicBook && symbol != "" {
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
