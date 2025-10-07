package pipeline

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
	privateEvents map[string]chan Event
	privateErrors map[string]chan error
	restResults   map[string]Event
}

const (
	accountStream = "account"
	orderStream   = "order"
)

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
		privateEvents: make(map[string]chan Event),
		privateErrors: make(map[string]chan error),
		restResults:   make(map[string]Event),
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
	if _, exists := m.privateEvents[accountStream]; !exists {
		m.privateEvents[accountStream] = make(chan Event, 10)
		m.privateErrors[accountStream] = make(chan error, 10)
	}
	sources = append(sources, PrivateSource{
		Events: m.privateEvents[accountStream],
		Errors: m.privateErrors[accountStream],
	})

	// Order events
	if _, exists := m.privateEvents[orderStream]; !exists {
		m.privateEvents[orderStream] = make(chan Event, 10)
		m.privateErrors[orderStream] = make(chan error, 10)
	}
	sources = append(sources, PrivateSource{
		Events: m.privateEvents[orderStream],
		Errors: m.privateErrors[orderStream],
	})

	return sources, nil
}

func (m *MockAdapter) ExecuteREST(ctx context.Context, req InteractionRequest) (<-chan Event, <-chan error, error) {
	events := make(chan Event, 1)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)

		if result, exists := m.restResults[req.CorrelationID]; exists {
			events <- result
		} else {
			// Default response
			events <- Event{
				Transport:     TransportREST,
				Symbol:        req.Symbol,
				At:            time.Now(),
				CorrelationID: req.CorrelationID,
				Payload: RestResponsePayload{Response: &RestResponse{
					RequestID:  req.CorrelationID,
					Method:     req.Method,
					Path:       req.Path,
					StatusCode: 200,
					Body:       map[string]interface{}{"status": "success"},
				}},
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
	gotBook := false
	gotTrade := false
	for !(gotBook && gotTrade) {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				t.Fatal("Events channel closed unexpectedly")
			}
			if evt.Channel != ChannelPublicWS {
				t.Errorf("Expected channel PublicWS, got %s", evt.Channel)
			}
			switch payload := evt.Payload.(type) {
			case BookPayload:
				if payload.Book == nil {
					t.Fatalf("expected book payload")
				}
				gotBook = true
			case TradePayload:
				if payload.Trade == nil {
					t.Fatalf("expected trade payload")
				}
				gotTrade = true
			default:
				// Ignore other payloads such as ticker/analytics
			}
		case <-ctx.Done():
			t.Fatal("Timeout waiting for events")
		}
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
		case adapter.privateEvents[accountStream] <- Event{
			Transport: TransportPrivateWS,
			Symbol:    "BTC-USDT",
			At:        time.Now(),
			Payload: AccountPayload{Account: &AccountEvent{
				Symbol:    "BTC-USDT",
				Balance:   bigRat("10"),
				Available: bigRat("8"),
				Locked:    bigRat("2"),
			}},
		}:
			t.Log("Account event sent")
		default:
			t.Log("Account event channel blocked")
		}

		select {
		case adapter.privateEvents[orderStream] <- Event{
			Transport: TransportPrivateWS,
			Symbol:    "ETH-USDT",
			At:        time.Now(),
			Payload: OrderPayload{Order: &OrderEvent{
				Symbol:      "ETH-USDT",
				OrderID:     "12345",
				Side:        "BUY",
				Price:       bigRat("3000"),
				Quantity:    bigRat("1"),
				Status:      "FILLED",
				Type:        "LIMIT",
				TimeInForce: "GTC",
			}},
		}:
			t.Log("Order event sent")
		default:
			t.Log("Order event channel blocked")
		}
		t.Log("Private events sent")
	}()

	// Verify private events are received
	gotAccount := false
	gotOrder := false
	for !(gotAccount && gotOrder) {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				if !gotAccount || !gotOrder {
					t.Fatal("Events channel closed before all private events received")
				}
				return
			}
			if evt.Channel != ChannelPrivateWS {
				t.Errorf("Expected channel PrivateWS, got %s", evt.Channel)
			}
			switch payload := evt.Payload.(type) {
			case AccountPayload:
				if payload.Account != nil {
					gotAccount = true
				}
			case OrderPayload:
				if payload.Order != nil {
					gotOrder = true
				}
			}
		case <-ctx.Done():
			t.Fatal("Timeout waiting for private events")
		}
	}

	if !gotAccount || !gotOrder {
		t.Fatalf("expected both account and order events (account=%v, order=%v)", gotAccount, gotOrder)
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
	responses := make(map[string]bool)
	for len(responses) < len(requests) {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				t.Fatalf("Events channel closed before all REST responses were received")
			}
			if evt.Channel != ChannelREST {
				t.Errorf("Expected channel REST, got %s", evt.Channel)
			}
			payload, ok := evt.Payload.(RestResponsePayload)
			if !ok || payload.Response == nil {
				t.Fatalf("expected RestResponse payload, got %T", evt.Payload)
			}
			responses[evt.CorrelationID] = true
		case <-ctx.Done():
			t.Fatal("Timeout waiting for REST responses")
		}
	}
}

// TestMultiChannelMixedWorkflows tests mixed channel workflows
func TestMultiChannelMixedWorkflows(t *testing.T) {
	adapter := NewMockAdapter()
	auth := &AuthContext{APIKey: "test-key", Secret: "test-secret"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Create a request that combines public feeds and REST calls
	req := PipelineRequest{
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
		adapter.privateEvents[accountStream] <- Event{
			Transport: TransportPrivateWS,
			Symbol:    "BTC-USDT",
			At:        time.Now(),
			Payload: AccountPayload{Account: &AccountEvent{
				Symbol:    "BTC-USDT",
				Balance:   bigRat("10"),
				Available: bigRat("8"),
				Locked:    bigRat("2"),
			}},
		}
	}()

	// Verify mixed events are received
	gotPublic := false
	gotPrivate := false
	deadline := time.After(2 * time.Second)
	for !(gotPublic && gotPrivate) {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				t.Fatalf("events channel closed before receiving mixed workflow events")
			}
			switch evt.Channel {
			case ChannelPublicWS:
				if _, ok := evt.Payload.(BookPayload); ok {
					gotPublic = true
				}
			case ChannelPrivateWS:
				if _, ok := evt.Payload.(AccountPayload); ok {
					gotPrivate = true
				}
			}
		case <-deadline:
			t.Fatal("Timeout waiting for mixed events")
		}
	}
}

func bigRat(s string) *big.Rat {
	r, _ := new(big.Rat).SetString(s)
	return r
}
