# Delivery Process & CI Standards

Defines process, CI checks, and migration phases.

## Migration Phases
Follow the 16-phase migration plan (bootstrap → release). Each phase has steps and predictable outcomes.

Highlights:
- Transport: timeouts, retries, backoff/jitter, idempotency, bounded retries. Validate with 429/5xx tests.
- Error normalization: unified error type, golden tests.
- WS: auto-reconnect, heartbeats, soak tests (<0.1% loss). Chaos test validation.
- Security: redact secrets, HMAC signing, SAST/dep/license scans clean.
- Observability: structured logs, metrics, traces, dashboards, spans for order flow.

## DX & Abstractions
- Must freeze: interfaces, models, events, errors, symbol/decimal/enums, capability bitset.
- Validate: Abstraction guideline checklist.

## CI Definition of Done
1. `make test` passes with race detector.
2. `go build ./...` passes.
3. `./protolint ./core ./providers/...` passes.
4. `go run ./cmd/validate-schemas` passes, vectors validate.
5. `go test ./conformance/...` passes offline suites.
6. Protocol changes bump `protocol/version.go` (semver).
7. Provider `SupportedProtocolVersion()` matches `protocol.ProtocolVersion`.
8. Provider file set complete, README updated.
