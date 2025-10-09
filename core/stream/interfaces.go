package stream

import (
	"context"
	"time"
)

// Handler represents user-supplied business logic that operates on decoded message envelopes.
// Implementations MUST be concurrency-safe and return promptly to preserve low-latency processing guarantees.
type Handler interface {
	Handle(ctx context.Context, env Envelope) (Outcome, error)
}

// HandlerFactory constructs a new Handler instance. Engines call factories per session to avoid shared state hazards.
type HandlerFactory func() Handler

// Middleware composes cross-cutting behavior (auth, validation, metrics) around a handler.
// Implementations SHOULD return a handler that delegates to the next handler in the chain.
type Middleware func(next Handler) Handler

// HandlerRegistration captures metadata required to bind a handler to inbound channels.
type HandlerRegistration struct {
	Name        string
	Version     string
	Channels    []string
	Factory     HandlerFactory
	Middleware  []Middleware
	Description string
}

// Engine governs the lifecycle of WebSocket sessions and dispatch pipelines.
type Engine interface {
	RegisterHandler(reg HandlerRegistration) error
	Dial(ctx context.Context) (Session, error)
	Metrics() MetricsReporter
	Close(ctx context.Context) error
}

// Session models a single WebSocket client connection managed by the framework.
type Session interface {
	ID() string
	Status() SessionStatus
	Endpoint() string
	LastHeartbeat() time.Time
	Errors() <-chan error
	Close(ctx context.Context) error
}

// SessionStatus enumerates lifecycle states for a managed connection.
type SessionStatus string

const (
	SessionPending  SessionStatus = "Pending"
	SessionActive   SessionStatus = "Active"
	SessionDraining SessionStatus = "Draining"
	SessionClosed   SessionStatus = "Closed"
)

// Envelope represents a pooled message container reused across dispatch cycles.
type Envelope interface {
	Reset()
	Raw() []byte
	SetRaw([]byte)
	Decoded() any
	SetDecoded(any)
	ReceivedAt() time.Time
	SetReceivedAt(time.Time)
	Validated() bool
	SetValidated(bool)
	Errors() []error
	AppendError(error)
}

// Outcome describes the result emitted by a handler invocation.
type Outcome interface {
	Type() OutcomeType
	Payload() any
	Error() error
	Metadata() map[string]string
	Latency() time.Duration
}

// OutcomeType enumerates handler result categories.
type OutcomeType string

const (
	OutcomeAck       OutcomeType = "Ack"
	OutcomeTransform OutcomeType = "Transform"
	OutcomeError     OutcomeType = "Error"
	OutcomeDrop      OutcomeType = "Drop"
)

// MetricsReporter exposes per-session telemetry snapshots for operators.
type MetricsReporter interface {
	Snapshot(sessionID string) (MetricsSample, bool)
	All() []MetricsSample
}

// MetricsSample provides a view of rolling metrics captured for a session.
type MetricsSample interface {
	SessionID() string
	Window() time.Duration
	Messages() uint64
	Errors() uint64
	P50Latency() time.Duration
	P95Latency() time.Duration
	AllocBytes() uint64
}
