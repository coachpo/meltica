package consumer

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

// Test Lambda helper methods

func TestLambdaSetAndGetRoutingVersion(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockBus, pm, logger)

	// Initial version should be 0
	if version := lambda.GetRoutingVersion(); version != 0 {
		t.Errorf("expected initial version 0, got %d", version)
	}

	// Set version
	lambda.SetRoutingVersion(5)

	if version := lambda.GetRoutingVersion(); version != 5 {
		t.Errorf("expected version 5, got %d", version)
	}

	// Set another version
	lambda.SetRoutingVersion(10)

	if version := lambda.GetRoutingVersion(); version != 10 {
		t.Errorf("expected version 10, got %d", version)
	}
}

func TestLambdaTradingEnabled(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockBus, pm, logger)

	// Initial state should be false
	if lambda.TradingEnabled() {
		t.Error("expected trading to be disabled initially")
	}
}

func TestLambdaShouldIgnore(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockBus, pm, logger)

	tests := []struct {
		name      string
		eventType schema.EventType
		event     *schema.Event
		expected  bool
	}{
		{
			name:      "nil event",
			eventType: "TRADE",
			event:     nil,
			expected:  true,
		},
		{
			name:      "empty event ID but version OK",
			eventType: "TRADE",
			event: &schema.Event{
				EventID:        "",
				RoutingVersion: 1,
			},
			expected: false, // Not ignored if version is valid
		},
		{
			name:      "valid event",
			eventType: "TRADE",
			event: &schema.Event{
				EventID: "evt-123",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lambda.shouldIgnore(tt.eventType, tt.event)
			if result != tt.expected {
				t.Errorf("expected shouldIgnore=%v, got %v", tt.expected, result)
			}
		})
	}
}

func TestIsCritical(t *testing.T) {
	tests := []struct {
		name      string
		eventType schema.EventType
		expected  bool
	}{
		{"CONTROL_ACK is critical", schema.EventTypeControlAck, true},
		{"CONTROL_RESULT is critical", schema.EventTypeControlResult, true},
		{"EXEC_REPORT is critical", schema.EventTypeExecReport, true},
		{"KLINE_SUMMARY is NOT critical", schema.EventTypeKlineSummary, false},
		{"TRADE is not critical", schema.EventTypeTrade, false},
		{"TICKER is not critical", schema.EventTypeTicker, false},
		{"BOOK_UPDATE is not critical", schema.EventTypeBookUpdate, false},
		{"BOOK_SNAPSHOT is not critical", schema.EventTypeBookSnapshot, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isCritical(tt.eventType)
			if result != tt.expected {
				t.Errorf("expected isCritical(%s)=%v, got %v", tt.eventType, tt.expected, result)
			}
		})
	}
}

func TestIsMarketData(t *testing.T) {
	tests := []struct {
		name      string
		eventType schema.EventType
		expected  bool
	}{
		{"TRADE is market data", schema.EventTypeTrade, true},
		{"TICKER is market data", schema.EventTypeTicker, true},
		{"BOOK_UPDATE is market data", schema.EventTypeBookUpdate, true},
		{"BOOK_SNAPSHOT is market data", schema.EventTypeBookSnapshot, true},
		{"KLINE_SUMMARY is market data", schema.EventTypeKlineSummary, true},
		{"CONTROL_ACK is not market data", schema.EventTypeControlAck, false},
		{"CONTROL_RESULT is not market data", schema.EventTypeControlResult, false},
		{"EXEC_REPORT is not market data", schema.EventTypeExecReport, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isMarketData(tt.eventType)
			if result != tt.expected {
				t.Errorf("expected isMarketData(%s)=%v, got %v", tt.eventType, tt.expected, result)
			}
		})
	}
}

// Test control message wrapper methods

func TestLambdaSubscribe(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	payload := schema.Subscribe{
		Type:    "TICKER",
		Filters: map[string]any{"symbol": "BTC-USD"},
	}

	ack, err := lambda.Subscribe(ctx, payload)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != schema.ControlMessageSubscribe {
		t.Errorf("expected SUBSCRIBE type, got %s", messages[0].Type)
	}
}

func TestLambdaUnsubscribe(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	payload := schema.Unsubscribe{
		Type: "TICKER",
	}

	ack, err := lambda.Unsubscribe(ctx, payload)
	if err != nil {
		t.Fatalf("Unsubscribe failed: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != schema.ControlMessageUnsubscribe {
		t.Errorf("expected UNSUBSCRIBE type, got %s", messages[0].Type)
	}
}

func TestLambdaSetTradingMode(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	payload := schema.TradingModePayload{
		Enabled: true,
	}

	ack, err := lambda.SetTradingMode(ctx, payload)
	if err != nil {
		t.Fatalf("SetTradingMode failed: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != schema.ControlMessageSetTradingMode {
		t.Errorf("expected TRADING_MODE type, got %s", messages[0].Type)
	}
}

func TestLambdaSubmitOrder(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	payload := schema.SubmitOrderPayload{
		ClientOrderID: "order-123",
		Symbol:        "BTC-USD",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Quantity:      "1.0",
	}

	ack, err := lambda.SubmitOrder(ctx, payload)
	if err != nil {
		t.Fatalf("SubmitOrder failed: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != schema.ControlMessageSubmitOrder {
		t.Errorf("expected SUBMIT_ORDER type, got %s", messages[0].Type)
	}
}

func TestLambdaQueryOrder(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewLambda("test", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")

	ctx := context.Background()

	payload := schema.QueryOrderPayload{
		ClientOrderID: "order-123",
	}

	ack, err := lambda.QueryOrder(ctx, payload)
	if err != nil {
		t.Fatalf("QueryOrder failed: %v", err)
	}

	if !ack.Success {
		t.Error("expected successful acknowledgement")
	}

	messages := mockControlBus.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}

	if messages[0].Type != schema.ControlMessageQueryOrder {
		t.Errorf("expected QUERY_ORDER type, got %s", messages[0].Type)
	}
}
