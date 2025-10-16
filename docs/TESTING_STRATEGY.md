# Testing Strategy for Complex Packages

This document outlines strategies for making the currently untestable packages in Meltica testable.

## Overview

Three package categories require testing infrastructure:
1. **internal/consumer** - Event consumers with bus dependencies
2. **internal/telemetry** - OpenTelemetry instrumentation
3. **internal/adapters** - Exchange-specific integrations

---

## 1. internal/consumer Package

### Current Challenges

- **Hard dependencies on buses**: Lambda requires `databus.Bus` and `controlbus.Bus`
- **Goroutine-based consumption**: Tests need to handle async event processing
- **Stateful operations**: Atomic counters, subscription management

### Testing Strategy

#### A. Interface-Based Dependency Injection (Recommended)

The consumer already uses interfaces for buses, which is excellent! Create mock implementations:

```go
// internal/consumer/testing.go (or testutil package)

package consumer

import (
	"context"
	"sync"
	
	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/bus/controlbus"
)

// MockDataBus provides a testable data bus implementation
type MockDataBus struct {
	mu            sync.Mutex
	subscriptions map[string]*MockSubscription
	nextID        int
}

type MockSubscription struct {
	ID      string
	Type    schema.EventType
	Channel chan *schema.Event
	Active  bool
}

func NewMockDataBus() *MockDataBus {
	return &MockDataBus{
		subscriptions: make(map[string]*MockSubscription),
	}
}

func (m *MockDataBus) Subscribe(ctx context.Context, typ schema.EventType) (string, <-chan *schema.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	id := fmt.Sprintf("sub-%d", m.nextID)
	m.nextID++
	
	ch := make(chan *schema.Event, 10)
	sub := &MockSubscription{
		ID:      id,
		Type:    typ,
		Channel: ch,
		Active:  true,
	}
	m.subscriptions[id] = sub
	
	return id, ch, nil
}

func (m *MockDataBus) Unsubscribe(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if sub, ok := m.subscriptions[id]; ok {
		sub.Active = false
		close(sub.Channel)
		delete(m.subscriptions, id)
	}
	return nil
}

func (m *MockDataBus) Publish(ctx context.Context, event *schema.Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, sub := range m.subscriptions {
		if sub.Active && sub.Type == event.Type {
			select {
			case sub.Channel <- event:
			case <-ctx.Done():
				return ctx.Err()
			default:
				// Non-blocking
			}
		}
	}
	return nil
}

// PublishSync publishes event and waits for consumption (test helper)
func (m *MockDataBus) PublishSync(event *schema.Event, timeout time.Duration) error {
	m.mu.Lock()
	subs := make([]*MockSubscription, 0, len(m.subscriptions))
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
		}
	}
	return nil
}

// MockControlBus provides testable control bus
type MockControlBus struct {
	mu       sync.Mutex
	messages []schema.ControlMessage
	acks     chan schema.ControlAcknowledgement
}

func NewMockControlBus() *MockControlBus {
	return &MockControlBus{
		messages: make([]schema.ControlMessage, 0),
		acks:     make(chan schema.ControlAcknowledgement, 10),
	}
}

func (m *MockControlBus) Send(ctx context.Context, msg schema.ControlMessage) (schema.ControlAcknowledgement, error) {
	m.mu.Lock()
	m.messages = append(m.messages, msg)
	m.mu.Unlock()
	
	// Simulate immediate acknowledgement
	ack := schema.ControlAcknowledgement{
		MessageID:  msg.MessageID,
		ConsumerID: msg.ConsumerID,
		Success:    true,
		Timestamp:  time.Now(),
	}
	
	select {
	case m.acks <- ack:
	default:
	}
	
	return ack, nil
}

func (m *MockControlBus) GetMessages() []schema.ControlMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	result := make([]schema.ControlMessage, len(m.messages))
	copy(result, m.messages)
	return result
}
```

#### B. Example Test Cases

