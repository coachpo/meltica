# Data Model — Lightweight Real-Time WebSocket Framework

## Entities

### ConnectionSession
- **Purpose**: Represents a live WebSocket client connection managed by the framework.
- **Identifier**: `SessionID` (uuid-like string generated at accept time).
- **Fields**:
  - `SessionID string`
  - `Endpoint string` (ws URL, validated via RFC 6455 scheme)
  - `Protocols []string` (negotiated subprotocols; uppercase canonical form)
  - `ConnectedAt time.Time`
  - `LastHeartbeat time.Time`
  - `Status SessionStatus` (enum: `Pending`, `Active`, `Draining`, `Closed`)
  - `InvalidCount uint32` (windowed counter of validation failures)
  - `Throughput MetricsSnapshot` (sliding aggregates for msgs/sec, latency)
  - `Context context.Context` (cancellation propagated downstream)
- **Relationships**:
  - Owns a `PoolHandle` for borrowing message envelopes.
  - Publishes to zero or more `HandlerBinding` instances.
- **Validation Rules**:
  - `Endpoint` must be absolute WS(S) URL.
  - `Protocols` list must not exceed 5 entries and must match `[A-Z0-9-]+`.
  - Status transitions follow state machine defined below.

### MessageEnvelope
- **Purpose**: Reusable container for decoded JSON payloads.
- **Identifier**: Reused objects tracked by pool; no persistent ID.
- **Fields**:
  - `Raw []byte` (buffer borrowed from pool, len <= configured max size)
  - `Decoded interface{}` (pointer to domain struct based on schema)
  - `ReceivedAt time.Time`
  - `Validated bool`
  - `Errors []validation.Error` (typed violations)
  - `Span trace.SpanContext` (optional observability context)
- **Relationships**:
  - Borrowed via `MessagePool.Acquire()` and released with `Release()`.
  - Linked to originating `ConnectionSession` through metadata.
- **Validation Rules**:
  - `Raw` capacity <= declared frame limit.
  - `Decoded` must match registered schema for the subscribed channel.
  - `Errors` empty implies `Validated == true`.

### HandlerOutcome
- **Purpose**: Result emitted by business logic handlers.
- **Fields**:
  - `OutcomeType OutcomeKind` (`Ack`, `Transform`, `Error`, `Drop`)
  - `Payload interface{}` (transformed message or error details)
  - `Latency time.Duration` (handler execution time)
  - `Error *errs.E` (present when `OutcomeType == Error`)
  - `Metadata map[string]string` (annotations for downstream routing)
- **Relationships**:
  - Produced by implementations of `framework.Handler`.
  - Consumed by downstream pipelines or response routers.
- **Validation Rules**:
  - `OutcomeType` must be explicit; no default values.
  - `Error` must be nil unless `OutcomeType == Error`.
  - `Payload` must adhere to canonical symbol + decimal policies if present.

### MetricsSnapshot
- **Purpose**: Captures rolling metrics for a connection.
- **Fields**:
  - `Window time.Duration`
  - `Messages uint64`
  - `Errors uint64`
  - `P50Latency time.Duration`
  - `P95Latency time.Duration`
  - `AllocBytes uint64`
- **Validation Rules**:
  - `Window` must be >= 1s and <= 5m.
  - Latency percentiles computed using HDR histograms to preserve precision.

### PoolHandle
- **Purpose**: Abstraction over pooled resources.
- **Fields**:
  - `EnvelopePool *sync.Pool`
  - `DecoderPool *json.DecoderPool`
  - `BufferPool *bytebufferpool.Pool`
- **Validation Rules**:
  - Pools must be non-nil; initialization occurs during framework bootstrap.

## State Transitions

```
Pending --> Active --> Draining --> Closed
     |         ^          |          ^
     |         |          |          |
     +---------+----------+----------+
```

- **Pending → Active**: Successful handshake and handler registration.
- **Active → Draining**: Triggered when upstream initiates graceful shutdown or throttling threshold exceeded.
- **Draining → Closed**: All in-flight envelopes flushed and connection closed.
- **Active → Closed**: Hard failure (network error, context cancellation) returns typed error.
- **Pending → Closed**: Authentication hook rejects the client.

## Validation Summary

- All enums use exhaustive switches (CQ-05).
- Numeric payloads represented via decimal-compatible structs to honour CQ-03.
- Pools validated at startup; failures surface as `errs.CodeInvalid`.
- Invalid message policy increments `InvalidCount` but retains connection, matching clarification.
