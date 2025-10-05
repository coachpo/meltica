# Delivery Process & CI Standards

Defines process, CI checks, and development phases.

## Development Phases
Follow the development phases from bootstrap to release. Each phase has steps and predictable outcomes.

Highlights:
- Transport: timeouts, retries, backoff/jitter, idempotency, bounded retries. Validate with 429/5xx tests.
- Error normalization: unified error type, unit tests.
- WS: auto-reconnect, heartbeats, reliable message delivery. Integration test validation.
- Security: redact secrets, HMAC signing, dependency scanning.
- Observability: structured logs, metrics, traces for debugging.

## DX & Abstractions
- Must freeze: interfaces, models, events, errors, symbol/decimal/enums, capability bitset.
- Validate: Code review and testing

## CI Definition of Done
1. `make test` passes with race detector.
2. `go build ./...` passes.
3. `go test ./... -race -count=1` passes.
4. Exchange adapters compile and pass unit tests.
5. Documentation is up to date.

## Quality Gates
- Core interfaces stable and well-documented
- Exchange adapters functional and tested
- Consistent behavior across different exchanges
- Proper error handling and status mapping
- Comprehensive test coverage
