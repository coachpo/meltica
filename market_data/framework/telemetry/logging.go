package telemetry

import (
	"sync"
	"time"

	"github.com/coachpo/meltica/core/stream"
)

// EventKind categorizes telemetry events emitted by the framework.
type EventKind string

const (
	EventMessageProcessed   EventKind = "message_processed"
	EventHandlerError       EventKind = "handler_error"
	EventValidationFailure  EventKind = "validation_failure"
	EventDecodeFailure      EventKind = "decode_failure"
	EventBackpressureNotice EventKind = "backpressure"
)

// Event captures structured telemetry data for observers.
type Event struct {
	Kind      EventKind
	SessionID string
	Handler   string
	Outcome   stream.OutcomeType
	Latency   time.Duration
	Error     error
	Metadata  map[string]string
	Timestamp time.Time
}

// Subscriber receives telemetry events.
type Subscriber func(Event)

// Emitter coordinates event distribution to subscribers.
type Emitter struct {
	mu          sync.RWMutex
	subscribers map[int]Subscriber
	nextID      int
}

// NewEmitter constructs an empty telemetry emitter.
func NewEmitter() *Emitter {
	return &Emitter{subscribers: make(map[int]Subscriber)}
}

// Emit publishes an event to all subscribers.
func (e *Emitter) Emit(event Event) {
	if e == nil {
		return
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now().UTC()
	}
	if len(event.Metadata) > 0 {
		cloned := make(map[string]string, len(event.Metadata))
		for k, v := range event.Metadata {
			cloned[k] = v
		}
		event.Metadata = cloned
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, subscriber := range e.subscribers {
		if subscriber == nil {
			continue
		}
		subscriber(event)
	}
}

// Subscribe registers a subscriber and returns a function to remove it.
func (e *Emitter) Subscribe(sub Subscriber) func() {
	if e == nil || sub == nil {
		return func() {}
	}
	e.mu.Lock()
	id := e.nextID
	e.nextID++
	e.subscribers[id] = sub
	e.mu.Unlock()
	return func() {
		e.mu.Lock()
		delete(e.subscribers, id)
		e.mu.Unlock()
	}
}
