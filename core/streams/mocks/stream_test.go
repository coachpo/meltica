package mocks

import (
	"context"
	"errors"
	"testing"

	coretransport "github.com/coachpo/meltica/core/transport"
)

func TestStreamClientDelegatesToInjectedFunctions(t *testing.T) {
	ctx := context.Background()
	sub := &StreamSubscription{}
	client := &StreamClient{
		ConnectFunc: func(c context.Context) error {
			if c != ctx {
				t.Fatalf("unexpected context")
			}
			return errors.New("connect")
		},
		CloseFunc: func() error { return errors.New("close") },
		SubscribeFunc: func(c context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
			if len(topics) != 1 || topics[0].Name != "ticker" {
				t.Fatalf("unexpected topics: %+v", topics)
			}
			return sub, errors.New("subscribe")
		},
		UnsubscribeFunc: func(c context.Context, s coretransport.StreamSubscription, topics ...coretransport.StreamTopic) error {
			if s != sub {
				t.Fatalf("unexpected subscription")
			}
			return errors.New("unsubscribe")
		},
		PublishFunc: func(c context.Context, msg coretransport.StreamMessage) error {
			if string(msg.Payload) != "{}" {
				t.Fatalf("unexpected payload: %s", msg.Payload)
			}
			return errors.New("publish")
		},
		HandleErrorFunc: func(c context.Context, err error) error {
			return errors.New("handled")
		},
	}

	if err := client.Connect(ctx); err == nil || err.Error() != "connect" {
		t.Fatalf("expected connect delegation, got %v", err)
	}
	if err := client.Close(); err == nil || err.Error() != "close" {
		t.Fatalf("expected close delegation, got %v", err)
	}
	gotSub, err := client.Subscribe(ctx, coretransport.StreamTopic{Name: "ticker"})
	if err == nil || err.Error() != "subscribe" || gotSub != sub {
		t.Fatalf("expected subscribe delegation, got sub=%v err=%v", gotSub, err)
	}
	if err := client.Unsubscribe(ctx, sub); err == nil || err.Error() != "unsubscribe" {
		t.Fatalf("expected unsubscribe delegation, got %v", err)
	}
	if err := client.Publish(ctx, coretransport.StreamMessage{Payload: []byte("{}")}); err == nil || err.Error() != "publish" {
		t.Fatalf("expected publish delegation, got %v", err)
	}
	if err := client.HandleError(ctx, errors.New("boom")); err == nil || err.Error() != "handled" {
		t.Fatalf("expected handle error delegation, got %v", err)
	}
}

func TestStreamClientFallbackBehaviour(t *testing.T) {
	ctx := context.Background()
	client := &StreamClient{}
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("expected no-op connect, got %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("expected no-op close, got %v", err)
	}

	subClosed := false
	sub := &StreamSubscription{CloseFunc: func() error {
		subClosed = true
		return errors.New("closed")
	}}

	if err := client.Unsubscribe(ctx, sub); err == nil || err.Error() != "closed" {
		t.Fatalf("expected fallback unsubscribe to close subscription, got %v", err)
	}
	if !subClosed {
		t.Fatalf("expected subscription close to be invoked")
	}

	if err := client.Publish(ctx, coretransport.StreamMessage{}); err != nil {
		t.Fatalf("expected no-op publish, got %v", err)
	}
	sentinel := errors.New("boom")
	if err := client.HandleError(ctx, sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("expected passthrough error, got %v", err)
	}

	var nilClient *StreamClient
	if err := nilClient.Close(); err != nil {
		t.Fatalf("nil receiver close should be no-op, got %v", err)
	}
	if resp, err := nilClient.Subscribe(ctx, coretransport.StreamTopic{}); err != nil || resp != nil {
		t.Fatalf("nil receiver subscribe should be no-op, got sub=%v err=%v", resp, err)
	}
	if err := nilClient.Publish(ctx, coretransport.StreamMessage{}); err != nil {
		t.Fatalf("nil receiver publish should be no-op: %v", err)
	}
	if err := nilClient.HandleError(ctx, sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("nil receiver should passthrough error, got %v", err)
	}

	messages := (&StreamSubscription{}).Messages()
	if _, ok := <-messages; ok {
		t.Fatalf("expected closed channel for default messages")
	}
	errs := (&StreamSubscription{}).Errors()
	if _, ok := <-errs; ok {
		t.Fatalf("expected closed channel for default errors")
	}
}
