package lambda

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

func TestNewBaseLambda(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test-lambda", config, mockDataBus, mockControlBus, mockOrderSubmitter, pm, &testNoOpStrategy{})

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.ID() != "test-lambda" {
		t.Errorf("expected id 'test-lambda', got %s", lambda.ID())
	}
	if lambda.Config().Symbol != "BTC-USDT" {
		t.Errorf("expected symbol 'BTC-USDT', got %s", lambda.Config().Symbol)
	}
	if lambda.Config().Provider != "fake" {
		t.Errorf("expected provider 'fake', got %s", lambda.Config().Provider)
	}
}

func TestNewLambdaDefaultID(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "ETH-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	// Default ID should be generated as "lambda-{symbol}-{provider}"
	expectedID := "lambda-ETH-USDT-fake"
	if lambda.ID() != expectedID {
		t.Errorf("expected default id '%s', got '%s'", expectedID, lambda.ID())
	}
}

func TestLambdaStartWithNilBus(t *testing.T) {
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test", config, nil, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil data bus")
	}
}

func TestLambdaStartWithNilControlBus(t *testing.T) {
	mockDataBus := NewMockDataBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test", config, mockDataBus, nil, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil control bus")
	}
}

func TestLambdaStartAndSubscribe(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test-lambda", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start lambda: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	subCount := mockDataBus.GetActiveSubscriptions()
	if subCount != 5 {
		t.Errorf("expected 5 subscriptions (TRADE, TICKER, BOOK_SNAPSHOT, BOOK_UPDATE, EXEC_REPORT), got %d", subCount)
	}

	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
}

func TestLambdaEnableTrading(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	if lambda.tradingActive.Load() {
		t.Error("expected trading to be disabled initially")
	}

	lambda.EnableTrading(true)

	if !lambda.tradingActive.Load() {
		t.Error("expected trading to be enabled after EnableTrading(true)")
	}

	lambda.EnableTrading(false)

	if lambda.tradingActive.Load() {
		t.Error("expected trading to be disabled after EnableTrading(false)")
	}
}

func TestLambdaMatchesSymbol(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"matching symbol", "BTC-USDT", true},
		{"non-matching symbol", "ETH-USDT", false},
		{"empty symbol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &schema.Event{Symbol: tt.symbol}
			result := lambda.matchesSymbol(evt)
			if result != tt.expected {
				t.Errorf("expected matchesSymbol=%v for symbol %s, got %v", tt.expected, tt.symbol, result)
			}
		})
	}
}

func TestLambdaMatchesProvider(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		{"matching provider", "fake", true},
		{"non-matching provider", "binance", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &schema.Event{Provider: tt.provider}
			result := lambda.matchesProvider(evt)
			if result != tt.expected {
				t.Errorf("expected matchesProvider=%v for provider %s, got %v", tt.expected, tt.provider, result)
			}
		})
	}
}

func TestLambdaMatchesProviderEmpty(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()

	config := Config{
		Symbol:   "BTC-USDT",
		Provider: "",
	}

	lambda := NewBaseLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, &testNoOpStrategy{})

	evt := &schema.Event{Provider: "any-provider"}
	if !lambda.matchesProvider(evt) {
		t.Error("expected empty provider config to match any provider")
	}
}

// TestIsMyOrder verifies that lambdas only process their own orders
func TestIsMyOrder(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()

	config1 := Config{Symbol: "BTC-USDT", Provider: "fake"}
	config2 := Config{Symbol: "BTC-USDT", Provider: "fake"}

	lambda1 := NewBaseLambda("lambda-1", config1, mockDataBus, mockControlBus, mockOrderSubmitter, nil, &testNoOpStrategy{})
	lambda2 := NewBaseLambda("lambda-2", config2, mockDataBus, mockControlBus, mockOrderSubmitter, nil, &testNoOpStrategy{})

	tests := []struct {
		name        string
		lambda      *BaseLambda
		clientOrder string
		expected    bool
	}{
		{"lambda1 recognizes its order", lambda1, "lambda-1-1234567890-1", true},
		{"lambda1 rejects lambda2 order", lambda1, "lambda-2-1234567890-1", false},
		{"lambda2 recognizes its order", lambda2, "lambda-2-9876543210-5", true},
		{"lambda2 rejects lambda1 order", lambda2, "lambda-1-9876543210-5", false},
		{"empty order ID", lambda1, "", false},
		{"malformed order ID", lambda1, "random-order-123", false},
		{"lambda1 with exact prefix match", lambda1, "lambda-1-", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.lambda.IsMyOrder(tt.clientOrder)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for order %s", tt.expected, result, tt.clientOrder)
			}
		})
	}
}

