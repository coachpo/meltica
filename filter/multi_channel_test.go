package filter

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

// MockAdapter implements the Adapter interface for testing multi-channel flows
type MockAdapter struct {
	capabilities  Capabilities
	bookEvents    map[string]chan corestreams.BookEvent
	tradeEvents   map[string]chan corestreams.TradeEvent
	tickerEvents  map[string]chan corestreams.TickerEvent
	privateEvents map[EventKind]chan EventEnvelope
	privateErrors map[EventKind]chan error
	restResults   map[string]EventEnvelope
}

func NewMockAdapter() *MockAdapter {
	return &MockAdapter{
		capabilities: Capabilities{
			Books:          true,
			Trades:         true,
			Tickers:        true,
			PrivateStreams: true,
			RESTEndpoints:  true,
		},
		bookEvents:    make(map[string]chan corestreams.BookEvent),
		tradeEvents:   make(map[string]chan corestreams.TradeEvent),
		tickerEvents:  make(map[string]chan corestreams.TickerEvent),
		privateEvents: make(map[EventKind]chan EventEnvelope),
		privateErrors: make(map[EventKind]chan error),
		restResults:   make(map[string]EventEnvelope),
	}
}

func (m *MockAdapter) Capabilities() Capabilities {
	return m.capabilities
}

func (m *MockAdapter) BookSources(ctx context.Context, symbols []string) ([]BookSource, error) {
	sources := make([]BookSource, 0, len(symbols))
	for _, symbol := range symbols {
		if _, exists := m.bookEvents[symbol]; !exists {
			m.bookEvents[symbol] = make(chan corestreams.BookEvent, 10)
		}
		sources = append(sources, BookSource{
			Symbol: symbol,
			Events: m.bookEvents[symbol],
			Errors: make(chan error),
		})
	}
	return sources, nil
}

func (m *MockAdapter) TradeSources(ctx context.Context, symbols []string) ([]TradeSource, error) {
	sources := make([]TradeSource, 0, len(symbols))
	for _, symbol := range symbols {
		if _, exists := m.tradeEvents[symbol]; !exists {
			m.tradeEvents[symbol] = make(chan corestreams.TradeEvent, 10)
		}
		sources = append(sources, TradeSource{
			Symbol: symbol,
			Events: m.tradeEvents[symbol],
			Errors: make(chan error),
		})
	}
	return sources, nil
}

func (m *MockAdapter) TickerSources(ctx context.Context, symbols []string) ([]TickerSource, error) {
	sources := make([]TickerSource, 0, len(symbols))
	for _, symbol := range symbols {
		if _, exists := m.tickerEvents[symbol]; !exists {
			m.tickerEvents[symbol] = make(chan corestreams.TickerEvent, 10)
		}
		sources = append(sources, TickerSource{
			Symbol: symbol,
			Events: m.tickerEvents[symbol],
			Errors: make(chan error),
		})
	}
	return sources, nil
}

func (m *MockAdapter) PrivateSources(ctx context.Context, auth *AuthContext) ([]PrivateSource, error) {
	fmt.Println("MockAdapter.PrivateSources called")
	sources := make([]PrivateSource, 0, 2)

	// Account events
	if _, exists := m.privateEvents[EventKindAccount]; !exists {
		m.privateEvents[EventKindAccount] = make(chan EventEnvelope, 10)
		m.privateErrors[EventKindAccount] = make(chan error, 10)
	}
	sources = append(sources, PrivateSource{
		Kind:   EventKindAccount,
		Events: m.privateEvents[EventKindAccount],
		Errors: m.privateErrors[EventKindAccount],
	})

	// Order events
	if _, exists := m.privateEvents[EventKindOrder]; !exists {
		m.privateEvents[EventKindOrder] = make(chan EventEnvelope, 10)
		m.privateErrors[EventKindOrder] = make(chan error, 10)
	}
	sources = append(sources, PrivateSource{
		Kind:   EventKindOrder,
		Events: m.privateEvents[EventKindOrder],
		Errors: m.privateErrors[EventKindOrder],
	})

	return sources, nil
}

func (m *MockAdapter) ExecuteREST(ctx context.Context, req InteractionRequest) (<-chan EventEnvelope, <-chan error, error) {
	events := make(chan EventEnvelope, 1)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)

		if result, exists := m.restResults[req.CorrelationID]; exists {
			events <- result
		} else {
			// Default response
			events <- EventEnvelope{
				Kind:          EventKindRestResponse,
				Channel:       ChannelREST,
				Symbol:        req.Symbol,
				CorrelationID: req.CorrelationID,
				Timestamp:     time.Now(),
				RestResponse: &RestResponse{
					RequestID:  req.CorrelationID,
					Method:     req.Method,
					Path:       req.Path,
					StatusCode: 200,
					Body:       map[string]interface{}{"status": "success"},
				},
			}
		}
	}()

	return events, errors, nil
}

