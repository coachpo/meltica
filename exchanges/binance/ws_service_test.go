package binance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
)

type stubSubscription struct {
	c   chan corestreams.RoutedMessage
	err chan error
}

func newStubSubscription() *stubSubscription {
	return &stubSubscription{
		c:   make(chan corestreams.RoutedMessage),
		err: make(chan error),
	}
}

func (s *stubSubscription) C() <-chan corestreams.RoutedMessage { return s.c }
func (s *stubSubscription) Err() <-chan error                   { return s.err }
func (s *stubSubscription) Close() error {
	close(s.c)
	close(s.err)
	return nil
}

type testRouter struct {
	sub           bnrouting.Subscription
	privateCalled bool
}

func (r *testRouter) SubscribePublic(context.Context, ...string) (bnrouting.Subscription, error) {
	return nil, errors.New("not implemented")
}
func (r *testRouter) SubscribePrivate(context.Context) (bnrouting.Subscription, error) {
	r.privateCalled = true
	return r.sub, nil
}
func (r *testRouter) Close() error { return nil }
func (r *testRouter) OrderBookSnapshot(string) (corestreams.BookEvent, bool) {
	return corestreams.BookEvent{}, false
}
func (r *testRouter) InitializeOrderBook(context.Context, string) error { return nil }

type publicRouter struct {
	topics []string
	sub    bnrouting.Subscription
}

func (r *publicRouter) SubscribePublic(_ context.Context, topics ...string) (bnrouting.Subscription, error) {
	r.topics = append(r.topics, topics...)
	return r.sub, nil
}

func (r *publicRouter) SubscribePrivate(context.Context) (bnrouting.Subscription, error) {
	return nil, errors.New("not implemented")
}

func (r *publicRouter) Close() error { return nil }

func (r *publicRouter) OrderBookSnapshot(string) (corestreams.BookEvent, bool) {
	return corestreams.BookEvent{}, false
}

func (r *publicRouter) InitializeOrderBook(context.Context, string) error { return nil }

func TestSubscribePrivateRejectsTopics(t *testing.T) {
	svc := newWSService(&testRouter{})
	if _, err := svc.SubscribePrivate(context.Background(), "mkt.trade:BTC-USDT"); !errors.Is(err, core.ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}
}

func TestSubscribePrivateDelegatesToRouter(t *testing.T) {
	router := &testRouter{sub: newStubSubscription()}
	svc := newWSService(router)
	sub, err := svc.SubscribePrivate(context.Background())
	if err != nil {
		t.Fatalf("SubscribePrivate returned error: %v", err)
	}
	if sub == nil {
		t.Fatalf("expected subscription instance")
	}
	if !router.privateCalled {
		t.Fatalf("expected router SubscribePrivate to be called")
	}
}

func TestSubscribePublicLifecycle(t *testing.T) {
	stub := newStubSubscription()
	router := &publicRouter{sub: stub}
	svc := newWSService(router)
	topic := core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT")
	sub, err := svc.SubscribePublic(context.Background(), topic)
	if err != nil {
		t.Fatalf("SubscribePublic returned error: %v", err)
	}
	if sub == nil {
		t.Fatalf("expected subscription instance")
	}
	if len(router.topics) != 1 || router.topics[0] != topic {
		t.Fatalf("expected topic %s to be forwarded, got %v", topic, router.topics)
	}

	go func() {
		stub.c <- corestreams.RoutedMessage{Topic: topic, Route: corestreams.RouteTradeUpdate}
	}()

	select {
	case msg := <-sub.C():
		if msg.Topic != topic {
			t.Fatalf("unexpected topic %s", msg.Topic)
		}
		if msg.Event != corestreams.RouteTradeUpdate {
			t.Fatalf("unexpected event %s", msg.Event)
		}
	case <-time.After(time.Second):
		t.Fatal("expected forwarded message")
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}
