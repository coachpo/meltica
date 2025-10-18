package controlbus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/coachpo/meltica/internal/errs"
	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/telemetry"
)

// MemoryBus provides an in-memory control bus backed by bounded channels.
type MemoryBus struct {
	cfg MemoryConfig

	ctx    context.Context
	cancel context.CancelFunc

	mu        sync.RWMutex
	consumers []*consumer
	once      sync.Once

	tracer        trace.Tracer
	meter         metric.Meter
	sendDuration  metric.Float64Histogram
	queueDepth    metric.Int64UpDownCounter
	commandErrors metric.Int64Counter
}

type consumer struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan Message
	once   sync.Once
}

// NewMemoryBus constructs a memory-backed control bus.
func NewMemoryBus(cfg MemoryConfig) *MemoryBus {
	cfg = cfg.normalize()
	ctx, cancel := context.WithCancel(context.Background())
	bus := new(MemoryBus)
	bus.cfg = cfg
	bus.ctx = ctx
	bus.cancel = cancel
	bus.tracer = otel.Tracer("controlbus")
	bus.meter = otel.Meter("controlbus")
	bus.sendDuration, _ = bus.meter.Float64Histogram("controlbus.send.duration",
		metric.WithDescription("Latency of control bus send operations"),
		metric.WithUnit("ms"))
	bus.queueDepth, _ = bus.meter.Int64UpDownCounter("controlbus.queue.depth",
		metric.WithDescription("Point-in-time depth of control bus queues"),
		metric.WithUnit("{message}"))
	bus.commandErrors, _ = bus.meter.Int64Counter("controlbus.send.errors",
		metric.WithDescription("Number of control bus send failures"),
		metric.WithUnit("{error}"))
	return bus
}

// Send enqueues the given command and waits for the acknowledgement.
func (b *MemoryBus) Send(ctx context.Context, cmd schema.ControlMessage) (schema.ControlAcknowledgement, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if cmd.Type == "" {
		return schema.ControlAcknowledgement{}, errs.New("controlbus/send", errs.CodeInvalid, errs.WithMessage("command type required"))
	}
	cmdType := string(cmd.Type)
	start := time.Now()
	status := "success"

	defer func() {
		if b.sendDuration != nil {
			attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, status)
			b.sendDuration.Record(ctx, float64(time.Since(start).Milliseconds()), metric.WithAttributes(attrs...))
		}
	}()

	ctx, span := b.tracer.Start(ctx, "controlbus.Send",
		trace.WithAttributes(attribute.String("command.type", cmdType)))
	defer span.End()
	reply := make(chan schema.ControlAcknowledgement, 1)
	message := Message{Command: cmd, Reply: reply}

	b.mu.RLock()
	consumers := append([]*consumer(nil), b.consumers...)
	b.mu.RUnlock()
	if len(consumers) == 0 {
		status = "no_consumers"
		if b.commandErrors != nil {
			attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, status)
			b.commandErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		return schema.ControlAcknowledgement{}, errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("no consumers available"))
	}

	for _, con := range consumers {
		if con == nil {
			continue
		}
		if b.queueDepth != nil {
			attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, "inflight")
			b.queueDepth.Add(ctx, int64(len(con.ch)), metric.WithAttributes(attrs...))
		}
		if con == nil || con.ctx.Err() != nil {
			continue
		}
		if err := b.enqueue(ctx, con, message); err != nil {
			if b.commandErrors != nil {
				attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, "enqueue_failed")
				b.commandErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			status = "enqueue_failed"
			return schema.ControlAcknowledgement{}, err
		}
		ack, err := b.awaitAck(ctx, reply)
		if err != nil {
			span.RecordError(err)
			if b.commandErrors != nil {
				attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, "ack_failed")
				b.commandErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
			}
			status = "ack_failed"
			return schema.ControlAcknowledgement{}, err
		}
		return ack, nil
	}
	status = "no_active_consumers"
	if b.commandErrors != nil {
		attrs := telemetry.CommandAttributes(telemetry.Environment(), cmdType, status)
		b.commandErrors.Add(ctx, 1, metric.WithAttributes(attrs...))
	}
	return schema.ControlAcknowledgement{}, errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("no active consumers"))
}

func (b *MemoryBus) awaitAck(ctx context.Context, reply <-chan schema.ControlAcknowledgement) (schema.ControlAcknowledgement, error) {
	select {
	case <-ctx.Done():
		return schema.ControlAcknowledgement{}, fmt.Errorf("await acknowledgement context: %w", ctx.Err())
	case <-b.ctx.Done():
		return schema.ControlAcknowledgement{}, errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("bus closed"))
	case ack := <-reply:
		return ack, nil
	}
}

// Consume registers a control bus consumer backed by a bounded queue.
func (b *MemoryBus) Consume(ctx context.Context) (<-chan Message, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	con := new(consumer)
	con.ctx = ctx
	con.cancel = cancel
	con.ch = make(chan Message, b.cfg.BufferSize)

	b.mu.Lock()
	b.consumers = append(b.consumers, con)
	b.mu.Unlock()

	go b.observe(con)
	return con.ch, nil
}

// Close shuts down the bus.
func (b *MemoryBus) Close() {
	b.once.Do(func() {
		b.cancel()
		b.mu.Lock()
		for _, con := range b.consumers {
			if con != nil {
				con.close()
			}
		}
		b.consumers = nil
		b.mu.Unlock()
	})
}

func (b *MemoryBus) observe(con *consumer) {
	<-con.ctx.Done()
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, candidate := range b.consumers {
		if candidate == con {
			b.consumers = append(b.consumers[:i], b.consumers[i+1:]...)
			break
		}
	}
	con.close()
}

func (b *MemoryBus) enqueue(ctx context.Context, con *consumer, msg Message) error {
	defer func() {
		if r := recover(); r != nil {
			// consumer closed channel; treat as unavailable.
			_ = r
		}
	}()
	select {
	case <-b.ctx.Done():
		return errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("bus closed"))
	case <-ctx.Done():
		return fmt.Errorf("enqueue context: %w", ctx.Err())
	case <-con.ctx.Done():
		return errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("consumer closed"))
	case con.ch <- msg:
		return nil
	default:
		return errs.New("controlbus/send", errs.CodeUnavailable, errs.WithMessage("consumer queue full"))
	}
}

func (c *consumer) close() {
	c.once.Do(func() {
		c.cancel()
		close(c.ch)
	})
}
