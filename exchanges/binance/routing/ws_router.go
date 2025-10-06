package routing

import (
	"context"
	"strings"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/internal"
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

// WSRouter manages websocket subscriptions, routing raw frames from Level 1 to Level 3.
type WSRouter struct {
	infra coretransport.StreamClient
	deps  WSDependencies
}

// NewWSRouter creates a websocket routing layer bound to the infrastructure client.
func NewWSRouter(infra coretransport.StreamClient, deps WSDependencies) *WSRouter {
	return &WSRouter{
		infra: infra,
		deps:  deps,
	}
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
	topicsForInfra := make([]coretransport.StreamTopic, len(streams))
	for i, stream := range streams {
		topicsForInfra[i] = coretransport.StreamTopic{Scope: coretransport.StreamScopePublic, Name: stream}
	}
	rawSub, err := w.infra.Subscribe(ctx, topicsForInfra...)
	if err != nil {
		return nil, err
	}
	sub := newWSSub(rawSub)

	go w.readLoop(sub)
	return sub, nil
}

// SubscribePrivate subscribes to the Binance user data stream and routes messages.
func (w *WSRouter) SubscribePrivate(ctx context.Context) (Subscription, error) {
	listenKey, err := w.deps.CreateListenKey(ctx)
	if err != nil {
		return nil, err
	}
	topic := coretransport.StreamTopic{Scope: coretransport.StreamScopePrivate, Name: listenKey}
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
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: corestreams.RouteUnknown}
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
			msg := RoutedMessage{Raw: raw.Data, At: raw.At, Route: corestreams.RouteUnknown}
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

// Close terminates the infrastructure client.
func (w *WSRouter) Close() error {
	if w.infra != nil {
		return w.infra.Close()
	}
	return nil
}

func (w *WSRouter) buildStreams(topics []string) ([]string, error) {
	streams := make([]string, 0, len(topics))
	for _, topic := range topics {
		channel, instrument, err := coretopics.Parse(topic)
		if err != nil {
			return nil, err
		}
		exchangeChannel := mapper.ExchangeChannelID(channel)
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
