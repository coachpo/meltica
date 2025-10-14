package orchestrator

import (
	"context"
	"sync"
	"testing"

	"github.com/coachpo/meltica/core/events"
)

type mergerRecyclerStub struct {
	mu       sync.Mutex
	partials []*events.Event
	merged   []*events.MergedEvent
}

func (m *mergerRecyclerStub) RecycleEvent(ev *events.Event) {
	if ev == nil {
		return
	}
	m.mu.Lock()
	m.partials = append(m.partials, ev)
	m.mu.Unlock()
}

func (m *mergerRecyclerStub) RecycleMergedEvent(ev *events.MergedEvent) {
	if ev == nil {
		return
	}
	m.mu.Lock()
	m.merged = append(m.merged, ev)
	m.mu.Unlock()
}

func (m *mergerRecyclerStub) RecycleExecReport(*events.ExecReport) {}
func (m *mergerRecyclerStub) RecycleMany(events []*events.Event) {
	for _, ev := range events {
		m.RecycleEvent(ev)
	}
}
func (m *mergerRecyclerStub) EnableDebugMode()                        {}
func (m *mergerRecyclerStub) DisableDebugMode()                       {}
func (m *mergerRecyclerStub) CheckoutEvent(*events.Event)             {}
func (m *mergerRecyclerStub) CheckoutMergedEvent(*events.MergedEvent) {}
func (m *mergerRecyclerStub) CheckoutExecReport(*events.ExecReport)   {}

func TestMergerCombinePartials(t *testing.T) {
	recycler := &mergerRecyclerStub{}
	pool := &sync.Pool{New: func() any { return &events.MergedEvent{} }}
	merger := NewMerger(pool, recycler)

	partials := []*events.Event{
		{TraceID: "t", Kind: events.KindMarketData, ProviderID: "binance", RoutingVersion: 3},
		{TraceID: "ignored", ProviderID: "coinbase"},
		nil,
	}

	merged, err := merger.MergeEvents(context.Background(), partials)
	if err != nil {
		t.Fatalf("MergeEvents returned error: %v", err)
	}
	if merged == nil {
		t.Fatalf("expected merged event")
	}
	if merged.TraceID != "t" || merged.RoutingVersion != 3 || merged.Kind != events.KindMarketData {
		t.Fatalf("merged event not composed correctly: %+v", merged)
	}
	expectedProviders := []string{"binance", "coinbase"}
	if len(merged.SourceProviders) != len(expectedProviders) {
		t.Fatalf("expected providers %v, got %v", expectedProviders, merged.SourceProviders)
	}
	for i, provider := range expectedProviders {
		if merged.SourceProviders[i] != provider {
			t.Fatalf("expected provider %s at index %d, got %s", provider, i, merged.SourceProviders[i])
		}
	}
	if len(recycler.partials) != 2 {
		t.Fatalf("expected two partials recycled, got %d", len(recycler.partials))
	}
}

func TestMergerHandlesEmptyPartials(t *testing.T) {
	merger := NewMerger(nil, &mergerRecyclerStub{})
	merged, err := merger.MergeEvents(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged != nil {
		t.Fatalf("expected nil merged event for empty partials")
	}
}

func TestMergerRecycleMergedReturnsToPool(t *testing.T) {
	recycler := &mergerRecyclerStub{}
	pool := &sync.Pool{New: func() any { return &events.MergedEvent{} }}
	merger := NewMerger(pool, recycler)

	merged := &events.MergedEvent{Event: events.Event{TraceID: "t"}}
	merger.RecycleMerged(merged)

	value := pool.Get()
	if value != merged {
		t.Fatalf("expected merged event returned to pool")
	}
	if len(recycler.merged) != 1 {
		t.Fatalf("expected recycler to track merged recycle")
	}
}

func TestStamperUpdatesAndStamps(t *testing.T) {
	stamper := NewStamper(10)
	if stamper.CurrentVersion() != 10 {
		t.Fatalf("expected initial version 10")
	}
	stamper.UpdateVersion(42)
	if stamper.CurrentVersion() != 42 {
		t.Fatalf("expected updated version 42")
	}
	ev := &events.Event{}
	stamper.StampRoutingVersion(ev)
	if ev.RoutingVersion != 42 {
		t.Fatalf("expected stamped version 42, got %d", ev.RoutingVersion)
	}
}
