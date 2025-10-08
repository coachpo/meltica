package bootstrap

import (
	"context"
	"fmt"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/exchanges/shared/routing"
)

func ExampleNewTradingService() {
	dispatcher := &stubDispatcher{}
	translator := &stubTranslator{}
	router, _ := routing.NewOrderRouter(
		dispatcher,
		translator,
		routing.WithCapabilities(capabilities.Of(
			capabilities.CapabilitySpotTradingREST,
			capabilities.CapabilityTradingSpotAmend,
			capabilities.CapabilityTradingSpotCancel,
		)),
		routing.WithCapabilityRequirement(routing.ActionPlace, capabilities.CapabilitySpotTradingREST),
	)
	service, _ := NewTradingService(TradingServiceConfig{Router: router})
	result, _ := service.Place(context.Background(), core.OrderRequest{Symbol: "BTC-USDT"})
	fmt.Println(service.Supports(routing.ActionPlace))
	fmt.Println(result.ID)
	// Output:
	// true
	// place
}

func TestNewTradingServiceRequiresRouter(t *testing.T) {
	if _, err := NewTradingService(TradingServiceConfig{}); err == nil {
		t.Fatal("expected error when router is nil")
	}
}

func TestTradingServiceDelegatesToRouter(t *testing.T) {
	dispatcher := &stubDispatcher{}
	translator := &stubTranslator{}
	caps := capabilities.Of(
		capabilities.CapabilitySpotTradingREST,
		capabilities.CapabilityTradingSpotAmend,
		capabilities.CapabilityTradingSpotCancel,
	)
	router, err := routing.NewOrderRouter(
		dispatcher,
		translator,
		routing.WithCapabilities(caps),
		routing.WithCapabilityRequirement(routing.ActionPlace, capabilities.CapabilitySpotTradingREST),
		routing.WithCapabilityRequirement(routing.ActionAmend, capabilities.CapabilitySpotTradingREST, capabilities.CapabilityTradingSpotAmend),
		routing.WithCapabilityRequirement(routing.ActionGet, capabilities.CapabilitySpotTradingREST),
		routing.WithCapabilityRequirement(routing.ActionCancel, capabilities.CapabilityTradingSpotCancel),
	)
	if err != nil {
		t.Fatalf("NewOrderRouter error: %v", err)
	}
	service, err := NewTradingService(TradingServiceConfig{Router: router})
	if err != nil {
		t.Fatalf("NewTradingService error: %v", err)
	}
	if got := service.Capabilities(); got != caps {
		t.Fatalf("expected capabilities %v, got %v", caps, got)
	}
	if !service.Supports(routing.ActionPlace) {
		t.Fatal("expected service to support place action")
	}
	ctx := context.Background()
	order, err := service.Place(ctx, core.OrderRequest{Symbol: "BTC-USDT"})
	if err != nil {
		t.Fatalf("Place error: %v", err)
	}
	if order.ID != "place" {
		t.Fatalf("expected order id 'place', got %s", order.ID)
	}
	if _, err := service.Amend(ctx, routing.OrderAmendRequest{Symbol: "BTC-USDT"}); err != nil {
		t.Fatalf("Amend error: %v", err)
	}
	if _, err := service.Get(ctx, routing.OrderQueryRequest{Symbol: "BTC-USDT"}); err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if err := service.Cancel(ctx, routing.OrderCancelRequest{Symbol: "BTC-USDT"}); err != nil {
		t.Fatalf("Cancel error: %v", err)
	}
	if dispatcher.calls != 4 {
		t.Fatalf("expected 4 dispatch calls, got %d", dispatcher.calls)
	}
}

type stubDispatcher struct {
	calls int
}

func (s *stubDispatcher) Dispatch(context.Context, routing.RESTMessage, any) error {
	s.calls++
	return nil
}

type stubTranslator struct{}

func (stubTranslator) PrepareCreate(ctx context.Context, req core.OrderRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{
		Message: routing.RESTMessage{Method: "POST", Path: "/place"},
		Into:    &struct{}{},
		Decode: func(any) (core.Order, error) {
			return core.Order{ID: "place", Symbol: req.Symbol}, nil
		},
	}, nil
}

func (stubTranslator) PrepareAmend(context.Context, routing.OrderAmendRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{
		Message: routing.RESTMessage{Method: "POST", Path: "/amend"},
		Into:    &struct{}{},
		Decode: func(any) (core.Order, error) {
			return core.Order{ID: "amend"}, nil
		},
	}, nil
}

func (stubTranslator) PrepareGet(context.Context, routing.OrderQueryRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{
		Message: routing.RESTMessage{Method: "GET", Path: "/get"},
		Into:    &struct{}{},
		Decode: func(any) (core.Order, error) {
			return core.Order{ID: "get"}, nil
		},
	}, nil
}

func (stubTranslator) PrepareCancel(context.Context, routing.OrderCancelRequest) (routing.DispatchSpec, error) {
	return routing.DispatchSpec{Message: routing.RESTMessage{Method: "DELETE", Path: "/cancel"}}, nil
}
