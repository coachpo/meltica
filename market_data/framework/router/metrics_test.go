package router

import (
	"sync"
	"testing"
	"time"
)

func TestRoutingMetricsRecordAndSnapshot(t *testing.T) {
	metrics := NewRoutingMetrics()
	metrics.RecordRoute("trade")
	metrics.RecordRoute("trade")
	metrics.RecordRoute("account")
	metrics.RecordError()
	metrics.RecordProcessing("trade", time.Millisecond, nil)
	metrics.RecordProcessing("trade", 2*time.Millisecond, assertiveError{})
	metrics.RecordProcessing("account", 5*time.Millisecond, nil)

	snapshot := metrics.Snapshot()
	if snapshot.MessagesRouted["trade"] != 2 {
		t.Fatalf("expected trade count 2, got %d", snapshot.MessagesRouted["trade"])
	}
	if snapshot.MessagesRouted["account"] != 1 {
		t.Fatalf("expected account count 1, got %d", snapshot.MessagesRouted["account"])
	}
	if snapshot.RoutingErrors != 1 {
		t.Fatalf("expected routing errors 1, got %d", snapshot.RoutingErrors)
	}

	inv := snapshot.ProcessorInvocations["trade"]
	if inv.Successes != 1 || inv.Errors != 1 {
		t.Fatalf("unexpected invocation counts %+v", inv)
	}
	lat := snapshot.ConversionDurations["trade"]
	if len(lat.Samples) != 2 {
		t.Fatalf("expected 2 latency samples, got %d", len(lat.Samples))
	}

	// ensure deep copy
	snapshot.MessagesRouted["trade"] = 0
	copyInv := snapshot.ProcessorInvocations["trade"]
	copyInv.Successes = 0
	if metrics.MessagesRouted["trade"] != 2 {
		t.Fatalf("original metrics mutated")
	}
	if metrics.ProcessorInvocations["trade"].Successes != 1 {
		t.Fatalf("original invocation mutated")
	}
}

func TestRoutingMetricsLatencyPercentiles(t *testing.T) {
	metrics := NewRoutingMetrics()
	durations := []time.Duration{1, 2, 3, 4, 5}
	for _, d := range durations {
		metrics.RecordProcessing("trade", d*time.Millisecond, nil)
	}

	hist := metrics.Snapshot().ConversionDurations["trade"]
	if hist.P50 != 3*time.Millisecond {
		t.Fatalf("expected p50 3ms, got %s", hist.P50)
	}
	if hist.P95 != 5*time.Millisecond {
		t.Fatalf("expected p95 5ms, got %s", hist.P95)
	}
	if hist.P99 != 5*time.Millisecond {
		t.Fatalf("expected p99 5ms, got %s", hist.P99)
	}
}

func TestRoutingMetricsConcurrency(t *testing.T) {
	metrics := NewRoutingMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			metrics.RecordRoute("trade")
			metrics.RecordProcessing("trade", time.Millisecond, nil)
			metrics.RecordError()
			metrics.RecordBackpressureStart("trade")
			metrics.RecordBackpressureEnd("trade")
		}()
	}
	wg.Wait()

	snapshot := metrics.Snapshot()
	if snapshot.MessagesRouted["trade"] != 100 {
		t.Fatalf("expected 100 routed messages, got %d", snapshot.MessagesRouted["trade"])
	}
	if snapshot.RoutingErrors != 100 {
		t.Fatalf("expected 100 errors, got %d", snapshot.RoutingErrors)
	}
	if len(snapshot.ConversionDurations["trade"].Samples) != 100 {
		t.Fatalf("expected 100 samples, got %d", len(snapshot.ConversionDurations["trade"].Samples))
	}
	if snapshot.BackpressureEvents != 100 {
		t.Fatalf("expected 100 backpressure events, got %d", snapshot.BackpressureEvents)
	}
	if depth := snapshot.ChannelDepth["trade"]; depth != 0 {
		t.Fatalf("expected trade channel depth 0, got %d", depth)
	}
}

func TestRoutingMetricsBackpressureGauge(t *testing.T) {
	metrics := NewRoutingMetrics()
	metrics.RecordBackpressureStart("trade")
	metrics.RecordBackpressureStart("trade")
	metrics.RecordBackpressureStart("orderbook")
	metrics.RecordBackpressureEnd("trade")

	snapshot := metrics.Snapshot()
	if snapshot.BackpressureEvents != 3 {
		t.Fatalf("expected 3 backpressure events, got %d", snapshot.BackpressureEvents)
	}
	if depth := snapshot.ChannelDepth["trade"]; depth != 1 {
		t.Fatalf("expected trade depth 1, got %d", depth)
	}
	if depth := snapshot.ChannelDepth["orderbook"]; depth != 1 {
		t.Fatalf("expected orderbook depth 1, got %d", depth)
	}

	metrics.RecordBackpressureEnd("trade")
	metrics.RecordBackpressureEnd("orderbook")

	snapshot = metrics.Snapshot()
	if _, ok := snapshot.ChannelDepth["trade"]; ok {
		t.Fatalf("expected trade depth entry removed")
	}
	if _, ok := snapshot.ChannelDepth["orderbook"]; ok {
		t.Fatalf("expected orderbook depth entry removed")
	}
}

type assertiveError struct{}

func (assertiveError) Error() string { return "assertive" }