// TestMultipleLambdasSameSymbol demonstrates that two lambdas trading the same symbol
// only process their own ExecReports
func TestMultipleLambdasSameSymbol(t *testing.T) {
	t.Skip("Integration test - demonstrates order isolation concept")

	// Scenario: Two lambdas both trading BTC-USDT
	// Lambda1 submits order: "lambda-1-timestamp-1"
	// Lambda2 submits order: "lambda-2-timestamp-1"
	//
	// When ExecReport comes back for "lambda-1-timestamp-1":
	// - Lambda1 processes it ✓
	// - Lambda2 ignores it ✓
	//
	// When ExecReport comes back for "lambda-2-timestamp-1":
	// - Lambda1 ignores it ✓
	// - Lambda2 processes it ✓
}

// Mock implementations

// MockDataBus provides a testable data bus implementation
type MockDataBus struct {
	mu            sync.Mutex
	subscriptions map[databus.SubscriptionID]*mockSubscription
	nextID        int
	publishedMu   sync.Mutex
	published     []*schema.Event
}

type mockSubscription struct {
	ID      databus.SubscriptionID
	Type    schema.EventType
	Channel chan *schema.Event
	Active  bool
}

// NewMockDataBus creates a new mock data bus for testing
func NewMockDataBus() *MockDataBus {
	return &MockDataBus{
		subscriptions: make(map[databus.SubscriptionID]*mockSubscription),
		published:     make([]*schema.Event, 0),
	}
}

// Subscribe implements databus.Bus
func (m *MockDataBus) Subscribe(ctx context.Context, typ schema.EventType) (databus.SubscriptionID, <-chan *schema.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := databus.SubscriptionID(fmt.Sprintf("sub-%d", m.nextID))
	m.nextID++

	ch := make(chan *schema.Event, 10)
	sub := &mockSubscription{
		ID:      id,
		Type:    typ,
		Channel: ch,
		Active:  true,
	}
	m.subscriptions[id] = sub

	return id, ch, nil
}

// Unsubscribe implements databus.Bus
func (m *MockDataBus) Unsubscribe(id databus.SubscriptionID) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sub, ok := m.subscriptions[id]; ok {
		sub.Active = false
		close(sub.Channel)
		delete(m.subscriptions, id)
	}
}

// Publish implements databus.Bus
func (m *MockDataBus) Publish(ctx context.Context, event *schema.Event) error {
	if event == nil {
		return fmt.Errorf("nil event")
	}

	m.publishedMu.Lock()
	m.published = append(m.published, event)
	m.publishedMu.Unlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range m.subscriptions {
		if sub.Active && sub.Type == event.Type {
			select {
			case sub.Channel <- event:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Non-blocking send
			}
		}
	}
	return nil
}

// Close implements databus.Bus
func (m *MockDataBus) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sub := range m.subscriptions {
		if sub.Active {
			sub.Active = false
			close(sub.Channel)
		}
	}
	m.subscriptions = make(map[databus.SubscriptionID]*mockSubscription)
}

