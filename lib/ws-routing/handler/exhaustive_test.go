package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
)

func TestRegistryStoresHandlersByEventType(t *testing.T) {
	registry := handler.NewRegistry()
	alpha := func(context.Context, internal.Event) error { return nil }
	beta := func(context.Context, internal.Event) error { return nil }
	if err := registry.Register("alpha", alpha); err != nil {
		t.Fatalf("register alpha: %v", err)
	}
	if err := registry.Register("beta", beta); err != nil {
		t.Fatalf("register beta: %v", err)
	}
	if handlers := registry.HandlersFor("alpha"); len(handlers) != 1 || handlers[0] == nil {
		t.Fatalf("expected handler for alpha, got %#v", handlers)
	}
	if handlers := registry.HandlersFor("missing"); len(handlers) != 0 {
		t.Fatalf("expected empty slice for unknown event type, got %#v", handlers)
	}
}

func TestRegistryRejectsInvalidRegistrations(t *testing.T) {
	registry := handler.NewRegistry()
	if err := registry.Register("", func(context.Context, internal.Event) error { return nil }); err == nil {
		t.Fatal("expected error for empty event type")
	}
	if err := registry.Register("alpha", nil); err == nil {
		t.Fatal("expected error for nil handler")
	}
}

func TestRegistryClearRemovesHandlers(t *testing.T) {
	registry := handler.NewRegistry()
	_ = registry.Register("alpha", func(context.Context, internal.Event) error { return nil })
	registry.Clear("alpha")
	if handlers := registry.HandlersFor("alpha"); len(handlers) != 0 {
		t.Fatalf("expected handlers cleared, got %#v", handlers)
	}
}

func TestChainApplySequentialMutations(t *testing.T) {
	chain := handler.NewChain()
	first := func(ctx context.Context, event internal.Event) (internal.Event, error) {
		event.Symbol = "A"
		return event, nil
	}
	second := func(ctx context.Context, event internal.Event) (internal.Event, error) {
		event.Type = "processed"
		return event, nil
	}
	if err := chain.Use(first); err != nil {
		t.Fatalf("use first middleware: %v", err)
	}
	if err := chain.Use(second); err != nil {
		t.Fatalf("use second middleware: %v", err)
	}
	result, err := chain.Apply(context.Background(), internal.Event{})
	if err != nil {
		t.Fatalf("apply chain: %v", err)
	}
	if result.Symbol != "A" || result.Type != "processed" {
		t.Fatalf("unexpected mutated event %#v", result)
	}
}

func TestChainRejectsNilMiddleware(t *testing.T) {
	chain := handler.NewChain()
	if err := chain.Use(nil); err == nil {
		t.Fatal("expected error when adding nil middleware")
	}
}

func TestChainApplyWithEmptyChainReturnsEvent(t *testing.T) {
	chain := handler.NewChain()
	event := internal.Event{Symbol: "unchanged"}
	result, err := chain.Apply(context.Background(), event)
	if err != nil {
		t.Fatalf("apply empty chain: %v", err)
	}
	if result.Symbol != "unchanged" {
		t.Fatalf("expected event unchanged, got %#v", result)
	}
}

func TestChainStopsOnError(t *testing.T) {
	chain := handler.NewChain()
	_ = chain.Use(func(ctx context.Context, event internal.Event) (internal.Event, error) {
		return event, errors.New("failure")
	})
	_ = chain.Use(func(ctx context.Context, event internal.Event) (internal.Event, error) {
		t.Fatal("second middleware should not run")
		return event, nil
	})
	if _, err := chain.Apply(context.Background(), internal.Event{}); err == nil {
		t.Fatal("expected error propagated from middleware")
	}
}

func TestChainRecoversFromPanic(t *testing.T) {
	chain := handler.NewChain()
	_ = chain.Use(func(ctx context.Context, event internal.Event) (internal.Event, error) {
		panic("boom")
	})
	if _, err := chain.Apply(context.Background(), internal.Event{}); err == nil {
		t.Fatal("expected panic to be converted into error")
	}
}
