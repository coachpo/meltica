package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/recycler"
	"github.com/coachpo/meltica/internal/schema"
)

// Lambda streams events from the data bus and applies a lambda handler before
// recycling them.
type Lambda struct {
	id            string
	bus           databus.Bus
	rec           recycler.Interface
	logger        *log.Logger
	routingVersion atomic.Int64
}

// NewLambda constructs a lambda consumer that prints received events.
func NewLambda(id string, bus databus.Bus, rec recycler.Interface, logger *log.Logger) *Lambda {
	if id == "" {
		id = "lambda-consumer"
	}
	return &Lambda{
		id:     id,
		bus:    bus,
		rec:    rec,
		logger: logger,
	}
}

// Start subscribes to the requested event types and prints them to stdout.
func (l *Lambda) Start(ctx context.Context, types []schema.EventType) (<-chan error, error) {
	if l.bus == nil {
		return nil, fmt.Errorf("lambda consumer %s: bus required", l.id)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	errs := make(chan error, len(types))
	subs := make([]subscription, 0, len(types))
	for _, typ := range types {
		id, ch, err := l.bus.Subscribe(ctx, typ)
		if err != nil {
			close(errs)
			for _, sub := range subs {
				l.bus.Unsubscribe(sub.id)
			}
			return nil, err
		}
		subs = append(subs, subscription{id: id, typ: typ, ch: ch})
	}
	go l.consume(ctx, subs, errs)
	return errs, nil
}

func (l *Lambda) consume(ctx context.Context, subs []subscription, errs chan<- error) {
	defer close(errs)
	var wg sync.WaitGroup
	wg.Add(len(subs))
	for _, sub := range subs {
		sub := sub
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-sub.ch:
					if !ok {
						return
					}
					l.handleEvent(ctx, sub.typ, evt)
				}
			}
		}()
	}
	wg.Wait()
	for _, sub := range subs {
		l.bus.Unsubscribe(sub.id)
	}
}

func (l *Lambda) handleEvent(ctx context.Context, typ schema.EventType, evt *schema.Event) {
	if evt == nil {
		return
	}
	
	// Check if we should ignore this event based on routing version
	if l.shouldIgnoreEvent(typ, evt) {
		if l.rec != nil {
			l.rec.RecycleEvent(evt)
		}
		return
	}
	
	l.printEvent(typ, evt)
	if l.rec != nil {
		l.rec.RecycleEvent(evt)
	}
	_ = ctx
}

// shouldIgnoreEvent determines if an event should be ignored based on routing version logic.
// Critical event types (ExecReport, ControlAck, ControlResult) are never ignored.
func (l *Lambda) shouldIgnoreEvent(typ schema.EventType, evt *schema.Event) bool {
	if evt == nil {
		return false
	}

	// Never ignore critical event types
	switch typ {
	case schema.EventTypeExecReport:
		return false
	// Note: ControlAck and ControlResult are not schema.EventTypes but could be handled
	// through a different mechanism if needed
	}

	// Check routing version - if event has a routing version and it's older than
	// our current routing version, ignore it
	currentVersion := int(l.routingVersion.Load())
	if evt.RoutingVersion > 0 && evt.RoutingVersion < currentVersion {
		return true
	}

	return false
}

// SetRoutingVersion updates the current routing version for this consumer.
func (l *Lambda) SetRoutingVersion(version int64) {
	l.routingVersion.Store(version)
}

// GetRoutingVersion returns the current routing version for this consumer.
func (l *Lambda) GetRoutingVersion() int64 {
	return l.routingVersion.Load()
}

func (l *Lambda) printEvent(typ schema.EventType, evt *schema.Event) {
	if evt == nil {
		return
	}
	ts := evt.EmitTS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	payload := fmt.Sprintf("%v", evt.Payload)
	line := fmt.Sprintf(
		"[consumer:%s] type=%s provider=%s symbol=%s seq=%d route=%d trace=%s payload=%s",
		l.id,
		typ,
		evt.Provider,
		evt.Symbol,
		evt.SeqProvider,
		evt.RoutingVersion,
		evt.TraceID,
		payload,
	)
	if l.logger != nil {
		l.logger.Println(line)
		return
	}
	fmt.Println(line)
}
