package binance

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/telemetry"
)

// FrameProvider subscribes to Binance websocket topics.
type FrameProvider interface {
	Subscribe(ctx context.Context, topics []string) (<-chan []byte, <-chan error, error)
}

// WSParser converts websocket frames into canonical events.
type WSParser interface {
	Parse(ctx context.Context, frame []byte, ingestTS time.Time) ([]*schema.Event, error)
}

// WSClient consumes websocket frames and emits canonical events.
type WSClient struct {
	providerName string
	provider     FrameProvider
	parser       WSParser
	clock        func() time.Time
	pools        *pool.PoolManager

	tracer             trace.Tracer
	meter              metric.Meter
	framesCounter      metric.Int64Counter
	frameErrorsCounter metric.Int64Counter
	frameLatency       metric.Float64Histogram
}

// NewWSClient creates a websocket client with the supplied provider and parser.
func NewWSClient(providerName string, provider FrameProvider, parser WSParser, clock func() time.Time, pools *pool.PoolManager) *WSClient {
	if clock == nil {
		clock = time.Now
	}
	client := &WSClient{providerName: providerName, provider: provider, parser: parser, clock: clock, pools: pools}
	client.tracer = otel.Tracer("binance.wsclient")
	client.meter = otel.Meter("binance.wsclient")
	client.framesCounter, _ = client.meter.Int64Counter("wsclient.frames.processed",
		metric.WithDescription("Number of websocket frames processed"),
		metric.WithUnit("{frame}"))
	client.frameErrorsCounter, _ = client.meter.Int64Counter("wsclient.frames.errors",
		metric.WithDescription("Number of websocket frame processing errors"),
		metric.WithUnit("{error}"))
	client.frameLatency, _ = client.meter.Float64Histogram("wsclient.frame.processing.duration",
		metric.WithDescription("Latency to decode and dispatch a websocket frame"),
		metric.WithUnit("ms"))
	return client
}

// Stream subscribes to the given topics and returns canonical events and error notifications.
func (c *WSClient) Stream(ctx context.Context, topics []string) (<-chan *schema.Event, <-chan error) {
	// Buffer events channel for high-frequency streams (Binance sends 900+ msgs/sec)
	events := make(chan *schema.Event, 2048)
	errs := make(chan error, 4)

	frames, providerErrs, err := c.provider.Subscribe(ctx, topics)
	if err != nil {
		go func() {
			defer close(events)
			defer close(errs)
			errs <- err
		}()
		return events, errs
	}

	go func() {
		defer close(events)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			case frame, ok := <-frames:
				if !ok {
					frames = nil
					if providerErrs == nil {
						return
					}
					continue
				}
				c.handleFrame(ctx, events, errs, frame)
			case err, ok := <-providerErrs:
				if !ok {
					providerErrs = nil
					if frames == nil {
						return
					}
					continue
				}
				if err != nil {
					select {
					case errs <- err:
					default:
					}
				}
			}
		}
	}()

	return events, errs
}

func (c *WSClient) handleFrame(ctx context.Context, events chan<- *schema.Event, errs chan<- error, payload []byte) {
	ingestTS := c.clock().UTC()
	start := time.Now()
	status := "success"

	// Parse frame directly without WsFrame wrapper
	parsed, err := c.parser.Parse(ctx, payload, ingestTS)
	if err != nil {
		status = "parse_failed"
		c.recordFrame(err, status, len(payload), time.Since(start))
		select {
		case errs <- err:
		default:
		}
		return
	}

	_, span := c.tracer.Start(ctx, "wsclient.handleFrame",
		trace.WithAttributes(
			attribute.String("provider", c.providerName),
			attribute.Int("frame.size_bytes", len(payload)),
			attribute.Int("events.count", len(parsed)),
		))
	defer span.End()

	for _, evt := range parsed {
		if evt == nil {
			continue
		}
		span.SetAttributes(attribute.String("event.type", string(evt.Type)))
		if evt.IngestTS.IsZero() {
			evt.IngestTS = ingestTS
		}
		if evt.EmitTS.IsZero() {
			evt.EmitTS = ingestTS
		}
		select {
		case <-ctx.Done():
			return
		case events <- evt:
		}
	}
	c.recordFrame(nil, status, len(payload), time.Since(start))
}

func (c *WSClient) recordFrame(err error, status string, size int, elapsed time.Duration) {
	env := telemetry.Environment()
	attrs := telemetry.MessageAttributes(env, c.providerName, status)
	if size > 0 {
		attrs = append(attrs, attribute.Int("frame.size_bytes", size))
	}
	if c.framesCounter != nil {
		c.framesCounter.Add(context.Background(), 1, metric.WithAttributes(attrs...))
	}
	if err != nil && c.frameErrorsCounter != nil {
		errAttrs := telemetry.OperationResultAttributes(env, c.providerName, "frame", status)
		c.frameErrorsCounter.Add(context.Background(), 1, metric.WithAttributes(errAttrs...))
	}
	if c.frameLatency != nil {
		c.frameLatency.Record(context.Background(), float64(elapsed.Milliseconds()), metric.WithAttributes(attrs...))
	}
}


