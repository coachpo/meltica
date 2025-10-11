package telemetry

import (
	"strings"

	bnwsrouting "github.com/coachpo/meltica/exchanges/binance/wsrouting"
	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

// BridgeSnapshot captures Layer-3 (business) dispatch metrics.
type BridgeSnapshot struct {
	RESTRequests uint64
	RESTFailures uint64
}

// FilterSnapshot captures Layer-4 (filter) forwarding metrics.
type FilterSnapshot struct {
	Forwarded uint64
	Dropped   uint64
}

// LayerSnapshots materializes telemetry for each architecture layer.
type LayerSnapshots struct {
	Connection wsrouting.MetricsSnapshot
	Routing    *bnwsrouting.RoutingMetrics
	Bridge     BridgeSnapshot
	Filter     FilterSnapshot
}

// Collect aggregates the supplied layer-specific telemetry into a single view.
func Collect(connection wsrouting.MetricsSnapshot, routingMetrics *bnwsrouting.RoutingMetrics, bridge BridgeSnapshot, filter FilterSnapshot) LayerSnapshots {
	var routingSnapshot *bnwsrouting.RoutingMetrics
	if routingMetrics != nil {
		routingSnapshot = routingMetrics.Snapshot()
	}
	return LayerSnapshots{
		Connection: connection,
		Routing:    routingSnapshot,
		Bridge:     bridge,
		Filter:     filter,
	}
}

// Flatten exposes the layered telemetry as Prometheus-ready gauge/counter values.
func (s LayerSnapshots) Flatten(prefix string) map[string]float64 {
	if prefix == "" {
		prefix = "binance"
	}
	prefix = strings.TrimSuffix(prefix, "_")
	metrics := map[string]float64{
		prefix + "_l1_messages_total":       float64(s.Connection.MessagesTotal),
		prefix + "_l1_errors_total":         float64(s.Connection.ErrorsTotal),
		prefix + "_l1_allocated_bytes":      float64(s.Connection.Allocated),
		prefix + "_l2_routing_errors_total": 0,
		prefix + "_l2_init_failures_total":  0,
		prefix + "_l2_backpressure_total":   0,
		prefix + "_l3_rest_requests_total":  float64(s.Bridge.RESTRequests),
		prefix + "_l3_rest_failures_total":  float64(s.Bridge.RESTFailures),
		prefix + "_l4_forwarded_total":      float64(s.Filter.Forwarded),
		prefix + "_l4_dropped_total":        float64(s.Filter.Dropped),
	}
	if s.Routing != nil {
		metrics[prefix+"_l2_routing_errors_total"] = float64(s.Routing.RoutingErrors)
		metrics[prefix+"_l2_init_failures_total"] = float64(s.Routing.ProcessorInitFailures)
		metrics[prefix+"_l2_backpressure_total"] = float64(s.Routing.BackpressureEvents)
	}
	return metrics
}
