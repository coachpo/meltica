package consumer

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

func TestNewLambda(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockBus, pm, logger)

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.id != "test-lambda" {
		t.Errorf("expected id 'test-lambda', got %s", lambda.id)
	}
	if lambda.consumerID != "test-lambda" {
		t.Errorf("expected consumerID 'test-lambda', got %s", lambda.consumerID)
	}
}

func TestNewLambdaDefaultID(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("", mockBus, pm, logger)

	if lambda.id != "lambda-consumer" {
		t.Errorf("expected default id 'lambda-consumer', got %s", lambda.id)
	}
}

func TestLambdaStartWithNilBus(t *testing.T) {
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", nil, pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx, []schema.EventType{"TRADE"})

	if err == nil {
		t.Error("expected error when starting with nil bus")
	}
}

func TestLambdaStartAndSubscribe(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockBus, pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx, []schema.EventType{"TRADE", "TICKER"})
	if err != nil {
		t.Fatalf("failed to start lambda: %v", err)
	}

	// Give it time to subscribe
	time.Sleep(50 * time.Millisecond)

	// Should have subscriptions for TRADE, TICKER, CONTROL_ACK, CONTROL_RESULT
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 4 {
		t.Errorf("expected 4 subscriptions (TRADE, TICKER, CONTROL_ACK, CONTROL_RESULT), got %d", subCount)
	}

	// Check for errors
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Success - no errors
	}

	cancel()
}

// TestLambdaReceiveEvent demonstrates event consumption pattern
// Note: Full event lifecycle testing requires proper pool integration
func TestLambdaReceiveEvent(t *testing.T) {
	t.Skip("Requires full pool setup for event lifecycle - see mock_bus_test.go for pattern")
}

func TestLambdaAttachControlBus(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "custom-consumer-id")

	if lambda.control == nil {
		t.Error("expected control bus to be attached")
	}
	if lambda.consumerID != "custom-consumer-id" {
		t.Errorf("expected consumerID 'custom-consumer-id', got %s", lambda.consumerID)
	}
}

func TestLambdaSendControlMessageWithoutBus(t *testing.T) {
	mockDataBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockDataBus, pm, logger)

	ctx := context.Background()
	_, err := lambda.SendControlMessage(ctx, "SUBSCRIBE", map[string]any{})

	if err == nil {
		t.Error("expected error when sending control message without control bus")
	}
}

func TestLambdaSendControlMessage(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	// Send a subscribe control message
	payload := map[string]any{
		"type":    "TICKER",
		"filters": map[string]any{"symbol": "ETH-USD"},
	}

	ack, err := lambda.SendControlMessage(ctx, "SUBSCRIBE", payload)
	if err != nil {
		t.Fatalf("failed to send control message: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	// Verify message was recorded
	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != "SUBSCRIBE" {
		t.Errorf("expected SUBSCRIBE type, got %s", messages[0].Type)
	}
	if messages[0].ConsumerID != "test-consumer" {
		t.Errorf("expected consumer ID 'test-consumer', got %s", messages[0].ConsumerID)
	}
}

func TestLambdaDuplicateEventTypes(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test-lambda", mockBus, pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Pass duplicate event types
	_, err := lambda.Start(ctx, []schema.EventType{"TRADE", "TRADE", "TICKER"})
	if err != nil {
		t.Fatalf("failed to start lambda: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Should have 4 subscriptions (TRADE, TICKER, CONTROL_ACK, CONTROL_RESULT)
	// Duplicates should be filtered out
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 4 {
		t.Errorf("expected 4 subscriptions (duplicates filtered), got %d", subCount)
	}

	cancel()
}

// TestLambdaMultipleMessages demonstrates the pattern for testing event streams
// Note: Full integration requires proper pool lifecycle management
func TestLambdaMultipleMessages(t *testing.T) {
	t.Skip("Requires full pool setup for event lifecycle - see mock_bus_test.go for pattern")
}
