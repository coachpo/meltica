package consumer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/schema"
)

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
	mu       sync.Mutex
	messages []schema.ControlMessage
	acks     map[string]schema.ControlAcknowledgement
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
