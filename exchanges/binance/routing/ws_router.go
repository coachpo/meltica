package routing

import (
	"context"

	corestreams "github.com/coachpo/meltica/core/streams"
	coretransport "github.com/coachpo/meltica/core/transport"
)

// WSDependencies exposes the exchange hooks required by the websocket routing layer.
type WSDependencies interface {
	corestreams.OrderBookSnapshotProvider
	CanonicalSymbol(binanceSymbol string) (string, error)
	NativeSymbol(canonical string) (string, error)
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
}

type RoutedMessage = corestreams.RoutedMessage

type Subscription = corestreams.Subscription

// WSRouter manages websocket subscriptions using Public and Private dispatchers.
type WSRouter struct {
	public  *PublicDispatcher
	private *PrivateDispatcher
}

// NewWSRouter creates a websocket routing layer bound to the infrastructure client.
func NewWSRouter(infra coretransport.StreamClient, deps WSDependencies) *WSRouter {
	return &WSRouter{
		public:  NewPublicDispatcher(infra, deps),
		private: NewPrivateDispatcher(infra, deps),
	}
}

// SubscribePublic subscribes to public streams via the public dispatcher.
func (w *WSRouter) SubscribePublic(ctx context.Context, topics ...string) (Subscription, error) {
	return w.public.Subscribe(ctx, topics...)
}

// SubscribePrivate subscribes to the private user data stream via the private dispatcher.
func (w *WSRouter) SubscribePrivate(ctx context.Context) (Subscription, error) {
	return w.private.Subscribe(ctx)
}

// Close terminates the router.
func (w *WSRouter) Close() error {
	// Dispatchers don't hold persistent resources; infrastructure client is closed elsewhere
	return nil
}

type wsSub struct {
	raw coretransport.StreamSubscription
	c   chan RoutedMessage
	err chan error
}

func newWSSub(raw coretransport.StreamSubscription) *wsSub {
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
