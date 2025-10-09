# Research Findings — Lightweight Real-Time WebSocket Framework

## Decision 1: JSON decoding strategy with goccy/go-json and pooling
- **Decision**: Use `json.DecoderPool().BorrowDecoder()` from `goccy/go-json` to obtain decoders backed by shared buffers, decode directly into pre-pooled message structs, and call `ReturnDecoder` immediately after handler dispatch.
- **Rationale**: Decoder pooling avoids per-message allocations, honours PERF-03, and integrates with `sync.Pool` managed message envelopes described in the spec. Configuring the decoder with `DisallowUnknownFields()` and `UseNumber()` preserves validation fidelity while remaining faster than the standard library.
- **Alternatives Considered**:
  - `encoding/json`: mature but significantly higher allocations and slower; rejected for failing PERF-03.
  - `jsoniter/go`: fast but introduces additional dependency without native pooling API; would require manual buffer management.

## Decision 2: WebSocket connection orchestration with gorilla/websocket
- **Decision**: Retain `github.com/gorilla/websocket` as the transport client, using dedicated read/write pumps per connection, `SetReadLimit` for frame size enforcement, and custom buffer pools via `websocket.NewPreparedMessage` for outbound fan-out.
- **Rationale**: Gorilla supports Meltica's existing adapters, offers mature control over read deadlines and compression toggles, and integrates with connection heartbeats/close codes. Maintaining a single reader goroutine satisfies CQ-02 separation and simplifies backpressure monitoring hooks.
- **Alternatives Considered**:
  - `nhooyr.io/websocket`: lean API but less alignment with existing Meltica transport abstractions; lacks built-in prepared message support.
  - `gobwas/ws`: lower-level, faster in benchmarks but requires manual masking/compression handling, increasing implementation complexity.

## Decision 3: Backpressure and invalid message handling policy
- **Decision**: Implement a sliding-window counter per connection that tracks validation failures but keeps the socket open, while exposing hooks to emit `errs.CodeInvalid` events and throttle inbound processing when per-connection queues exceed configured depth.
- **Rationale**: Aligns with the clarified requirement to drop invalid messages without disconnecting (UX-04) and provides operators with metrics needed for tuning. Integrating throttling with pooled buffers prevents uncontrolled memory growth and fulfils PERF-03.
- **Alternatives Considered**:
  - Immediate disconnect on first invalid payload: contradicts clarification and harms resilience.
  - Global rate limiter only: insufficient granularity; would not protect individual connections or offer actionable telemetry.
