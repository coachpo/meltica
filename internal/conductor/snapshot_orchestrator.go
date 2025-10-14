package conductor

import (
	"context"
	"sync"

	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/snapshot"
)

// SnapshotOrchestrator preserves the historical Meltica orchestration contract used by integration tests.
type SnapshotOrchestrator struct {
	throttle *Throttle

	mu    sync.Mutex
	state map[string]map[string]any
	seq   map[string]uint64
}

// NewOrchestrator constructs a snapshot orchestrator for historical tests.
// The snapshot store parameter is retained for historical usage but is not used by the lightweight implementation.
func NewOrchestrator(_ snapshot.Store, throttle *Throttle) *SnapshotOrchestrator {
	return &SnapshotOrchestrator{
		throttle: throttle,
		state:    make(map[string]map[string]any),
		seq:      make(map[string]uint64),
	}
}

// Run fuses snapshot and delta events, emitting canonical snapshot updates.
func (o *SnapshotOrchestrator) Run(ctx context.Context, events <-chan schema.MelticaEvent) (<-chan schema.MelticaEvent, <-chan error) {
	out := make(chan schema.MelticaEvent, 16)
	errs := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				if fused, emit := o.processEvent(evt); emit {
					if o.throttle == nil || o.throttle.Allow(evt.Instrument) {
						select {
						case <-ctx.Done():
							return
						case out <- fused:
						}
					}
				}
			}
		}
	}()

	return out, errs
}

func (o *SnapshotOrchestrator) processEvent(evt schema.MelticaEvent) (schema.MelticaEvent, bool) {
	if evt.Instrument == "" {
		return schema.MelticaEvent{}, false
	}
	o.mu.Lock()
	defer o.mu.Unlock()

	data := o.ensureState(evt.Instrument)
	switch evt.Type {
	case schema.CanonicalType("ORDERBOOK.SNAPSHOT"):
		copyMapInto(data, payloadAsMap(evt.Payload))
	case schema.CanonicalType("ORDERBOOK.DELTA"), schema.CanonicalType("ORDERBOOK.UPDATE"):
		o.applyDelta(data, payloadAsMap(evt.Payload))
	default:
		return schema.MelticaEvent{}, false
	}

	nextSeq := o.seq[evt.Instrument] + 1
	o.seq[evt.Instrument] = nextSeq

	fused := schema.MelticaEvent{
		Type:           schema.CanonicalType("ORDERBOOK.SNAPSHOT"),
		Source:         evt.Source,
		Ts:             evt.Ts,
		Instrument:     evt.Instrument,
		Market:         evt.Market,
		Seq:            nextSeq,
		TraceID:        evt.TraceID,
		RoutingVersion: evt.RoutingVersion,
		Payload:        cloneMap(data),
	}
	return fused, true
}

func (o *SnapshotOrchestrator) ensureState(instrument string) map[string]any {
	if m, ok := o.state[instrument]; ok && m != nil {
		return m
	}
	m := make(map[string]any)
	o.state[instrument] = m
	return m
}

func (o *SnapshotOrchestrator) applyDelta(state map[string]any, delta map[string]any) {
	if len(delta) == 0 {
		return
	}
	side, _ := delta["side"].(string)
	if price, ok := toFloat(delta["price"]); ok {
		switch side {
		case "bid", "Bid", "BID":
			state["topBid"] = price
		case "ask", "Ask", "ASK":
			state["topAsk"] = price
		}
	}
	if qty, ok := toFloat(delta["qty"]); ok {
		switch side {
		case "bid", "Bid", "BID":
			state["topBidQty"] = qty
		case "ask", "Ask", "ASK":
			state["topAskQty"] = qty
		}
	}
}

func copyMapInto(dst, src map[string]any) {
	if dst == nil || src == nil {
		return
	}
	for k, v := range src {
		dst[k] = v
	}
}

func payloadAsMap(payload any) map[string]any {
	switch v := payload.(type) {
	case map[string]any:
		return cloneMap(v)
	case *map[string]any:
		if v == nil {
			return nil
		}
		return cloneMap(*v)
	default:
		return nil
	}
}

func cloneMap(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func toFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float32:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	default:
		return 0, false
	}
}