```go
// internal/consumer/lambda_test.go

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

func TestLambdaBasicSubscription(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	
	lambda := NewLambda("test-lambda", mockBus, pm, logger)
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	errCh, err := lambda.Start(ctx, []schema.EventType{"TRADE"})
	if err != nil {
		t.Fatalf("failed to start lambda: %v", err)
	}
	
	// Give it time to subscribe
	time.Sleep(50 * time.Millisecond)
	
	// Publish a test event
	testEvent := &schema.Event{
		EventID:  "test-1",
		Provider: "test",
		Symbol:   "BTC-USD",
		Type:     "TRADE",
		Payload:  schema.TradePayload{Price: "50000", Quantity: "1.0"},
	}
	
	if err := mockBus.PublishSync(testEvent, time.Second); err != nil {
		t.Fatalf("failed to publish: %v", err)
	}
	
	// Check for errors
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(100 * time.Millisecond):
		// Success - no errors
	}
}

func TestLambdaControlMessage(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)
	
	lambda := NewLambda("test-lambda", mockDataBus, pm, logger)
	lambda.AttachControlBus(mockControlBus, "test-consumer")
	
	ctx := context.Background()
	
	// Send a subscribe control message
	payload := schema.SubscribePayload{
		Type:    "TICKER",
		Filters: map[string]any{"symbol": "ETH-USD"},
	}
	
	ack, err := lambda.SendControlMessage(ctx, schema.ControlTypeSubscribe, payload)
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
	
	if messages[0].Type != schema.ControlTypeSubscribe {
		t.Errorf("expected subscribe type, got %s", messages[0].Type)
	}
}
```

#### C. Test Utilities to Create

```go
// internal/testutil/events.go - Helper functions for creating test events

package testutil

import (
	"github.com/coachpo/meltica/internal/schema"
	"time"
)

func NewTradeEvent(symbol, price, qty string) *schema.Event {
	return &schema.Event{
		EventID:  fmt.Sprintf("trade-%d", time.Now().UnixNano()),
		Provider: "test",
		Symbol:   symbol,
		Type:     schema.EventTypeTrade,
		EmitTS:   time.Now(),
		Payload: schema.TradePayload{
			Price:    price,
			Quantity: qty,
			Side:     schema.TradeSideBuy,
		},
	}
}

func NewTickerEvent(symbol, bid, ask string) *schema.Event {
	return &schema.Event{
		EventID:  fmt.Sprintf("ticker-%d", time.Now().UnixNano()),
		Provider: "test",
		Symbol:   symbol,
		Type:     schema.EventTypeTicker,
		EmitTS:   time.Now(),
		Payload: schema.TickerPayload{
			BidPrice: bid,
			AskPrice: ask,
		},
	}
}
```

---

## 2. internal/telemetry Package

### Current Challenges

- **External dependencies**: OpenTelemetry SDK initialization
- **Side effects**: Global OTel state configuration
- **Network calls**: OTLP exporters connect to real endpoints

### Testing Strategy

#### A. Disable in Tests (Simplest)

```go
// internal/telemetry/telemetry_test.go

package telemetry

import (
	"context"
	"testing"
)

func TestProviderDisabled(t *testing.T) {
	cfg := Config{
		Enabled: false,
	}
	
	provider, err := NewProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("failed to create disabled provider: %v", err)
	}
	
	if provider.tracerProvider != nil {
		t.Error("expected nil tracer provider when disabled")
	}
	if provider.meterProvider != nil {
		t.Error("expected nil meter provider when disabled")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	
	if cfg.OTLPEndpoint == "" {
		t.Error("expected non-empty OTLP endpoint")
	}
	if cfg.ServiceName == "" {
		t.Error("expected non-empty service name")
	}
	if cfg.SampleRate <= 0 || cfg.SampleRate > 1 {
		t.Errorf("invalid sample rate: %f", cfg.SampleRate)
	}
}
```

#### B. In-Memory Exporters (Better)

```go
// internal/telemetry/testing.go

package telemetry

import (
	"context"
	
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
	"bytes"
)

// NewTestProvider creates a telemetry provider with in-memory exporters for testing
func NewTestProvider(ctx context.Context) (*Provider, *bytes.Buffer, error) {
	var buf bytes.Buffer
	
	exporter, err := stdouttrace.New(
		stdouttrace.WithWriter(&buf),
		stdouttrace.WithPrettyPrint(),
	)
	if err != nil {
		return nil, nil, err
	}
	
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithSampler(trace.AlwaysSample()),
	)
	
	provider := &Provider{
		tracerProvider: tp,
		config: Config{
			Enabled:     true,
			ServiceName: "test-service",
		},
	}
	
	return provider, &buf, nil
}

// Test usage:
func TestTracerCreation(t *testing.T) {
	ctx := context.Background()
	provider, buf, err := NewTestProvider(ctx)
	if err != nil {
		t.Fatalf("failed to create test provider: %v", err)
	}
	defer provider.Shutdown(ctx)
	
	tracer := provider.Tracer("test")
	_, span := tracer.Start(ctx, "test-span")
	span.End()
	
	// Verify trace was recorded
	if buf.Len() == 0 {
		t.Error("expected trace output")
	}
}
```