func (m *MockAdapter) InitPrivateSession(ctx context.Context, auth *AuthContext) error {
	return nil
}

func (m *MockAdapter) Close() {
	// Close all channels
	for _, ch := range m.bookEvents {
		close(ch)
	}
	for _, ch := range m.tradeEvents {
		close(ch)
	}
	for _, ch := range m.tickerEvents {
		close(ch)
	}
	for _, ch := range m.privateEvents {
		close(ch)
	}
	for _, ch := range m.privateErrors {
		close(ch)
	}
}

// TestMultiChannelPublicStreams tests mixed public stream processing
func TestMultiChannelPublicStreams(t *testing.T) {
	adapter := NewMockAdapter()
	facade := NewInteractionFacade(adapter, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := facade.SubscribePublic(ctx, []string{"BTC-USDT", "ETH-USDT"},
		WithBooks(),
		WithTrades(),
		WithTickers(),
		WithBookDepth(5),
	)
	if err != nil {
		t.Fatalf("Failed to subscribe: %v", err)
	}
	defer stream.Close()

	// Send some mock events
	go func() {
		adapter.bookEvents["BTC-USDT"] <- corestreams.BookEvent{
			Symbol: "BTC-USDT",
			Time:   time.Now(),
			Bids:   []core.BookDepthLevel{{Price: bigRat("50000"), Qty: bigRat("1")}},
			Asks:   []core.BookDepthLevel{{Price: bigRat("50001"), Qty: bigRat("1")}},
		}
		adapter.tradeEvents["ETH-USDT"] <- corestreams.TradeEvent{
			Symbol:   "ETH-USDT",
			Time:     time.Now(),
			Price:    bigRat("3000"),
			Quantity: bigRat("0.5"),
		}
	}()

	// Verify events are received
	var receivedEvents []EventKind
	for i := 0; i < 2; i++ {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				t.Fatal("Events channel closed unexpectedly")
			}
			receivedEvents = append(receivedEvents, evt.Kind)
			if evt.Channel != ChannelPublicWS {
				t.Errorf("Expected channel PublicWS, got %s", evt.Channel)
			}
		case <-ctx.Done():
			t.Fatal("Timeout waiting for events")
		}
	}

	if len(receivedEvents) != 2 {
		t.Errorf("Expected 2 events, got %d", len(receivedEvents))
	}
}

// TestMultiChannelPrivateStreams tests private stream processing
func TestMultiChannelPrivateStreams(t *testing.T) {
	adapter := NewMockAdapter()
	auth := &AuthContext{APIKey: "test-key", Secret: "test-secret"}
	facade := NewInteractionFacade(adapter, auth)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	stream, err := facade.SubscribePrivate(ctx)
	if err != nil {
		t.Fatalf("Failed to subscribe private: %v", err)
	}
	defer stream.Close()

	// Send mock private events
	go func() {
		// Give the pipeline time to initialize
		time.Sleep(100 * time.Millisecond)

		t.Log("Sending private events...")
		select {
		case adapter.privateEvents[EventKindAccount] <- EventEnvelope{
			Kind:      EventKindAccount,
			Channel:   ChannelPrivateWS,
			Symbol:    "BTC-USDT",
			Timestamp: time.Now(),
			AccountEvent: &AccountEvent{
				Symbol:    "BTC-USDT",
				Balance:   bigRat("10"),
				Available: bigRat("8"),
				Locked:    bigRat("2"),
			},
		}:
			t.Log("Account event sent")
		default:
			t.Log("Account event channel blocked")
		}

		select {
		case adapter.privateEvents[EventKindOrder] <- EventEnvelope{
			Kind:      EventKindOrder,
			Channel:   ChannelPrivateWS,
			Symbol:    "ETH-USDT",
			Timestamp: time.Now(),
			OrderEvent: &OrderEvent{
				Symbol:      "ETH-USDT",
				OrderID:     "12345",
				Side:        "BUY",
				Price:       bigRat("3000"),
				Quantity:    bigRat("1"),
				Status:      "FILLED",
				Type:        "LIMIT",
				TimeInForce: "GTC",
			},
		}:
			t.Log("Order event sent")
		default:
			t.Log("Order event channel blocked")
		}
		t.Log("Private events sent")
	}()

	// Verify private events are received
	var receivedEvents []EventKind
	for i := 0; i < 2; i++ {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				if i == 0 {
					t.Fatal("Events channel closed unexpectedly")
				}
				break
			}
			t.Logf("Received event: %s from channel: %s", evt.Kind, evt.Channel)
			receivedEvents = append(receivedEvents, evt.Kind)
			if evt.Channel != ChannelPrivateWS {
				t.Errorf("Expected channel PrivateWS, got %s", evt.Channel)
			}
		case <-ctx.Done():
			if i == 0 {
				t.Fatal("Timeout waiting for private events")
			}
			break
		}
	}

	if len(receivedEvents) < 1 {
		t.Errorf("Expected at least 1 event, got %d", len(receivedEvents))
	}
}

