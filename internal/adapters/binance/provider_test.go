package binance

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/dispatcher"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

type mockWSClient struct {
	events chan *schema.Event
	errs   chan error
}

func (m *mockWSClient) Stream(_ context.Context, _ []string) (<-chan *schema.Event, <-chan error) {
	return m.events, m.errs
}

type mockRESTClient struct {
	events chan *schema.Event
	errs   chan error
}

func (m *mockRESTClient) Poll(_ context.Context, _ []RESTPoller) (<-chan *schema.Event, <-chan error) {
	return m.events, m.errs
}

func TestNewProvider(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	if provider == nil {
		t.Fatal("expected non-nil provider")
	}
	if provider.name != "test" {
		t.Errorf("expected name 'test', got %s", provider.name)
	}
}

func TestNewProvider_DefaultName(t *testing.T) {
	provider := NewProvider("", nil, nil, ProviderOptions{})
	
	if provider.name != "binance" {
		t.Errorf("expected default name 'binance', got %s", provider.name)
	}
}

func TestProvider_Start_NilContext(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	err := provider.Start(nil)
	if err == nil {
		t.Error("expected error for nil context")
	}
	if !strings.Contains(err.Error(), "context") {
		t.Errorf("expected 'context' in error, got: %v", err)
	}
}

func TestProvider_Start_AlreadyStarted(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx := context.Background()
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("first start failed: %v", err)
	}
	
	err = provider.Start(ctx)
	if err == nil {
		t.Error("expected error for already started provider")
	}
	if !strings.Contains(err.Error(), "already started") {
		t.Errorf("expected 'already started' in error, got: %v", err)
	}
}

func TestProvider_Start_Success(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	cancel()
	time.Sleep(50 * time.Millisecond)
}

func TestProvider_Events(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	events := provider.Events()
	if events == nil {
		t.Error("expected non-nil events channel")
	}
}

func TestProvider_Errors(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	errs := provider.Errors()
	if errs == nil {
		t.Error("expected non-nil errors channel")
	}
}

func TestProvider_SubmitOrder_NotStarted(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	order := schema.OrderRequest{
		Symbol:   "BTC-USDT",
		Side:     schema.TradeSideBuy,
		Quantity: "1.0",
	}
	
	err := provider.SubmitOrder(context.Background(), order)
	if err == nil {
		t.Error("expected error for not started provider")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("expected 'not started' in error, got: %v", err)
	}
}