#### C. Focus on Configuration Testing

Most of telemetry testing should focus on configuration logic rather than OTel internals:

```go
func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		expected Config
	}{
		{
			name: "default values",
			env:  map[string]string{},
			expected: Config{
				Enabled:      true,
				OTLPEndpoint: "localhost:4318",
				ServiceName:  "meltica",
			},
		},
		{
			name: "custom endpoint",
			env: map[string]string{
				"OTEL_EXPORTER_OTLP_ENDPOINT": "custom:4318",
			},
			expected: Config{
				Enabled:      true,
				OTLPEndpoint: "custom:4318",
			},
		},
		{
			name: "disabled",
			env: map[string]string{
				"OTEL_ENABLED": "false",
			},
			expected: Config{
				Enabled: false,
			},
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.env {
				os.Setenv(k, v)
			}
			defer func() {
				for k := range tt.env {
					os.Unsetenv(k)
				}
			}()
			
			cfg := DefaultConfig()
			
			if cfg.Enabled != tt.expected.Enabled {
				t.Errorf("Enabled: got %v, want %v", cfg.Enabled, tt.expected.Enabled)
			}
			// ... test other fields
		})
	}
}
```

---

## 3. internal/adapters Package

### Current Challenges

- **WebSocket connections**: Real network I/O with Binance
- **REST API calls**: External HTTP dependencies
- **Complex state machines**: Order books, connection management
- **Time-sensitive logic**: Reconnection, heartbeats

### Testing Strategy

#### A. Layered Testing Approach

1. **Unit Tests**: Parser functions (already possible)
2. **Integration Tests**: Mock network layer
3. **End-to-End Tests**: Recorded fixtures

#### B. Mock WebSocket Client

```go
// internal/adapters/binance/testing.go

package binance

import (
	"context"
	"sync"
	"time"
)

// MockWSClient simulates WebSocket behavior for testing
type MockWSClient struct {
	mu              sync.Mutex
	connected       bool
	subscriptions   map[string]bool
	messageHandlers []func([]byte)
	sentMessages    [][]byte
}

func NewMockWSClient() *MockWSClient {
	return &MockWSClient{
		subscriptions: make(map[string]bool),
	}
}

func (m *MockWSClient) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.connected = true
	return nil
}

func (m *MockWSClient) Subscribe(topics []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for _, topic := range topics {
		m.subscriptions[topic] = true
	}
	return nil
}

func (m *MockWSClient) Send(data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.sentMessages = append(m.sentMessages, data)
	return nil
}

func (m *MockWSClient) OnMessage(handler func([]byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.messageHandlers = append(m.messageHandlers, handler)
}

// SimulateMessage injects a message as if it came from Binance
func (m *MockWSClient) SimulateMessage(data []byte) {
	m.mu.Lock()
	handlers := make([]func([]byte), len(m.messageHandlers))
	copy(handlers, m.messageHandlers)
	m.mu.Unlock()
	
	for _, handler := range handlers {
		handler(data)
	}
}

func (m *MockWSClient) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.connected = false
	return nil
}

// GetSubscriptions returns current subscriptions (test helper)
func (m *MockWSClient) GetSubscriptions() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	subs := make([]string, 0, len(m.subscriptions))
	for topic := range m.subscriptions {
		subs = append(subs, topic)
	}
	return subs
}
```

#### C. Fixture-Based Testing

