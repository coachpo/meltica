package lambda

import (
	"context"
	"sync"
	"testing"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

// mockStrategy tracks callback invocations for testing
type mockStrategy struct {
	mu                       sync.Mutex
	onTradeCalled            int
	onTickerCalled           int
	onBookSnapshotCalled     int
	onOrderFilledCalled      int
	onOrderRejectedCalled    int
	onPartialFillCalled      int
	onCancelledCalled        int
	onAcknowledgedCalled     int
	onExpiredCalled          int
	lastTradePrice           float64
	lastRejectionReason      string
}

func (m *mockStrategy) OnTrade(_ context.Context, _ *schema.Event, _ schema.TradePayload, price float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTradeCalled++
	m.lastTradePrice = price
}

func (m *mockStrategy) OnTicker(_ context.Context, _ *schema.Event, _ schema.TickerPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onTickerCalled++
}

func (m *mockStrategy) OnBookSnapshot(_ context.Context, _ *schema.Event, _ schema.BookSnapshotPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onBookSnapshotCalled++
}

func (m *mockStrategy) OnOrderFilled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onOrderFilledCalled++
}

func (m *mockStrategy) OnOrderRejected(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload, reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onOrderRejectedCalled++
	m.lastRejectionReason = reason
}

func (m *mockStrategy) OnOrderPartialFill(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPartialFillCalled++
}

func (m *mockStrategy) OnOrderCancelled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onCancelledCalled++
}

func (m *mockStrategy) OnOrderAcknowledged(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAcknowledgedCalled++
}

func (m *mockStrategy) OnOrderExpired(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onExpiredCalled++
}

func (m *mockStrategy) OnKlineSummary(_ context.Context, _ *schema.Event, _ schema.KlineSummaryPayload) {}

func (m *mockStrategy) OnControlAck(_ context.Context, _ *schema.Event, _ schema.ControlAckPayload) {}

func (m *mockStrategy) OnControlResult(_ context.Context, _ *schema.Event, _ schema.ControlResultPayload) {}

func (m *mockStrategy) getCounts() (int, int, int, int, int, int, int, int, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.onTradeCalled, m.onTickerCalled, m.onBookSnapshotCalled,
		m.onOrderFilledCalled, m.onOrderRejectedCalled, m.onPartialFillCalled, m.onCancelledCalled,
		m.onAcknowledgedCalled, m.onExpiredCalled
}

// TestStrategyCallbacksInvoked verifies that base lambda properly delegates to strategy
func TestStrategyCallbacksInvoked(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	pm := pool.NewPoolManager()

	if err := pm.RegisterPool("Event", 10, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}

	strategy := &mockStrategy{}
	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, mockOrderSubmitter, pm, strategy)

	ctx := context.Background()

	// Test OnTrade callback
	lambda.handleTrade(ctx, &schema.Event{
		Type:     schema.EventTypeTrade,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload:  schema.TradePayload{Price: "100.5", Quantity: "1.0", Side: schema.TradeSideBuy},
	})

	// Test OnTicker callback
	lambda.handleTicker(ctx, &schema.Event{
		Type:     schema.EventTypeTicker,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload:  schema.TickerPayload{LastPrice: "100.0", BidPrice: "99.5", AskPrice: "100.5"},
	})

	// Test OnBookSnapshot callback
	lambda.handleBookSnapshot(ctx, &schema.Event{
		Type:     schema.EventTypeBookSnapshot,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.BookSnapshotPayload{
			Bids: []schema.PriceLevel{{Price: "99.0", Quantity: "10"}},
			Asks: []schema.PriceLevel{},
		},
	})

	// Verify callbacks were invoked
	trade, ticker, snapshot, _, _, _, _, _, _ := strategy.getCounts()
	if trade != 1 {
		t.Errorf("expected OnTrade called 1 time, got %d", trade)
	}
	if ticker != 1 {
		t.Errorf("expected OnTicker called 1 time, got %d", ticker)
	}
	if snapshot != 1 {
		t.Errorf("expected OnBookSnapshot called 1 time, got %d", snapshot)
	}
	if strategy.lastTradePrice != 100.5 {
		t.Errorf("expected last trade price 100.5, got %.2f", strategy.lastTradePrice)
	}
}

// TestExecReportCallbacks verifies order lifecycle callbacks
func TestExecReportCallbacks(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	pm := pool.NewPoolManager()

	if err := pm.RegisterPool("Event", 10, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}

	strategy := &mockStrategy{}
	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test-lambda", config, mockDataBus, mockControlBus, mockOrderSubmitter, pm, strategy)

	ctx := context.Background()

	// Test OnOrderFilled
	reason := "INSUFFICIENT_BALANCE"
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID:  "test-lambda-123-1",
			State:          schema.ExecReportStateFILLED,
			FilledQuantity: "1.0",
			AvgFillPrice:   "100.0",
		},
	})

	// Test OnOrderRejected
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID: "test-lambda-123-2",
			State:         schema.ExecReportStateREJECTED,
			RejectReason:  &reason,
		},
	})

	// Test OnOrderCancelled
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID: "test-lambda-123-3",
			State:         schema.ExecReportStateCANCELLED,
		},
	})

	// Test OnOrderPartialFill
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID:  "test-lambda-123-4",
			State:          schema.ExecReportStatePARTIAL,
			FilledQuantity: "0.5",
			RemainingQty:   "0.5",
		},
	})

	// Verify callbacks
	_, _, _, filled, rejected, partial, cancelled, _, _ := strategy.getCounts()
	if filled != 1 {
		t.Errorf("expected OnOrderFilled called 1 time, got %d", filled)
	}
	if rejected != 1 {
		t.Errorf("expected OnOrderRejected called 1 time, got %d", rejected)
	}
	if cancelled != 1 {
		t.Errorf("expected OnOrderCancelled called 1 time, got %d", cancelled)
	}
	if partial != 1 {
		t.Errorf("expected OnOrderPartialFill called 1 time, got %d", partial)
	}
	if strategy.lastRejectionReason != "INSUFFICIENT_BALANCE" {
		t.Errorf("expected rejection reason 'INSUFFICIENT_BALANCE', got '%s'", strategy.lastRejectionReason)
	}
}