// TestMultiChannelRESTRequests tests REST API call processing
func TestMultiChannelRESTRequests(t *testing.T) {
	adapter := NewMockAdapter()
	facade := NewInteractionFacade(adapter, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	requests := []InteractionRequest{
		GetAccountInfo("req-1"),
		GetOpenOrders("BTC-USDT", "req-2"),
	}

	stream, err := facade.FetchREST(ctx, requests)
	if err != nil {
		t.Fatalf("Failed to execute REST requests: %v", err)
	}
	defer stream.Close()

	// Verify REST responses are received
	var receivedResponses []string
	for i := 0; i < 2; i++ {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				if i == 0 {
					t.Fatal("Events channel closed unexpectedly")
				}
				break
			}
			if evt.Kind != EventKindRestResponse {
				t.Errorf("Expected RestResponse, got %s", evt.Kind)
			}
			if evt.Channel != ChannelREST {
				t.Errorf("Expected channel REST, got %s", evt.Channel)
			}
			receivedResponses = append(receivedResponses, evt.CorrelationID)
		case <-ctx.Done():
			if i == 0 {
				t.Fatal("Timeout waiting for REST responses")
			}
			break
		}
	}

	if len(receivedResponses) < 1 {
		t.Errorf("Expected at least 1 response, got %d", len(receivedResponses))
	}
}

// TestMultiChannelMixedWorkflows tests mixed channel workflows
func TestMultiChannelMixedWorkflows(t *testing.T) {
	adapter := NewMockAdapter()
	auth := &AuthContext{APIKey: "test-key", Secret: "test-secret"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Create a request that combines public feeds and REST calls
	req := FilterRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds: FeedSelection{
			Books:   true,
			Trades:  true,
			Tickers: true,
		},
		EnablePrivate: true,
		RESTRequests: []InteractionRequest{
			GetAccountInfo("mixed-req-1"),
		},
		EnableSnapshots: true,
	}

	coordinator := NewCoordinator(adapter, auth)
	stream, err := coordinator.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Failed to create mixed workflow: %v", err)
	}
	defer stream.Close()

	// Send events from all channels
	go func() {
		// Give the pipeline time to initialize
		time.Sleep(100 * time.Millisecond)

		// Public event
		adapter.bookEvents["BTC-USDT"] <- corestreams.BookEvent{
			Symbol: "BTC-USDT",
			Time:   time.Now(),
			Bids:   []core.BookDepthLevel{{Price: bigRat("50000"), Qty: bigRat("1")}},
			Asks:   []core.BookDepthLevel{{Price: bigRat("50001"), Qty: bigRat("1")}},
		}
		// Private event
		adapter.privateEvents[EventKindAccount] <- EventEnvelope{
			Kind:      EventKindAccount,
			Channel:   ChannelPrivateWS,
			Timestamp: time.Now(),
			AccountEvent: &AccountEvent{
				Symbol:    "BTC-USDT",
				Balance:   bigRat("10"),
				Available: bigRat("8"),
				Locked:    bigRat("2"),
			},
		}
	}()

	// Verify mixed events are received
	var receivedChannels []ChannelType
	for i := 0; i < 2; i++ { // book + account (REST response may not come immediately)
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				if i == 0 {
					t.Fatal("Events channel closed unexpectedly")
				}
				break
			}
			receivedChannels = append(receivedChannels, evt.Channel)
		case <-ctx.Done():
			if i == 0 {
				t.Fatal("Timeout waiting for mixed events")
			}
			break
		}
	}

	// Should have events from at least 1 channel
	if len(receivedChannels) < 1 {
		t.Errorf("Expected at least 1 event, got %d", len(receivedChannels))
	}
}

func bigRat(s string) *big.Rat {
	r, _ := new(big.Rat).SetString(s)
	return r
}