```go
// internal/adapters/binance/parser_test.go

package binance

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// Test with real recorded messages
func TestParseTradeMessage(t *testing.T) {
	// Load fixture
	fixture := loadFixture(t, "testdata/trade_message.json")
	
	event, err := parseTradeStream(fixture)
	if err != nil {
		t.Fatalf("failed to parse trade: %v", err)
	}
	
	if event.Type != "TRADE" {
		t.Errorf("expected TRADE type, got %s", event.Type)
	}
	
	payload, ok := event.Payload.(TradePayload)
	if !ok {
		t.Fatal("expected TradePayload")
	}
	
	if payload.Price == "" {
		t.Error("expected non-empty price")
	}
}

func loadFixture(t *testing.T, path string) []byte {
	t.Helper()
	
	data, err := os.ReadFile(filepath.Join("testdata", path))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", path, err)
	}
	return data
}

// Create testdata directory with real Binance messages
// testdata/trade_message.json:
// {
//   "e": "trade",
//   "E": 1672531200000,
//   "s": "BTCUSDT",
//   "t": 12345,
//   "p": "50000.00",
//   "q": "0.001",
//   "T": 1672531200000,
//   "m": false
// }
```

#### D. Provider Testing Pattern

```go
// internal/adapters/binance/provider_test.go

package binance

import (
	"context"
	"testing"
	"time"
	
	"github.com/coachpo/meltica/internal/pool"
)

func TestProviderSubscribe(t *testing.T) {
	mockWS := NewMockWSClient()
	mockREST := NewMockRESTClient()
	pm := pool.NewPoolManager()
	
	provider := NewProvider("test", mockWS, mockREST, ProviderOptions{
		Topics: []string{"btcusdt@trade"},
		Pools:  pm,
	})
	
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	
	if err := provider.Start(ctx); err != nil {
		t.Fatalf("failed to start provider: %v", err)
	}
	
	// Subscribe to a canonical route
	err := provider.Subscribe(ctx, dispatcher.Route{
		Provider: "binance",
		Symbol:   "BTC-USD",
		Type:     "TRADE",
	})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	
	// Verify WS subscription
	subs := mockWS.GetSubscriptions()
	if len(subs) == 0 {
		t.Error("expected WebSocket subscription")
	}
	
	// Simulate incoming message
	mockWS.SimulateMessage([]byte(`{
		"e":"trade",
		"s":"BTCUSDT",
		"p":"50000.00",
		"q":"1.0",
		"T":1672531200000
	}`))
	
	// Read from events channel
	select {
	case event := <-provider.Events():
		if event.Symbol != "BTC-USD" {
			t.Errorf("expected BTC-USD, got %s", event.Symbol)
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}
```

---

## 4. Recommended Testing Priorities

### High Priority (Most Impact)

1. **Consumer package mocks** - Enables testing of all consumer logic
2. **Parser functions in adapters** - Pure functions, easy wins
3. **Telemetry config/disabled mode** - Configuration logic

### Medium Priority

4. **Mock WebSocket/REST clients** - Enables adapter integration tests
5. **Fixture-based adapter tests** - Test with real message formats

### Low Priority (Diminishing Returns)

6. **Full telemetry provider tests** - Complex, low business value
7. **Live adapter tests** - Brittle, slow, requires network

---

## 5. Project Structure Changes

Consider creating a testutil package:

```
internal/
  testutil/
    bus.go          # Mock bus implementations
    events.go       # Test event builders
    adapters.go     # Mock adapter clients
    fixtures.go     # Fixture loading utilities
```

---

## 6. Quick Wins to Start

1. **Add parser tests** for binance package (pure functions)
2. **Create mock buses** in consumer package
3. **Test telemetry config** functions
4. **Add fixture directory** with real Binance messages

This will immediately boost coverage from 25% toward 50%+.

---

## Summary

**Key Principles:**
- ✅ **Dependency injection**: Use interfaces (already done for buses!)
- ✅ **Mock external I/O**: WebSocket, REST, OTLP exporters
- ✅ **Test in layers**: Unit → Integration → E2E
- ✅ **Use fixtures**: Record real messages for parser tests
- ✅ **Focus on logic**: Test business logic, not framework code

**Expected Coverage After Implementation:**
- Consumer: 60-70% (with mock buses)
- Telemetry: 40-50% (config + disabled mode)
- Adapters: 40-60% (parsers + mock clients)
- **Total: 50-60% overall**

To reach 70%, you'd need full integration tests with recorded sessions, which has diminishing returns. 50-60% with good unit + integration coverage is very healthy for a real-time trading system.
