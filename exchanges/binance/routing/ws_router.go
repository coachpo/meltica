package routing

import (
	"context"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/layers"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
	bnwsrouting "github.com/coachpo/meltica/exchanges/binance/wsrouting"
	"github.com/coachpo/meltica/exchanges/processors"
)

type streamClientProvider interface {
	LegacyStreamClient() coretransport.StreamClient
}

// WSDependencies exposes the exchange hooks required by the websocket routing layer.
type WSDependencies interface {
	corestreams.BookSnapshotProvider
	CanonicalSymbol(binanceSymbol string) (string, error)
	NativeSymbol(canonical string) (string, error)
	NativeTopic(topic core.Topic) (string, error)
	CreateListenKey(ctx context.Context) (string, error)
	KeepAliveListenKey(ctx context.Context, key string) error
	CloseListenKey(ctx context.Context, key string) error
}

type RoutedMessage = corestreams.RoutedMessage

type Subscription = corestreams.Subscription

// WSRouter manages websocket subscriptions using Public and Private dispatchers.
type WSRouter struct {
	public     *PublicDispatcher
	private    *PrivateDispatcher
	hub        *processorHub
	table      *bnwsrouting.RoutingTable
	dispatcher *bnwsrouting.RouterDispatcher
}

// NewWSRouter creates a websocket routing layer bound to the infrastructure client.
func NewWSRouter(conn layers.WSConnection, deps WSDependencies) *WSRouter {
	if conn == nil {
		panic("binance routing: nil WSConnection")
	}
	provider, ok := conn.(streamClientProvider)
	if !ok {
		panic("binance routing: WSConnection missing legacy stream client provider")
	}
	infra := provider.LegacyStreamClient()
	if infra == nil {
		panic("binance routing: legacy stream client required")
	}

	ctx := context.Background()
	table := bnwsrouting.NewRoutingTable()
	descriptors := BinanceMessageTypeDescriptors()

	if err := registerBinanceProcessors(table, deps, descriptors); err != nil {
		panic(err)
	}

	metrics := bnwsrouting.NewRoutingMetrics()
	dispatcher := bnwsrouting.NewRouterDispatcher(ctx, metrics)
	hub := newProcessorHub(ctx, table, dispatcher, descriptors)

	return &WSRouter{
		public:     NewPublicDispatcher(infra, deps, table, dispatcher, hub),
		private:    NewPrivateDispatcher(infra, deps, table, dispatcher, hub),
		hub:        hub,
		table:      table,
		dispatcher: dispatcher,
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
	if w.hub != nil {
		w.hub.Close()
	}
	if w.dispatcher != nil {
		w.dispatcher.Shutdown()
	}
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

func registerBinanceProcessors(table *bnwsrouting.RoutingTable, deps WSDependencies, descriptors []*bnwsrouting.MessageTypeDescriptor) error {
	for _, desc := range descriptors {
		var proc processors.Processor
		switch desc.ID {
		case "binance.trade":
			proc = NewTradeProcessorAdapter(deps)
		case "binance.orderbook":
			proc = NewOrderBookProcessorAdapter(deps)
		case "binance.ticker":
			proc = NewTickerProcessorAdapter(deps)
		case "binance.user.order":
			proc = NewOrderUpdateProcessorAdapter()
		case "binance.user.balance":
			proc = NewBalanceProcessorAdapter()
		default:
			return errs.New("", errs.CodeExchange, errs.WithMessage("unknown binance descriptor"))
		}
		if err := table.Register(desc, proc); err != nil {
			return err
		}
	}
	return nil
}

// RegisterProcessors registers Binance message type descriptors and processors with the provided routing table.
func RegisterProcessors(table *bnwsrouting.RoutingTable, deps WSDependencies) error {
	if table == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("routing table required"))
	}
	return registerBinanceProcessors(table, deps, BinanceMessageTypeDescriptors())
}

// AsLayerInterface exposes the router as a layers.WSRouting implementation.
func (w *WSRouter) AsLayerInterface() layers.WSRouting {
	if w == nil {
		return nil
	}
	return &layerWSRouting{router: w}
}
