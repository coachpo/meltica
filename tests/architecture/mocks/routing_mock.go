package mocks

import (
	"context"
	"sync"

	"github.com/coachpo/meltica/core/layers"
	sharedrouting "github.com/coachpo/meltica/exchanges/shared/routing"
)

// Ensure interface compliance.
var (
	_ layers.Routing     = (*MockRouting)(nil)
	_ layers.RESTRouting = (*MockRESTRouting)(nil)
)

// MockRouting provides a configurable implementation of layers.Routing.
type MockRouting struct {
	mu sync.Mutex

	SubscribeFn    func(context.Context, layers.SubscriptionRequest) error
	UnsubscribeFn  func(context.Context, layers.SubscriptionRequest) error
	ParseMessageFn func([]byte) (layers.NormalizedMessage, error)

	handler         layers.MessageHandler
	subscriptions   []layers.SubscriptionRequest
	unsubscriptions []layers.SubscriptionRequest
}

// NewMockRouting constructs a routing mock with no-op behaviour.
func NewMockRouting() *MockRouting {
	return &MockRouting{
		SubscribeFn:   func(context.Context, layers.SubscriptionRequest) error { return nil },
		UnsubscribeFn: func(context.Context, layers.SubscriptionRequest) error { return nil },
		ParseMessageFn: func(data []byte) (layers.NormalizedMessage, error) {
			return layers.NormalizedMessage{Data: data}, nil
		},
	}
}

// Subscribe records the request and executes the configured behaviour.
func (m *MockRouting) Subscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	m.mu.Lock()
	m.subscriptions = append(m.subscriptions, req)
	fn := m.SubscribeFn
	m.mu.Unlock()
	return fn(ctx, req)
}

// Unsubscribe records the request and executes the configured behaviour.
func (m *MockRouting) Unsubscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	m.mu.Lock()
	m.unsubscriptions = append(m.unsubscriptions, req)
	fn := m.UnsubscribeFn
	m.mu.Unlock()
	return fn(ctx, req)
}

// OnMessage stores the handler for later invocation.
func (m *MockRouting) OnMessage(handler layers.MessageHandler) {
	m.mu.Lock()
	m.handler = handler
	m.mu.Unlock()
}

// ParseMessage delegates to the configured behaviour.
func (m *MockRouting) ParseMessage(raw []byte) (layers.NormalizedMessage, error) {
	m.mu.Lock()
	fn := m.ParseMessageFn
	m.mu.Unlock()
	return fn(raw)
}

// Emit triggers the registered handler with the supplied message.
func (m *MockRouting) Emit(msg layers.NormalizedMessage) {
	m.mu.Lock()
	h := m.handler
	m.mu.Unlock()
	if h != nil {
		h(msg)
	}
}

// Subscriptions returns a snapshot of recorded subscribe requests.
func (m *MockRouting) Subscriptions() []layers.SubscriptionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]layers.SubscriptionRequest, len(m.subscriptions))
	copy(copied, m.subscriptions)
	return copied
}

// Unsubscriptions returns a snapshot of recorded unsubscribe requests.
func (m *MockRouting) Unsubscriptions() []layers.SubscriptionRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := make([]layers.SubscriptionRequest, len(m.unsubscriptions))
	copy(copied, m.unsubscriptions)
	return copied
}

// MockRESTRouting decorates MockRouting with REST helpers and legacy dispatcher access.
type MockRESTRouting struct {
	*MockRouting
	BuildRequestFn         func(context.Context, layers.APIRequest) (*layers.HTTPRequest, error)
	ParseResponseFn        func(*layers.HTTPResponse) (layers.NormalizedResponse, error)
	LegacyRESTDispatcherFn func() sharedrouting.RESTDispatcher
}

// NewMockRESTRouting constructs a REST routing mock.
func NewMockRESTRouting() *MockRESTRouting {
	return &MockRESTRouting{
		MockRouting: NewMockRouting(),
		BuildRequestFn: func(context.Context, layers.APIRequest) (*layers.HTTPRequest, error) {
			return &layers.HTTPRequest{}, nil
		},
		ParseResponseFn: func(*layers.HTTPResponse) (layers.NormalizedResponse, error) {
			return layers.NormalizedResponse{}, nil
		},
		LegacyRESTDispatcherFn: func() sharedrouting.RESTDispatcher { return nil },
	}
}

// BuildRequest delegates to the configured function.
func (m *MockRESTRouting) BuildRequest(ctx context.Context, req layers.APIRequest) (*layers.HTTPRequest, error) {
	return m.BuildRequestFn(ctx, req)
}

// ParseResponse delegates to the configured function.
func (m *MockRESTRouting) ParseResponse(resp *layers.HTTPResponse) (layers.NormalizedResponse, error) {
	return m.ParseResponseFn(resp)
}

// LegacyRESTDispatcher returns the configured dispatcher.
func (m *MockRESTRouting) LegacyRESTDispatcher() sharedrouting.RESTDispatcher {
	return m.LegacyRESTDispatcherFn()
}
