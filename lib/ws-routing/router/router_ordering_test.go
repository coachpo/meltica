package router_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
	"github.com/coachpo/meltica/lib/ws-routing/router"
	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

func TestRouterPreservesPerSymbolOrdering(t *testing.T) {
	registry := handler.NewRegistry()
	var mu sync.Mutex
	results := make(map[string][]string)
	if err := registry.Register("event", func(_ context.Context, event internal.Event) error {
		mu.Lock()
		results[event.Symbol] = append(results[event.Symbol], string(event.Payload))
		mu.Unlock()
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	unit, err := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	ctx := context.Background()
	events := []struct {
		symbol string
		id     string
	}{
		{symbol: "BTC-USD", id: "1"},
		{symbol: "ETH-USD", id: "1"},
		{symbol: "BTC-USD", id: "2"},
		{symbol: "BTC-USD", id: "3"},
		{symbol: "ETH-USD", id: "2"},
	}
	for _, evt := range events {
		event := internal.Event{Symbol: evt.symbol, Type: "event", Payload: []byte(evt.id), Timestamp: time.Now()}
		if err := unit.Dispatch(ctx, event); err != nil {
			t.Fatalf("dispatch %s/%s: %v", evt.symbol, evt.id, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if got, want := results["BTC-USD"], []string{"1", "2", "3"}; !equalSlice(got, want) {
		t.Fatalf("btc order mismatch: got %v want %v", got, want)
	}
	if got, want := results["ETH-USD"], []string{"1", "2"}; !equalSlice(got, want) {
		t.Fatalf("eth order mismatch: got %v want %v", got, want)
	}
}

func TestRouterAllowsParallelSymbols(t *testing.T) {
	registry := handler.NewRegistry()
	slowReady := make(chan struct{})
	slowRelease := make(chan struct{})
	fastDone := make(chan struct{})
	var once sync.Once
	if err := registry.Register("event", func(_ context.Context, event internal.Event) error {
		switch event.Symbol {
		case "slow":
			once.Do(func() { close(slowReady) })
			<-slowRelease
		case "fast":
			close(fastDone)
		}
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	unit, err := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = unit.Dispatch(ctx, internal.Event{Symbol: "slow", Type: "event"})
	}()
	<-slowReady
	go func() {
		defer wg.Done()
		if err := unit.Dispatch(ctx, internal.Event{Symbol: "fast", Type: "event"}); err != nil {
			t.Errorf("dispatch fast: %v", err)
		}
	}()
	select {
	case <-fastDone:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("fast handler blocked by slow symbol")
	}
	close(slowRelease)
	completed := make(chan struct{})
	go func() {
		wg.Wait()
		close(completed)
	}()
	select {
	case <-completed:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("dispatch goroutines did not finish")
	}
}

func TestRouterBlocksOnBackpressure(t *testing.T) {
	registry := handler.NewRegistry()
	block := make(chan struct{})
	if err := registry.Register("event", func(_ context.Context, event internal.Event) error {
		<-block
		return nil
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	unit, err := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	ctx := context.Background()
	done := make(chan struct{})
	go func() {
		_ = unit.Dispatch(ctx, internal.Event{Symbol: "slow", Type: "event"})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("dispatch returned before handler released")
	case <-time.After(50 * time.Millisecond):
	}
	close(block)
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("dispatch did not resume after releasing handler")
	}
}

func equalSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
