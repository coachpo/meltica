package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
	coretransport "github.com/coachpo/meltica/core/transport"
)

// Ensure interfaces are satisfied at compile time.
var (
	_ layers.Connection   = (*MockConnection)(nil)
	_ layers.WSConnection = (*MockWSConnection)(nil)
)

// MockConnection provides a configurable implementation of layers.Connection for tests.
type MockConnection struct {
	mu                 sync.Mutex
	ConnectFn          func(context.Context) error
	CloseFn            func() error
	IsConnectedFn      func() bool
	SetReadDeadlineFn  func(time.Time) error
	SetWriteDeadlineFn func(time.Time) error

	connectCalls int
	closeCalls   int
}

// NewMockConnection constructs a mock with no-op behaviour by default.
func NewMockConnection() *MockConnection {
	return &MockConnection{
		ConnectFn: func(context.Context) error { return nil },
		CloseFn:   func() error { return nil },
		IsConnectedFn: func() bool {
			return true
		},
		SetReadDeadlineFn:  func(time.Time) error { return nil },
		SetWriteDeadlineFn: func(time.Time) error { return nil },
	}
}

// Connect increments the call count and executes the configured behaviour.
func (m *MockConnection) Connect(ctx context.Context) error {
	m.mu.Lock()
	m.connectCalls++
	fn := m.ConnectFn
	m.mu.Unlock()
	return fn(ctx)
}

// Close increments the call count and executes the configured behaviour.
func (m *MockConnection) Close() error {
	m.mu.Lock()
	m.closeCalls++
	fn := m.CloseFn
	m.mu.Unlock()
	return fn()
}

// IsConnected delegates to the configured function.
func (m *MockConnection) IsConnected() bool {
	m.mu.Lock()
	fn := m.IsConnectedFn
	m.mu.Unlock()
	return fn()
}

// SetReadDeadline delegates to the configured function.
func (m *MockConnection) SetReadDeadline(t time.Time) error {
	m.mu.Lock()
	fn := m.SetReadDeadlineFn
	m.mu.Unlock()
	return fn(t)
}

// SetWriteDeadline delegates to the configured function.
func (m *MockConnection) SetWriteDeadline(t time.Time) error {
	m.mu.Lock()
	fn := m.SetWriteDeadlineFn
	m.mu.Unlock()
	return fn(t)
}

// ConnectCalls returns the number of times Connect was invoked.
func (m *MockConnection) ConnectCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connectCalls
}

// CloseCalls returns the number of times Close was invoked.
func (m *MockConnection) CloseCalls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closeCalls
}

// MockWSConnection decorates MockConnection with websocket-specific behaviour.
type MockWSConnection struct {
	*MockConnection
	ReadMessageFn  func() (int, []byte, error)
	WriteMessageFn func(int, []byte) error
	PingFn         func(context.Context) error
	OnPongFn       func(func())

	StreamClient coretransport.StreamClient
}

// NewMockWSConnection constructs a websocket-capable mock connection.
func NewMockWSConnection() *MockWSConnection {
	return &MockWSConnection{
		MockConnection: NewMockConnection(),
		ReadMessageFn:  func() (int, []byte, error) { return 0, nil, nil },
		WriteMessageFn: func(int, []byte) error { return nil },
		PingFn:         func(context.Context) error { return nil },
		OnPongFn:       func(func()) {},
	}
}

// ReadMessage delegates to the configured function.
func (m *MockWSConnection) ReadMessage() (int, []byte, error) {
	return m.ReadMessageFn()
}

// WriteMessage delegates to the configured function.
func (m *MockWSConnection) WriteMessage(messageType int, data []byte) error {
	return m.WriteMessageFn(messageType, data)
}

// Ping delegates to the configured function.
func (m *MockWSConnection) Ping(ctx context.Context) error {
	return m.PingFn(ctx)
}

// OnPong delegates to the configured function.
func (m *MockWSConnection) OnPong(handler func()) {
	m.OnPongFn(handler)
}

// LegacyStreamClient exposes the underlying stream client used by the adapter.
func (m *MockWSConnection) LegacyStreamClient() coretransport.StreamClient {
	return m.StreamClient
}
