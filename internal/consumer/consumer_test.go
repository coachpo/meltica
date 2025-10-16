package consumer

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/schema"
)

func TestNewConsumer(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	if consumer == nil {
		t.Fatal("expected non-nil consumer")
	}
	if consumer.ID != "test-consumer" {
		t.Errorf("expected ID 'test-consumer', got %s", consumer.ID)
	}
	if consumer.bus == nil {
		t.Error("expected bus to be set")
	}
	if consumer.logger == nil {
		t.Error("expected logger to be set")
	}
}

func TestConsumerStart(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	events, errs := consumer.Start(ctx, []schema.EventType{"TRADE", "TICKER"})

	if events == nil {
		t.Fatal("expected non-nil events channel")
	}
	if errs == nil {
		t.Fatal("expected non-nil errors channel")
	}

	// Give time for subscriptions
	time.Sleep(50 * time.Millisecond)

	// Should have 2 subscriptions
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 2 {
		t.Errorf("expected 2 subscriptions, got %d", subCount)
	}
}

func TestConsumerStartIdempotent(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	ctx := context.Background()

	events1, errs1 := consumer.Start(ctx, []schema.EventType{"TRADE"})
	events2, errs2 := consumer.Start(ctx, []schema.EventType{"TICKER"})

	// Should return same channels
	if events1 != events2 {
		t.Error("expected Start to be idempotent (same events channel)")
	}
	if errs1 != errs2 {
		t.Error("expected Start to be idempotent (same errors channel)")
	}
}

func TestConsumerReceiveEvent(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	events, _ := consumer.Start(ctx, []schema.EventType{"TRADE"})

	// Give time for subscription
	time.Sleep(50 * time.Millisecond)

	// Publish event
	testEvent := &schema.Event{
		EventID:  "test-1",
		Provider: "test",
		Symbol:   "BTC-USD",
		Type:     "TRADE",
		TraceID:  "trace-123",
	}

	if err := mockBus.PublishSync(ctx, testEvent, time.Second); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}

	// Should receive the event
	select {
	case evt := <-events:
		if evt == nil {
			t.Error("received nil event")
		} else if evt.EventID != "test-1" {
			t.Errorf("expected event ID 'test-1', got %s", evt.EventID)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestConsumerLogEvent(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	event := &schema.Event{
		EventID: "evt-1",
		TraceID: "trace-1",
	}

	// Should not panic
	consumer.logEvent("TRADE", event)
	consumer.logEvent("TRADE", nil) // nil event
}

func TestConsumerLogEventWithPayloadTraceID(t *testing.T) {
	mockBus := NewMockDataBus()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	consumer := NewConsumer("test-consumer", mockBus, logger)

	// Event with trace_id in payload
	event := &schema.Event{
		EventID: "evt-1",
		Payload: map[string]any{
			"trace_id": "trace-from-payload",
		},
	}

	// Should extract trace_id from payload
	consumer.logEvent("TRADE", event)

	// Event with traceId (camelCase) in payload
	event2 := &schema.Event{
		EventID: "evt-2",
		Payload: map[string]any{
			"traceId": "trace-from-payload-camel",
		},
	}

	consumer.logEvent("TICKER", event2)
}

func TestConsumerLogEventNilLogger(t *testing.T) {
	mockBus := NewMockDataBus()

	consumer := NewConsumer("test-consumer", mockBus, nil)

	event := &schema.Event{
		EventID: "evt-1",
		TraceID: "trace-1",
	}

	// Should not panic with nil logger
	consumer.logEvent("TRADE", event)
}
