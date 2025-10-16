package dispatcher

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/config"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

// Runtime coordinates dispatcher ingestion and delivery.
type Runtime struct {
	bus            databus.Bus
	table          *Table
	pools          *pool.PoolManager
	cfg            config.DispatcherRuntimeConfig
	ordering       *StreamOrdering
	clock          func() time.Time
	dedupe         map[string]time.Time
	dedupeWindow   time.Duration
	dedupeCapacity int

	tracer                trace.Tracer
	eventsIngestedCounter metric.Int64Counter
	eventsDroppedCounter  metric.Int64Counter
	eventsDuplicateCounter metric.Int64Counter
	eventsBufferedGauge   metric.Int64UpDownCounter
	processingDuration    metric.Float64Histogram
}

// NewRuntime constructs a dispatcher runtime instance.
func NewRuntime(bus databus.Bus, table *Table, pools *pool.PoolManager, cfg config.DispatcherRuntimeConfig, _ interface{}) *Runtime {
	clock := time.Now
	ordering := NewStreamOrdering(cfg.StreamOrdering, clock)
	runtime := new(Runtime)
	runtime.bus = bus
	runtime.table = table
	runtime.pools = pools
	runtime.cfg = cfg
	runtime.ordering = ordering
	runtime.clock = clock
	runtime.dedupe = make(map[string]time.Time, 1024)
	runtime.dedupeWindow = 5 * time.Minute
	runtime.dedupeCapacity = 8192

	runtime.tracer = otel.Tracer("dispatcher")
	meter := otel.Meter("dispatcher")
	runtime.eventsIngestedCounter, _ = meter.Int64Counter("dispatcher.events.ingested",
		metric.WithDescription("Number of events ingested by dispatcher"),
		metric.WithUnit("{event}"))
	runtime.eventsDroppedCounter, _ = meter.Int64Counter("dispatcher.events.dropped",
		metric.WithDescription("Number of events dropped"),
		metric.WithUnit("{event}"))
	runtime.eventsDuplicateCounter, _ = meter.Int64Counter("dispatcher.events.duplicate",
		metric.WithDescription("Number of duplicate events detected"),
		metric.WithUnit("{event}"))
	runtime.eventsBufferedGauge, _ = meter.Int64UpDownCounter("dispatcher.events.buffered",
		metric.WithDescription("Number of events currently buffered"),
		metric.WithUnit("{event}"))
	runtime.processingDuration, _ = meter.Float64Histogram("dispatcher.processing.duration",
		metric.WithDescription("Event processing duration"),
		metric.WithUnit("ms"))

	return runtime
}

// Start consumes canonical events and delivers them onto the data bus until the context is cancelled.
func (r *Runtime) Start(ctx context.Context, events <-chan *schema.Event) <-chan error {
	errCh := make(chan error, 4)
	go r.run(ctx, events, errCh)
	return errCh
}

func (r *Runtime) run(ctx context.Context, events <-chan *schema.Event, errCh chan<- error) {
	flushInterval := r.cfg.StreamOrdering.FlushInterval
	if flushInterval <= 0 {
		flushInterval = 50 * time.Millisecond
	}
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	defer close(errCh)

	publish := func(batch []*schema.Event) {
		if len(batch) == 0 {
			return
		}
		_, span := r.tracer.Start(ctx, "dispatcher.publish_batch",
			trace.WithAttributes(attribute.Int("batch.size", len(batch))))
		defer span.End()

		for i, evt := range batch {
			if evt == nil {
				batch[i] = nil
				continue
			}
			if evt.Provider == "" {
				evt.Provider = "binance"
			}
			// Pass original event to bus; bus handles routing and cloning
			if err := r.bus.Publish(ctx, evt); err != nil {
				span.RecordError(err)
				select {
				case errCh <- err:
				default:
				}
			}
			batch[i] = nil
		}
	}

	for {
		select {
		case <-ctx.Done():
			publish(r.ordering.Flush(r.clock()))
			return
		case evt, ok := <-events:
			if !ok {
				publish(r.ordering.Flush(r.clock()))
				return
			}
			if evt == nil {
				continue
			}

			start := r.clock()
			_, span := r.tracer.Start(ctx, "dispatcher.process_event",
				trace.WithAttributes(
					attribute.String("event.type", string(evt.Type)),
					attribute.String("event.provider", evt.Provider),
					attribute.String("event.id", evt.EventID),
				))

			if r.eventsIngestedCounter != nil {
				r.eventsIngestedCounter.Add(ctx, 1, metric.WithAttributes(
					attribute.String("event.type", string(evt.Type)),
					attribute.String("provider", evt.Provider)))
			}

			if rv := r.currentRoutingVersion(); rv > 0 {
				evt.RoutingVersion = rv
			}
			if evt.EmitTS.IsZero() {
				evt.EmitTS = r.clock().UTC()
			}
			if !r.markSeen(evt.EventID) {
				if r.eventsDuplicateCounter != nil {
					r.eventsDuplicateCounter.Add(ctx, 1, metric.WithAttributes(
						attribute.String("event.type", string(evt.Type))))
				}
				span.AddEvent("duplicate_event_dropped")
				r.releaseEvent(evt)
				span.End()
				continue
			}
			ready, buffered := r.ordering.OnEvent(evt)
			if !buffered {
				if r.eventsDroppedCounter != nil {
					r.eventsDroppedCounter.Add(ctx, 1, metric.WithAttributes(
						attribute.String("event.type", string(evt.Type)),
						attribute.String("reason", "not_buffered")))
				}
				r.releaseEvent(evt)
			} else {
				if r.eventsBufferedGauge != nil {
					r.eventsBufferedGauge.Add(ctx, 1, metric.WithAttributes(
						attribute.String("event.type", string(evt.Type))))
				}
			}

			duration := r.clock().Sub(start).Milliseconds()
			if r.processingDuration != nil {
				r.processingDuration.Record(ctx, float64(duration), metric.WithAttributes(
					attribute.String("event.type", string(evt.Type))))
			}

			span.End()
			publish(ready)

			if len(ready) > 0 && r.eventsBufferedGauge != nil {
				r.eventsBufferedGauge.Add(ctx, -int64(len(ready)), metric.WithAttributes(
					attribute.String("event.type", string(evt.Type))))
			}
		case <-ticker.C:
			flushed := r.ordering.Flush(r.clock())
			if len(flushed) > 0 && r.eventsBufferedGauge != nil {
				r.eventsBufferedGauge.Add(ctx, -int64(len(flushed)))
			}
			publish(flushed)
		}
	}
}

func (r *Runtime) markSeen(eventID string) bool {
	if eventID == "" {
		return true
	}
	now := r.clock().UTC()
	if ts, ok := r.dedupe[eventID]; ok {
		if now.Sub(ts) < r.dedupeWindow {
			return false
		}
	}
	r.dedupe[eventID] = now
	if len(r.dedupe) > r.dedupeCapacity {
		r.gcDedupe(now)
	}
	return true
}

func (r *Runtime) gcDedupe(now time.Time) {
	threshold := now.Add(-r.dedupeWindow)
	for id, ts := range r.dedupe {
		if ts.Before(threshold) {
			delete(r.dedupe, id)
		}
	}
}

func (r *Runtime) currentRoutingVersion() int {
	if r.table == nil {
		return 0
	}
	return int(r.table.Version())
}

func (r *Runtime) releaseEvent(evt *schema.Event) {
	if r.pools != nil {
		r.pools.RecycleCanonicalEvent(evt)
	}
}
