package telemetry

import (
	"testing"

	framework "github.com/coachpo/meltica/market_data/framework"
	frameworkrouter "github.com/coachpo/meltica/market_data/framework/router"
)

func TestCollectProducesRoutingSnapshot(t *testing.T) {
	conn := framework.MetricsSnapshot{MessagesTotal: 42, ErrorsTotal: 2, Allocated: 1024}
	routingMetrics := frameworkrouter.NewRoutingMetrics()
	routingMetrics.RecordError()
	layers := Collect(conn, routingMetrics, BridgeSnapshot{RESTRequests: 7}, FilterSnapshot{Forwarded: 11, Dropped: 1})

	if layers.Routing == nil {
		t.Fatal("expected routing snapshot to be populated")
	}
	if layers.Routing == routingMetrics {
		t.Fatal("snapshot should not share pointer with input metrics")
	}
	if got := layers.Routing.RoutingErrors; got != 1 {
		t.Fatalf("expected routing errors to equal 1, got %d", got)
	}

	// Mutate original metrics after collection; snapshot should remain stable.
	routingMetrics.RecordError()
	if layers.Routing.RoutingErrors != 1 {
		t.Fatalf("routing snapshot mutated after Collect: got %d", layers.Routing.RoutingErrors)
	}
}

func TestLayerSnapshotsFlatten(t *testing.T) {
	conn := framework.MetricsSnapshot{MessagesTotal: 10, ErrorsTotal: 1, Allocated: 2048}
	routingMetrics := frameworkrouter.NewRoutingMetrics()
	routingMetrics.RecordError()
	routingMetrics.RecordBackpressureStart("binance.trade")
	layers := Collect(conn, routingMetrics, BridgeSnapshot{RESTRequests: 5, RESTFailures: 1}, FilterSnapshot{Forwarded: 9, Dropped: 2})

	flat := layers.Flatten("binance")
	checks := map[string]float64{
		"binance_l1_messages_total":       10,
		"binance_l1_errors_total":         1,
		"binance_l1_allocated_bytes":      2048,
		"binance_l2_routing_errors_total": 1,
		"binance_l2_backpressure_total":   1,
		"binance_l3_rest_requests_total":  5,
		"binance_l3_rest_failures_total":  1,
		"binance_l4_forwarded_total":      9,
		"binance_l4_dropped_total":        2,
	}

	for key, expected := range checks {
		if got, ok := flat[key]; !ok {
			t.Fatalf("expected key %s missing from flattened metrics", key)
		} else if got != expected {
			t.Fatalf("unexpected value for %s: got %v, want %v", key, got, expected)
		}
	}
}
