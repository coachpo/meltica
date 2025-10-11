package wsrouting_test

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/lib/ws-routing/connection"
)

type mockTransport struct {
	mu     sync.Mutex
	queue  chan connection.Event
	sent   []connection.Event
	closed bool
}

func newMockTransport(buffer int) *mockTransport {
	return &mockTransport{queue: make(chan connection.Event, buffer), sent: make([]connection.Event, 0)}
}

func (m *mockTransport) Receive() <-chan connection.Event {
	return m.queue
}

func (m *mockTransport) Send(event connection.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("transport closed")
	}
	m.sent = append(m.sent, event)
	return nil
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.closed = true
	close(m.queue)
	return nil
}

func TestMockTransportImplementsAbstractTransport(t *testing.T) {
	var _ connection.AbstractTransport = (*mockTransport)(nil)
}

func TestMockTransportSendRecordsEvents(t *testing.T) {
	transport := newMockTransport(2)
	events := []connection.Event{
		{Symbol: "BTC-USD", Type: "trade", Timestamp: time.Now()},
		{Symbol: "ETH-USD", Type: "trade", Timestamp: time.Now()},
	}
	for _, event := range events {
		if err := transport.Send(event); err != nil {
			t.Fatalf("send event: %v", err)
		}
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	if len(transport.sent) != len(events) {
		t.Fatalf("expected %d events recorded, got %d", len(events), len(transport.sent))
	}
	for i, event := range events {
		if transport.sent[i].Symbol != event.Symbol {
			t.Fatalf("expected symbol %s at index %d, got %s", event.Symbol, i, transport.sent[i].Symbol)
		}
	}
}

func TestMockTransportClosePreventsSend(t *testing.T) {
	transport := newMockTransport(1)
	if err := transport.Close(); err != nil {
		t.Fatalf("close transport: %v", err)
	}
	if err := transport.Send(connection.Event{}); err == nil {
		t.Fatalf("expected error when sending after close")
	}
	if _, ok := <-transport.Receive(); ok {
		t.Fatalf("expected receive channel to be closed")
	}
}