// TestExecReportACKAndExpiredInvoked verifies ACK and EXPIRED states trigger their callbacks
func TestExecReportACKAndExpiredInvoked(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	if err := pm.RegisterPool("Event", 10, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}

	strategy := &mockStrategy{}
	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test-lambda", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, strategy)

	ctx := context.Background()

	// Send ACK event
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID: "test-lambda-123-1",
			State:         schema.ExecReportStateACK,
		},
	})

	// Send EXPIRED event
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID: "test-lambda-123-2",
			State:         schema.ExecReportStateEXPIRED,
		},
	})

	// Verify callbacks were invoked
	_, _, _, filled, rejected, partial, cancelled, ack, expired := strategy.getCounts()
	if ack != 1 {
		t.Errorf("expected OnOrderAcknowledged called 1 time, got %d", ack)
	}
	if expired != 1 {
		t.Errorf("expected OnOrderExpired called 1 time, got %d", expired)
	}
	// Verify other callbacks were NOT invoked
	total := filled + rejected + partial + cancelled
	if total != 0 {
		t.Errorf("expected no trading callbacks for ACK/EXPIRED states, got %d total calls", total)
	}
}

// TestMarketStateUpdates verifies market state is properly tracked
func TestMarketStateUpdates(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()

	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), nil, &testNoOpStrategy{})

	ctx := context.Background()

	// Initial state should be zero
	market := lambda.GetMarketState()
	if market.LastPrice != 0 || market.BidPrice != 0 || market.AskPrice != 0 {
		t.Error("expected initial market state to be zero")
	}

	// Update via ticker
	tickerEvt := &schema.Event{
		Type:     schema.EventTypeTicker,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload:  schema.TickerPayload{LastPrice: "100.0", BidPrice: "99.5", AskPrice: "100.5"},
	}
	lambda.handleEvent(ctx, schema.EventTypeTicker, tickerEvt)

	// Verify state updated
	market = lambda.GetMarketState()
	if market.LastPrice != 100.0 {
		t.Errorf("expected last price 100.0, got %.2f", market.LastPrice)
	}
	if market.BidPrice != 99.5 {
		t.Errorf("expected bid price 99.5, got %.2f", market.BidPrice)
	}
	if market.AskPrice != 100.5 {
		t.Errorf("expected ask price 100.5, got %.2f", market.AskPrice)
	}
	if market.Spread != 1.0 {
		t.Errorf("expected spread 1.0, got %.2f", market.Spread)
	}
}

// TestOrderCounterIncrement verifies order counter is properly incremented
func TestOrderCounterIncrement(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	pm := pool.NewPoolManager()

	if err := pm.RegisterPool("OrderRequest", 10, func() interface{} { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}

	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, mockOrderSubmitter, pm, &testNoOpStrategy{})

	ctx := context.Background()

	// Initial count should be 0
	if count := lambda.GetOrderCount(); count != 0 {
		t.Errorf("expected initial order count 0, got %d", count)
	}

	// Submit order
	price := "100.0"
	if err := lambda.SubmitOrder(ctx, schema.TradeSideBuy, "1.0", &price); err != nil {
		t.Fatalf("submit order failed: %v", err)
	}

	// Count should be incremented
	if count := lambda.GetOrderCount(); count != 1 {
		t.Errorf("expected order count 1 after submit, got %d", count)
	}

	// Submit market order
	if err := lambda.SubmitMarketOrder(ctx, schema.TradeSideSell, "0.5"); err != nil {
		t.Fatalf("submit market order failed: %v", err)
	}

	// Count should be incremented again
	if count := lambda.GetOrderCount(); count != 2 {
		t.Errorf("expected order count 2 after second submit, got %d", count)
	}
}

// TestStrategyNotCalledForOtherOrders verifies order ownership filtering
func TestStrategyNotCalledForOtherOrders(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	if err := pm.RegisterPool("Event", 10, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register pool: %v", err)
	}

	strategy := &mockStrategy{}
	config := Config{Symbol: "BTC-USDT", Provider: "fake"}
	lambda := NewBaseLambda("test-lambda", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, strategy)

	ctx := context.Background()

	// ExecReport for another lambda's order
	lambda.handleExecReport(ctx, &schema.Event{
		Type:     schema.EventTypeExecReport,
		Symbol:   "BTC-USDT",
		Provider: "fake",
		Payload: schema.ExecReportPayload{
			ClientOrderID:  "other-lambda-123-1",
			State:          schema.ExecReportStateFILLED,
			FilledQuantity: "1.0",
		},
	})

	// Verify strategy was NOT called
	_, _, _, filled, rejected, partial, cancelled, ack, expired := strategy.getCounts()
	total := filled + rejected + partial + cancelled + ack + expired
	if total != 0 {
		t.Errorf("expected strategy not called for other lambda's order, got %d calls", total)
	}
}
