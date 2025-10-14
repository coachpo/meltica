package recycler

import (
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/coachpo/meltica/core/events"
)

func newTestRecycler(t *testing.T) *RecyclerImpl {
	t.Helper()
	eventPool := &sync.Pool{New: func() any { return &events.Event{} }}
	mergedPool := &sync.Pool{New: func() any { return &events.MergedEvent{} }}
	execPool := &sync.Pool{New: func() any { return &events.ExecReport{} }}
	metrics := NewRecyclerMetrics(prometheus.NewRegistry())
	return NewRecycler(eventPool, mergedPool, execPool, metrics)
}

func TestRecycleEventResetsAndRecordsMetrics(t *testing.T) {
	metrics := NewRecyclerMetrics(prometheus.NewRegistry())
	eventPool := &sync.Pool{New: func() any { return &events.Event{} }}
	recycler := NewRecycler(eventPool, &sync.Pool{}, &sync.Pool{}, metrics)

	ev := &events.Event{
		TraceID:        "trace",
		RoutingVersion: 7,
		Kind:           events.KindExecReport,
		Payload:        struct{}{},
		IngestTS:       time.Now(),
		SeqProvider:    9,
		ProviderID:     "binance",
	}

	recycler.RecycleEvent(ev)

	if ev.TraceID != "" || ev.RoutingVersion != 0 || ev.Kind != 0 || ev.Payload != nil || !ev.IngestTS.IsZero() || ev.SeqProvider != 0 || ev.ProviderID != "" {
		t.Fatalf("event fields not reset: %+v", ev)
	}

	got := testutil.ToFloat64(metrics.recycleTotal.WithLabelValues(events.KindExecReport.String()))
	if got != 1 {
		t.Fatalf("expected recycle counter to be 1, got %f", got)
	}
}

func TestRecycleManyIgnoresNilEvents(t *testing.T) {
	metrics := NewRecyclerMetrics(prometheus.NewRegistry())
	recycler := NewRecycler(&sync.Pool{New: func() any { return &events.Event{} }}, &sync.Pool{}, &sync.Pool{}, metrics)

	recycler.RecycleMany([]*events.Event{
		nil,
		{Kind: events.KindMarketData},
		nil,
		{Kind: events.KindExecReport},
	})

	market := testutil.ToFloat64(metrics.recycleTotal.WithLabelValues(events.KindMarketData.String()))
	exec := testutil.ToFloat64(metrics.recycleTotal.WithLabelValues(events.KindExecReport.String()))
	if market != 1 || exec != 1 {
		t.Fatalf("expected counters to increment once each, got market=%f exec=%f", market, exec)
	}
}

func TestDebugDoublePutDetection(t *testing.T) {
	metrics := NewRecyclerMetrics(prometheus.NewRegistry())
	recycler := NewRecycler(&sync.Pool{}, &sync.Pool{}, &sync.Pool{}, metrics)
	recycler.EnableDebugMode()
	ev := &events.Event{}
	recycler.RecycleEvent(ev)

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on double recycle")
		}
		if got := testutil.ToFloat64(metrics.doublePutTotal); got != 1 {
			t.Fatalf("expected double-put counter increment, got %f", got)
		}
	}()

	recycler.RecycleEvent(ev)
}

func TestCheckoutAllowsReuseInDebugMode(t *testing.T) {
	recycler := NewRecycler(&sync.Pool{}, &sync.Pool{}, &sync.Pool{}, NewRecyclerMetrics(prometheus.NewRegistry()))
	recycler.EnableDebugMode()
	ev := &events.Event{}
	recycler.RecycleEvent(ev)
	recycler.CheckoutEvent(ev)

	// Should not panic after checkout.
	recycler.RecycleEvent(ev)
}

func TestRecycleExecReportResetsFields(t *testing.T) {
	recycler := NewRecycler(&sync.Pool{}, &sync.Pool{}, &sync.Pool{}, NewRecyclerMetrics(prometheus.NewRegistry()))
	report := &events.ExecReport{TraceID: "t", ClientOrderID: "c", ExchangeID: "e", Status: "s", Reason: "r"}
	recycler.RecycleExecReport(report)
	if report.TraceID != "" || report.ClientOrderID != "" || report.ExchangeID != "" || report.Status != "" || report.Reason != "" {
		t.Fatalf("exec report not reset: %+v", report)
	}
}

func TestRecycleMergedEventResetsAndReturnsToPool(t *testing.T) {
	mergedPool := &sync.Pool{New: func() any { return &events.MergedEvent{} }}
	recycler := NewRecycler(&sync.Pool{}, mergedPool, &sync.Pool{}, NewRecyclerMetrics(prometheus.NewRegistry()))
	recycler.EnableDebugMode()
	merged := &events.MergedEvent{Event: events.Event{TraceID: "x"}, SourceProviders: []string{"a"}, MergeWindowID: "id"}
	recycler.RecycleMergedEvent(merged)
	if merged.TraceID != "" || len(merged.SourceProviders) != 0 || merged.MergeWindowID != "" {
		t.Fatalf("merged event not reset: %+v", merged)
	}
}

func TestGlobalInitialization(t *testing.T) {
	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		_ = Global()
	}()
	if !didPanic {
		t.Fatalf("expected panic when accessing global before initialization")
	}

	eventPool := &sync.Pool{New: func() any { return &events.Event{} }}
	mergedPool := &sync.Pool{New: func() any { return &events.MergedEvent{} }}
	execPool := &sync.Pool{New: func() any { return &events.ExecReport{} }}
	metrics := NewRecyclerMetrics(prometheus.NewRegistry())
	InitGlobal(eventPool, mergedPool, execPool, metrics)

	if Global() == nil {
		t.Fatalf("expected global recycler instance")
	}
}
