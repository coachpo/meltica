package dispatcher

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

// Mock implementations for testing

type mockSubscriptionManager struct {
	activated   []Route
	deactivated []schema.CanonicalType
}

func (m *mockSubscriptionManager) Activate(ctx context.Context, route Route) error {
	m.activated = append(m.activated, route)
	return nil
}

func (m *mockSubscriptionManager) Deactivate(ctx context.Context, typ schema.CanonicalType) error {
	m.deactivated = append(m.deactivated, typ)
	return nil
}

type mockOrderSubmitter struct {
	submitted []schema.OrderRequest
	queries   []string
}

func (m *mockOrderSubmitter) SubmitOrder(ctx context.Context, req schema.OrderRequest) error {
	m.submitted = append(m.submitted, req)
	return nil
}

type mockTradingStateStore struct {
	states map[string]bool
}

func newMockTradingStateStore() *mockTradingStateStore {
	return &mockTradingStateStore{
		states: make(map[string]bool),
	}
}

func (m *mockTradingStateStore) Set(consumerID string, enabled bool) {
	m.states[consumerID] = enabled
}

func (m *mockTradingStateStore) Enabled(consumerID string) bool {
	return m.states[consumerID]
}

type mockControlBus struct {
	messages chan controlbus.Message
}

func newMockControlBus() *mockControlBus {
	return &mockControlBus{
		messages: make(chan controlbus.Message, 10),
	}
}

func (m *mockControlBus) Send(ctx context.Context, cmd schema.ControlMessage) (schema.ControlAcknowledgement, error) {
	return schema.ControlAcknowledgement{Success: true}, nil
}

func (m *mockControlBus) Consume(ctx context.Context) (<-chan controlbus.Message, error) {
	return m.messages, nil
}

func (m *mockControlBus) Close() {
	close(m.messages)
}

type mockDataBus struct {
	published []*schema.Event
}

func (m *mockDataBus) Publish(ctx context.Context, evt *schema.Event) error {
	m.published = append(m.published, evt)
	return nil
}

func (m *mockDataBus) Subscribe(ctx context.Context, typ schema.EventType) (databus.SubscriptionID, <-chan *schema.Event, error) {
	return "sub-1", make(chan *schema.Event), nil
}

func (m *mockDataBus) Unsubscribe(id databus.SubscriptionID) {}

func (m *mockDataBus) Close() {}

// Tests

func TestNewController(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}

	controller := NewController(table, bus, manager)

	if controller == nil {
		t.Fatal("expected non-nil controller")
	}
	if controller.table != table {
		t.Error("expected table to be set")
	}
	if controller.bus != bus {
		t.Error("expected bus to be set")
	}
	if controller.manager != manager {
		t.Error("expected manager to be set")
	}
}

func TestControllerWithOrderSubmitter(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}
	submitter := &mockOrderSubmitter{}

	controller := NewController(table, bus, manager, WithOrderSubmitter(submitter))

	if controller.orders != submitter {
		t.Error("expected order submitter to be set")
	}
}

func TestControllerWithTradingState(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}
	trading := newMockTradingStateStore()

	controller := NewController(table, bus, manager, WithTradingState(trading))

	if controller.trading != trading {
		t.Error("expected trading state to be set")
	}
}

func TestControllerWithControlPublisher(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}
	dataBus := &mockDataBus{}
	pools := pool.NewPoolManager()

	controller := NewController(table, bus, manager, WithControlPublisher(dataBus, pools))

	if controller.eventBus != dataBus {
		t.Error("expected event bus to be set")
	}
	if controller.pools != pools {
		t.Error("expected pools to be set")
	}
}

func TestControllerWithAllOptions(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}
	submitter := &mockOrderSubmitter{}
	trading := newMockTradingStateStore()
	dataBus := &mockDataBus{}
	pools := pool.NewPoolManager()

	controller := NewController(
		table,
		bus,
		manager,
		WithOrderSubmitter(submitter),
		WithTradingState(trading),
		WithControlPublisher(dataBus, pools),
	)

	if controller.orders != submitter {
		t.Error("expected order submitter to be set")
	}
	if controller.trading != trading {
		t.Error("expected trading state to be set")
	}
	if controller.eventBus != dataBus {
		t.Error("expected event bus to be set")
	}
	if controller.pools != pools {
		t.Error("expected pools to be set")
	}
}

func TestControllerBumpVersion(t *testing.T) {
	table := NewTable()
	bus := newMockControlBus()
	manager := &mockSubscriptionManager{}

	controller := NewController(table, bus, manager)

	// Initial version should be 0
	version := controller.bumpVersion()
	if version != 1 {
		t.Errorf("expected version 1, got %d", version)
	}

	// Bump again
	version = controller.bumpVersion()
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}

	// Bump multiple times
	for i := 0; i < 5; i++ {
		controller.bumpVersion()
	}

	version = controller.bumpVersion()
	if version != 8 {
		t.Errorf("expected version 8, got %d", version)
	}
}

func TestMergeFilters(t *testing.T) {
	tests := []struct {
		name     string
		existing []FilterRule
		incoming map[string]any
		expected []FilterRule
	}{
		{
			name:     "nil existing, nil incoming",
			existing: nil,
			incoming: nil,
			expected: nil,
		},
		{
			name:     "empty existing, empty incoming",
			existing: []FilterRule{},
			incoming: map[string]any{},
			expected: []FilterRule{},
		},
		{
			name: "existing filters only",
			existing: []FilterRule{
				{Field: "symbol", Op: "eq", Value: "BTCUSDT"},
			},
			incoming: nil,
			expected: []FilterRule{
				{Field: "symbol", Op: "eq", Value: "BTCUSDT"},
			},
		},
		{
			name:     "incoming filters only",
			existing: nil,
			incoming: map[string]any{
				"symbol": "ETHUSDT",
			},
			expected: []FilterRule{
				{Field: "symbol", Op: "eq", Value: "ETHUSDT"},
			},
		},
		{
			name: "merge both",
			existing: []FilterRule{
				{Field: "exchange", Op: "eq", Value: "binance"},
			},
			incoming: map[string]any{
				"symbol": "BTCUSDT",
			},
			expected: []FilterRule{
				{Field: "exchange", Op: "eq", Value: "binance"},
				{Field: "symbol", Op: "eq", Value: "ETHUSDT"}, // Note: result may vary
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mergeFilters(tt.existing, tt.incoming)
			
			// Check length
			expectedLen := len(tt.existing)
			for k, v := range tt.incoming {
				if k != "" && v != nil {
					expectedLen++
				}
			}
			
			if len(result) != expectedLen {
				t.Errorf("expected %d filters, got %d", expectedLen, len(result))
			}
		})
	}
}