func TestProvider_SubmitOrder_Success(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	
	provider := NewProvider("test", nil, nil, ProviderOptions{Pools: pools})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	order := schema.OrderRequest{
		ClientOrderID: "test-order-1",
		Symbol:        "BTC-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Quantity:      "1.0",
	}
	
	err = provider.SubmitOrder(ctx, order)
	if err != nil {
		t.Errorf("submit order failed: %v", err)
	}
	
	select {
	case evt := <-provider.Events():
		if evt.Type != schema.EventTypeExecReport {
			t.Errorf("expected ExecReport event, got %v", evt.Type)
		}
		payload, ok := evt.Payload.(schema.ExecReportPayload)
		if !ok {
			t.Fatalf("expected ExecReportPayload, got %T", evt.Payload)
		}
		if payload.ClientOrderID != "test-order-1" {
			t.Errorf("expected client order ID test-order-1, got %s", payload.ClientOrderID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for exec report event")
	}
}

func TestProvider_SubmitOrder_WithDefaultProvider(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	
	provider := NewProvider("test", nil, nil, ProviderOptions{Pools: pools})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	order := schema.OrderRequest{
		ClientOrderID: "test-order-2",
		Symbol:        "BTC-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeMarket,
		Quantity:      "0.5",
	}
	
	err = provider.SubmitOrder(ctx, order)
	if err != nil {
		t.Errorf("submit order failed: %v", err)
	}
	
	select {
	case evt := <-provider.Events():
		if evt.Provider != "test" {
			t.Errorf("expected provider name to be set to 'test', got %s", evt.Provider)
		}
	case <-time.After(200 * time.Millisecond):
		t.Error("timeout waiting for exec report event")
	}
}

func TestProvider_SubscribeRoute_NoType(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx := context.Background()
	provider.Start(ctx)
	
	route := dispatcher.Route{}
	err := provider.SubscribeRoute(route)
	if err == nil {
		t.Error("expected error for route with no type")
	}
	if !strings.Contains(err.Error(), "type required") {
		t.Errorf("expected 'type required' in error, got: %v", err)
	}
}

func TestProvider_SubscribeRoute_NotStarted(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	route := dispatcher.Route{
		Type: schema.CanonicalType(schema.EventTypeTrade),
	}
	
	err := provider.SubscribeRoute(route)
	if err == nil {
		t.Error("expected error for not started provider")
	}
	if !strings.Contains(err.Error(), "not started") {
		t.Errorf("expected 'not started' in error, got: %v", err)
	}
}

func TestProvider_SubscribeRoute_Success(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	route := dispatcher.Route{
		Type:     schema.CanonicalType(schema.EventTypeTrade),
		WSTopics: []string{"btcusdt@aggTrade"},
	}
	
	err = provider.SubscribeRoute(route)
	if err != nil {
		t.Errorf("subscribe route failed: %v", err)
	}
	
	err = provider.SubscribeRoute(route)
	if err != nil {
		t.Errorf("re-subscribe should not error: %v", err)
	}
}

func TestProvider_UnsubscribeRoute_NoType(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	err := provider.UnsubscribeRoute("")
	if err == nil {
		t.Error("expected error for empty route type")
	}
	if !strings.Contains(err.Error(), "type required") {
		t.Errorf("expected 'type required' in error, got: %v", err)
	}
}

func TestProvider_UnsubscribeRoute_NotSubscribed(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx := context.Background()
	provider.Start(ctx)
	
	err := provider.UnsubscribeRoute(schema.CanonicalType(schema.EventTypeTrade))
	if err != nil {
		t.Errorf("unsubscribe non-existent route should not error: %v", err)
	}
}

func TestProvider_UnsubscribeRoute_Success(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	route := dispatcher.Route{
		Type:     schema.CanonicalType(schema.EventTypeTrade),
		WSTopics: []string{"btcusdt@aggTrade"},
	}
	
	err = provider.SubscribeRoute(route)
	if err != nil {
		t.Fatalf("subscribe route failed: %v", err)
	}
	
	err = provider.UnsubscribeRoute(schema.CanonicalType(schema.EventTypeTrade))
	if err != nil {
		t.Errorf("unsubscribe route failed: %v", err)
	}
}

func TestProvider_EmitError(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	
	provider := NewProvider("test", nil, nil, ProviderOptions{Pools: pools})
	
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	err := provider.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	
	testErr := errors.New("test error")
	provider.emitError(testErr)
	
	select {
	case receivedErr := <-provider.Errors():
		if receivedErr.Error() != "test error" {
			t.Errorf("expected 'test error', got %v", receivedErr)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for error")
	}
}

func TestProvider_PrepareOrderBook_Trade(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	
	provider := NewProvider("test", nil, nil, ProviderOptions{Pools: pools})
	
	evt := &schema.Event{
		Type:   schema.EventTypeTrade,
		Symbol: "BTC-USDT",
		Payload: schema.TradePayload{
			TradeID: "123",
			Price:   "50000.00",
		},
	}
	
	extra, emitCurrent, err := provider.prepareOrderBook(evt)
	if err != nil {
		t.Errorf("prepare order book failed: %v", err)
	}
	if !emitCurrent {
		t.Error("expected emitCurrent to be true for trade event")
	}
	if len(extra) != 0 {
		t.Error("expected no extra events for trade")
	}
}

func TestProvider_PrepareOrderBook_NilEvent(t *testing.T) {
	provider := NewProvider("test", nil, nil, ProviderOptions{})
	
	extra, emitCurrent, err := provider.prepareOrderBook(nil)
	if err != nil {
		t.Errorf("expected no error for nil event, got %v", err)
	}
	if emitCurrent {
		t.Error("expected emitCurrent to be false for nil event")
	}
	if len(extra) != 0 {
		t.Error("expected no extra events for nil event")
	}
}

func TestOrderKey(t *testing.T) {
	tests := []struct {
		provider      string
		clientOrderID string
		expected      string
	}{
		{"binance", "order123", "binance:order123"},
		{"BINANCE", "order123", "binance:order123"},
		{"  binance  ", "  order123  ", "binance:order123"},
	}
	
	for _, tt := range tests {
		result := orderKey(tt.provider, tt.clientOrderID)
		if result != tt.expected {
			t.Errorf("expected %s, got %s", tt.expected, result)
		}
	}
}

func TestValueOrDefault(t *testing.T) {
	tests := []struct {
		name     string
		value    *string
		expected string
	}{
		{"nil", nil, ""},
		{"empty", stringPtr(""), ""},
		{"value", stringPtr("test"), "test"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := valueOrDefault(tt.value)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestPollersFromRoute(t *testing.T) {
	restFns := []dispatcher.RestFn{
		{
			Name:     "orderbook",
			Endpoint: "/api/v3/depth",
			Interval: 5 * time.Second,
			Parser:   "orderbook",
		},
	}
	
	pollers := pollersFromRoute(restFns)
	
	if len(pollers) != 1 {
		t.Fatalf("expected 1 poller, got %d", len(pollers))
	}
	
	if pollers[0].Name != "orderbook" {
		t.Errorf("expected name 'orderbook', got %s", pollers[0].Name)
	}
	if pollers[0].Endpoint != "/api/v3/depth" {
		t.Errorf("expected endpoint '/api/v3/depth', got %s", pollers[0].Endpoint)
	}
	if pollers[0].Interval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", pollers[0].Interval)
	}
}

func TestPollersFromRoute_Empty(t *testing.T) {
	pollers := pollersFromRoute(nil)
	if pollers != nil {
		t.Error("expected nil pollers for nil input")
	}
	
	pollers = pollersFromRoute([]dispatcher.RestFn{})
	if pollers != nil {
		t.Error("expected nil pollers for empty input")
	}
}

func TestCoerceBookSnapshot(t *testing.T) {
	payload := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000", Quantity: "1.0"}},
		Asks: []schema.PriceLevel{{Price: "50001", Quantity: "0.5"}},
	}
	
	result, ok := coerceBookSnapshot(payload)
	if !ok {
		t.Error("expected coercion to succeed for value type")
	}
	if len(result.Bids) != 1 {
		t.Error("expected bids to be preserved")
	}
	
	result, ok = coerceBookSnapshot(&payload)
	if !ok {
		t.Error("expected coercion to succeed for pointer type")
	}
	if len(result.Bids) != 1 {
		t.Error("expected bids to be preserved")
	}
	
	_, ok = coerceBookSnapshot("invalid")
	if ok {
		t.Error("expected coercion to fail for invalid type")
	}
	
	_, ok = coerceBookSnapshot((*schema.BookSnapshotPayload)(nil))
	if ok {
		t.Error("expected coercion to fail for nil pointer")
	}
}

func stringPtr(s string) *string {
	return &s
}
