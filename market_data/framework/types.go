package framework

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/stream"
)

// ConnectionSession models an active WebSocket connection managed by the framework.
// It implements stream.Session so core consumers can treat sessions uniformly.
type ConnectionSession struct {
	SessionID    string
	EndpointAddr string
	Protocols    []string
	ConnectedAt  time.Time
	LastBeat     time.Time
	State        stream.SessionStatus
	InvalidCount uint32
	Throughput   *MetricsSnapshot
	BaseContext  context.Context
	Attributes   map[string]string

	errorCh chan error
	closeFn func(context.Context) error
	mu      sync.RWMutex
}

// SetErrorChannel provides the channel used to surface asynchronous session errors.
func (s *ConnectionSession) SetErrorChannel(ch chan error) {
	s.mu.Lock()
	s.errorCh = ch
	s.mu.Unlock()
}

// SetCloseFunc wires the function invoked when Close is called.
func (s *ConnectionSession) SetCloseFunc(fn func(context.Context) error) {
	s.mu.Lock()
	s.closeFn = fn
	s.mu.Unlock()
}

// SetLastHeartbeat records the most recent heartbeat timestamp.
func (s *ConnectionSession) SetLastHeartbeat(t time.Time) {
	s.mu.Lock()
	s.LastBeat = t
	s.mu.Unlock()
}

// SetStatus replaces the current lifecycle status.
func (s *ConnectionSession) SetStatus(status stream.SessionStatus) {
	s.mu.Lock()
	s.State = status
	s.mu.Unlock()
}

// SetContext updates the session base context.
func (s *ConnectionSession) SetContext(ctx context.Context) {
	s.mu.Lock()
	s.BaseContext = ctx
	s.mu.Unlock()
}

// SetInvalidCount updates the sliding window count of invalid payloads observed.
func (s *ConnectionSession) SetInvalidCount(count uint32) {
	s.mu.Lock()
	s.InvalidCount = count
	s.mu.Unlock()
}

// Context returns the current base context for the session.
func (s *ConnectionSession) Context() context.Context {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.BaseContext
}

// SetMetadata stores session-scoped metadata.
func (s *ConnectionSession) SetMetadata(meta map[string]string) {
	s.mu.Lock()
	if len(meta) == 0 {
		s.Attributes = nil
		s.mu.Unlock()
		return
	}
	s.Attributes = make(map[string]string, len(meta))
	for k, v := range meta {
		s.Attributes[k] = v
	}
	s.mu.Unlock()
}

// Metadata returns the stored session metadata.
func (s *ConnectionSession) Metadata() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.Attributes) == 0 {
		return nil
	}
	copyMeta := make(map[string]string, len(s.Attributes))
	for k, v := range s.Attributes {
		copyMeta[k] = v
	}
	return copyMeta
}

// ID returns the session identifier.
func (s *ConnectionSession) ID() string { return s.SessionID }

// Status returns the lifecycle status of the session.
func (s *ConnectionSession) Status() stream.SessionStatus { return s.State }

// Endpoint returns the dialed WebSocket endpoint.
func (s *ConnectionSession) Endpoint() string { return s.EndpointAddr }

// LastHeartbeat returns the last recorded heartbeat time.
func (s *ConnectionSession) LastHeartbeat() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LastBeat
}

// Errors exposes the asynchronous error channel for the session.
func (s *ConnectionSession) Errors() <-chan error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.errorCh
}

// Close invokes the configured close function if present.
func (s *ConnectionSession) Close(ctx context.Context) error {
	s.mu.RLock()
	fn := s.closeFn
	s.mu.RUnlock()
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// SpanContext represents optional trace metadata carried with envelopes.
type SpanContext struct {
	TraceID string
	SpanID  string
}

// MessageEnvelope is a pooled container for decoded payloads.
type MessageEnvelope struct {
	RawData    []byte
	DecodedVal any
	Received   time.Time
	IsValid    bool
	ErrorsList []error
	Span       SpanContext
}

// Reset clears all reusable state on the envelope.
func (e *MessageEnvelope) Reset() {
	e.RawData = e.RawData[:0]
	e.DecodedVal = nil
	e.Received = time.Time{}
	e.IsValid = false
	e.ErrorsList = e.ErrorsList[:0]
	e.Span = SpanContext{}
}

// Raw returns the underlying buffer.
func (e *MessageEnvelope) Raw() []byte { return e.RawData }

// SetRaw sets the underlying buffer content.
func (e *MessageEnvelope) SetRaw(b []byte) { e.RawData = b }

// Decoded returns the decoded payload.
func (e *MessageEnvelope) Decoded() any { return e.DecodedVal }

// SetDecoded stores the decoded payload.
func (e *MessageEnvelope) SetDecoded(v any) { e.DecodedVal = v }

// ReceivedAt returns the timestamp the envelope was created.
func (e *MessageEnvelope) ReceivedAt() time.Time { return e.Received }

// SetReceivedAt assigns the timestamp the envelope entered the pipeline.
func (e *MessageEnvelope) SetReceivedAt(t time.Time) { e.Received = t }

// Validated reports whether the payload has passed validation.
func (e *MessageEnvelope) Validated() bool { return e.IsValid }

// SetValidated marks the validation state of the envelope.
func (e *MessageEnvelope) SetValidated(valid bool) { e.IsValid = valid }

// Errors returns the accumulated validation errors.
func (e *MessageEnvelope) Errors() []error { return e.ErrorsList }

// AppendError records an additional validation error.
func (e *MessageEnvelope) AppendError(err error) {
	if err == nil {
		return
	}
	e.ErrorsList = append(e.ErrorsList, err)
}

// HandlerOutcome represents the result of a handler invocation.
type HandlerOutcome struct {
	OutcomeType stream.OutcomeType
	PayloadData any
	LatencyDur  time.Duration
	Err         error
	Meta        map[string]string
}

// Type returns the outcome category.
func (o HandlerOutcome) Type() stream.OutcomeType { return o.OutcomeType }

// Payload returns the handler payload.
func (o HandlerOutcome) Payload() any { return o.PayloadData }

// Error returns the handler error, if any.
func (o HandlerOutcome) Error() error { return o.Err }

// Metadata returns additional annotations.
func (o HandlerOutcome) Metadata() map[string]string { return o.Meta }

// Latency returns the handler execution duration.
func (o HandlerOutcome) Latency() time.Duration { return o.LatencyDur }

// MetricsSnapshot captures rolling telemetry for a session.
type MetricsSnapshot struct {
	Session       string
	WindowLength  time.Duration
	MessagesTotal uint64
	ErrorsTotal   uint64
	P50           time.Duration
	P95           time.Duration
	Allocated     uint64
}

// SessionID returns the session identifier associated with the snapshot.
func (m MetricsSnapshot) SessionID() string { return m.Session }

// Window returns the aggregation window.
func (m MetricsSnapshot) Window() time.Duration { return m.WindowLength }

// Messages returns the number of processed messages.
func (m MetricsSnapshot) Messages() uint64 { return m.MessagesTotal }

// Errors returns the number of processing errors.
func (m MetricsSnapshot) Errors() uint64 { return m.ErrorsTotal }

// P50Latency returns the median latency for the window.
func (m MetricsSnapshot) P50Latency() time.Duration { return m.P50 }

// P95Latency returns the 95th percentile latency for the window.
func (m MetricsSnapshot) P95Latency() time.Duration { return m.P95 }

// AllocBytes returns the approximate bytes allocated during processing.
func (m MetricsSnapshot) AllocBytes() uint64 { return m.Allocated }
