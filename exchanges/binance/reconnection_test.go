package binance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
)

type stubRoutingSubscription struct {
	messages chan corestreams.RoutedMessage
	errors   chan error
	closed   bool
}

func newStubRoutingSubscription(buffer int) *stubRoutingSubscription {
	return &stubRoutingSubscription{
		messages: make(chan corestreams.RoutedMessage, buffer),
		errors:   make(chan error, 1),
	}
}

func (s *stubRoutingSubscription) C() <-chan corestreams.RoutedMessage { return s.messages }
func (s *stubRoutingSubscription) Err() <-chan error                   { return s.errors }
func (s *stubRoutingSubscription) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.messages)
	close(s.errors)
	return nil
}

type reconnectingRouter struct {
	publicSubs []*stubRoutingSubscription
}

func (r *reconnectingRouter) nextSubscription() *stubRoutingSubscription {
	if len(r.publicSubs) == 0 {
		return nil
	}
	sub := r.publicSubs[0]
	r.publicSubs = r.publicSubs[1:]
	return sub
}

func (r *reconnectingRouter) SubscribePublic(context.Context, ...string) (bnrouting.Subscription, error) {
	sub := r.nextSubscription()
	if sub == nil {
		return nil, errors.New("no subscriptions available")
	}
	return sub, nil
}

func (r *reconnectingRouter) SubscribePrivate(context.Context) (bnrouting.Subscription, error) {
	return nil, errors.New("not implemented")
}

func (r *reconnectingRouter) Close() error { return nil }

func TestWSServerSubscriptionRecoversAfterError(t *testing.T) {
	first := newStubRoutingSubscription(1)
	second := newStubRoutingSubscription(1)
	router := &reconnectingRouter{publicSubs: []*stubRoutingSubscription{first, second}}
	svc := newWSService(router)

	topic := core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT")
	sub, err := svc.SubscribePublic(context.Background(), topic)
	require.NoError(t, err)
	require.NotNil(t, sub)

	first.messages <- corestreams.RoutedMessage{Topic: topic, Route: corestreams.RouteTradeUpdate}
	select {
	case msg := <-sub.C():
		require.Equal(t, corestreams.RouteTradeUpdate, msg.Event)
	case <-time.After(time.Second):
		t.Fatal("expected forwarded message")
	}

	first.errors <- errors.New("connection dropped")
	select {
	case err := <-sub.Err():
		require.Error(t, err)
	case <-time.After(time.Second):
		t.Fatal("expected propagated error")
	}
	require.NoError(t, sub.Close())

	resub, err := svc.SubscribePublic(context.Background(), topic)
	require.NoError(t, err)
	require.NotNil(t, resub)
	second.messages <- corestreams.RoutedMessage{Topic: topic, Route: corestreams.RouteTradeUpdate}
	select {
	case msg := <-resub.C():
		require.Equal(t, corestreams.RouteTradeUpdate, msg.Event)
	case <-time.After(time.Second):
		t.Fatal("expected message after reconnection")
	}
	require.NoError(t, resub.Close())
}