// PublishSync publishes event and waits for it to be delivered (test helper)
func (m *MockDataBus) PublishSync(ctx context.Context, event *schema.Event, timeout time.Duration) error {
	m.mu.Lock()
	subs := make([]*mockSubscription, 0)
	for _, sub := range m.subscriptions {
		if sub.Active && sub.Type == event.Type {
			subs = append(subs, sub)
		}
	}
	m.mu.Unlock()

	for _, sub := range subs {
		select {
		case sub.Channel <- event:
		case <-time.After(timeout):
			return fmt.Errorf("timeout publishing to %s", sub.ID)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// GetPublishedEvents returns all published events (test helper)
func (m *MockDataBus) GetPublishedEvents() []*schema.Event {
	m.publishedMu.Lock()
	defer m.publishedMu.Unlock()

	result := make([]*schema.Event, len(m.published))
	copy(result, m.published)
	return result
}

// GetActiveSubscriptions returns count of active subscriptions (test helper)
func (m *MockDataBus) GetActiveSubscriptions() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.subscriptions)
}

// MockControlBus provides a testable control bus implementation
type MockControlBus struct {
	mu        sync.Mutex
	messages  []schema.ControlMessage
	acks      map[string]schema.ControlAcknowledgement
	consumers chan controlbus.Message
	autoReply bool
}

// NewMockControlBus creates a new mock control bus for testing
func NewMockControlBus() *MockControlBus {
	return &MockControlBus{
		messages:  make([]schema.ControlMessage, 0),
		acks:      make(map[string]schema.ControlAcknowledgement),
		consumers: make(chan controlbus.Message, 10),
		autoReply: true,
	}
}

// Send implements controlbus.Bus
func (m *MockControlBus) Send(ctx context.Context, msg schema.ControlMessage) (schema.ControlAcknowledgement, error) {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()

	// Simulate immediate acknowledgement if auto-reply is enabled
	if m.autoReply {
		ack := schema.ControlAcknowledgement{
			MessageID:      msg.MessageID,
			ConsumerID:     msg.ConsumerID,
			Success:        true,
			RoutingVersion: 1,
			Timestamp:      time.Now(),
		}

		m.mu.Lock()
		m.acks[msg.MessageID] = ack
		m.mu.Unlock()

		return ack, nil
	}

	// Wait for manual ack if auto-reply disabled
	select {
	case <-ctx.Done():
		return schema.ControlAcknowledgement{}, ctx.Err()
	case <-time.After(5 * time.Second):
		return schema.ControlAcknowledgement{}, fmt.Errorf("timeout waiting for ack")
	}
}

// Consume implements controlbus.Bus
func (m *MockControlBus) Consume(ctx context.Context) (<-chan controlbus.Message, error) {
	return m.consumers, nil
}

// Close implements controlbus.Bus
func (m *MockControlBus) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()

	close(m.consumers)
}

// GetMessages returns all sent messages (test helper)
func (m *MockControlBus) GetMessages() []schema.ControlMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]schema.ControlMessage, len(m.messages))
	copy(result, m.messages)
	return result
}

// GetMessageCount returns count of sent messages (test helper)
func (m *MockControlBus) GetMessageCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	return len(m.messages)
}

// SetAutoReply controls whether Send automatically returns acks (test helper)
func (m *MockControlBus) SetAutoReply(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.autoReply = enabled
}

// GetLastMessage returns the most recent message (test helper)
func (m *MockControlBus) GetLastMessage() *schema.ControlMessage {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.messages) == 0 {
		return nil
	}
	return &m.messages[len(m.messages)-1]
}

// MockOrderSubmitter implements OrderSubmitter for testing.
type MockOrderSubmitter struct {
	mu     sync.Mutex
	orders []schema.OrderRequest
	err    error
}

func NewMockOrderSubmitter() *MockOrderSubmitter {
	return &MockOrderSubmitter{
		orders: make([]schema.OrderRequest, 0),
	}
}

func (m *MockOrderSubmitter) SubmitOrder(_ context.Context, req schema.OrderRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return m.err
	}
	m.orders = append(m.orders, req)
	return nil
}

func (m *MockOrderSubmitter) GetOrders() []schema.OrderRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	orders := make([]schema.OrderRequest, len(m.orders))
	copy(orders, m.orders)
	return orders
}

func (m *MockOrderSubmitter) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// testNoOpStrategy is a test-only no-op strategy
type testNoOpStrategy struct{}

func (s *testNoOpStrategy) OnTrade(_ context.Context, _ *schema.Event, _ schema.TradePayload, _ float64) {}
func (s *testNoOpStrategy) OnTicker(_ context.Context, _ *schema.Event, _ schema.TickerPayload)           {}
func (s *testNoOpStrategy) OnBookSnapshot(_ context.Context, _ *schema.Event, _ schema.BookSnapshotPayload) {}
func (s *testNoOpStrategy) OnBookUpdate(_ context.Context, _ *schema.Event, _ schema.BookUpdatePayload)   {}
func (s *testNoOpStrategy) OnOrderFilled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}
func (s *testNoOpStrategy) OnOrderRejected(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload, _ string) {}
func (s *testNoOpStrategy) OnOrderPartialFill(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}
func (s *testNoOpStrategy) OnOrderCancelled(_ context.Context, _ *schema.Event, _ schema.ExecReportPayload) {}
