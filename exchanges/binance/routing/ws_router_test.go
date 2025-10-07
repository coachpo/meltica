package routing

import (
	"context"
	"strings"
	"testing"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretopics "github.com/coachpo/meltica/core/topics"
	coretransport "github.com/coachpo/meltica/core/transport"
)

type stubDeps struct {
	listenKey  string
	closeCalls int
}

func (d *stubDeps) CanonicalSymbol(binanceSymbol string) (string, error) {
	return strings.ToUpper(binanceSymbol), nil
}

func (d *stubDeps) NativeSymbol(canonical string) (string, error) {
	return strings.ReplaceAll(strings.ToUpper(canonical), "-", ""), nil
}

func (d *stubDeps) CreateListenKey(ctx context.Context) (string, error) {
	if d.listenKey == "" {
		d.listenKey = "listen-key"
	}
	return d.listenKey, nil
}

func (d *stubDeps) KeepAliveListenKey(ctx context.Context, key string) error { return nil }

func (d *stubDeps) CloseListenKey(ctx context.Context, key string) error {
	d.closeCalls++
	return nil
}

func (d *stubDeps) OrderBookDepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	return corestreams.BookEvent{}, 0, nil
}

func newMockSubscription() *mocks.StreamSubscription {
	messages := make(chan coretransport.RawMessage)
	errorsCh := make(chan error)
	close(messages)
	close(errorsCh)
	return &mocks.StreamSubscription{
		MessagesFunc: func() <-chan coretransport.RawMessage { return messages },
		ErrorsFunc:   func() <-chan error { return errorsCh },
		CloseFunc:    func() error { return nil },
	}
}

func TestWSRouterSubscribePublicUsesStreamClient(t *testing.T) {
	ctx := context.Background()
	deps := &stubDeps{}
	var captured [][]coretransport.StreamTopic
	streamClient := &mocks.StreamClient{}
	streamClient.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		topicsCopy := make([]coretransport.StreamTopic, len(topics))
		copy(topicsCopy, topics)
		captured = append(captured, topicsCopy)
		return newMockSubscription(), nil
	}
	router := NewWSRouter(streamClient, deps)
	defer router.Close()

	sub, err := router.SubscribePublic(ctx, coretopics.Trade("BTC-USDT"))
	if err != nil {
		t.Fatalf("subscribe public failed: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription")
	}
	_ = sub.Close()

	if len(captured) != 1 {
		t.Fatalf("expected 1 subscribe call, got %d", len(captured))
	}
	topics := captured[0]
	if len(topics) != 1 {
		t.Fatalf("expected single topic, got %d", len(topics))
	}
	topic := topics[0]
	if topic.Scope != coretransport.StreamScopePublic {
		t.Fatalf("expected public scope, got %s", topic.Scope)
	}
	if topic.Name != "btcusdt@trade" {
		t.Fatalf("unexpected stream name: %s", topic.Name)
	}
}

func TestWSRouterSubscribePrivateUsesStreamClient(t *testing.T) {
	ctx := context.Background()
	deps := &stubDeps{listenKey: "private-key"}
	var captured [][]coretransport.StreamTopic
	streamClient := &mocks.StreamClient{}
	streamClient.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		topicsCopy := make([]coretransport.StreamTopic, len(topics))
		copy(topicsCopy, topics)
		captured = append(captured, topicsCopy)
		return newMockSubscription(), nil
	}
	router := NewWSRouter(streamClient, deps)
	defer router.Close()

	sub, err := router.SubscribePrivate(ctx)
	if err != nil {
		t.Fatalf("subscribe private failed: %v", err)
	}
	if sub == nil {
		t.Fatal("expected subscription")
	}
	if len(captured) != 1 || len(captured[0]) != 1 {
		t.Fatalf("expected exactly one topic capture, got %#v", captured)
	}
	topic := captured[0][0]
	if topic.Scope != coretransport.StreamScopePrivate {
		t.Fatalf("expected private scope, got %s", topic.Scope)
	}
	if topic.Name != deps.listenKey {
		t.Fatalf("expected listen key %s, got %s", deps.listenKey, topic.Name)
	}

	if err := sub.Close(); err != nil {
		t.Fatalf("closing subscription failed: %v", err)
	}
	if deps.closeCalls == 0 {
		t.Fatalf("expected listen key to be closed")
	}
}

func TestWSRouterCloseClosesStreamClient(t *testing.T) {
	// WSRouter.Close() no longer closes the infrastructure client directly.
	// The infrastructure client is managed by the TransportBundle.
	// This test now verifies that Close() returns without error.
	deps := &stubDeps{}
	streamClient := &mocks.StreamClient{}
	router := NewWSRouter(streamClient, deps)
	if err := router.Close(); err != nil {
		t.Fatalf("close returned error: %v", err)
	}
}
