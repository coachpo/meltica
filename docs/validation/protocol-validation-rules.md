### Protocol Validation Rules

This document defines validation rules for the meltica protocol. Each rule lists an ID, intent, scope, and validation standard.

- Severity:
  - MUST: failing blocks CI.
  - SHOULD: failing warns; may block in strict mode.
  - INFO: advisory.
- Unless noted, rules apply to production code (excluding `**/test/**`, `**/*_test.go`).
- Paths use the actual Go-based layout: `core/`, `exchanges/<name>/`, `config/`, `errs/`, `cmd/`.

---

### Core API & Interfaces

1) CORE-IFACE-EXCHANGE (MUST)
- Intent: Freeze `Exchange` interface surface.
- Scope: `core/`.
- Validation standard:
  - Using Go AST, find interface `Exchange` in `core` package.
  - Its method set MUST be exactly:
    - `Name() string`
    - `Capabilities() ExchangeCapabilities`
    - `SupportedProtocolVersion() string`
  - No extra methods. Method names, parameter and result types MUST match exactly.

2) CORE-IFACE-SPOT (MUST)
- Intent: Freeze spot market interface surface.
- Scope: `core/`.
- Validation standard:
  - Go AST must contain spot market interface methods with names and signatures exactly matching the current implementation in `core/core.go`.

3) CORE-IFACE-FUTURES (MUST)
- Intent: Freeze futures market interface surface.
- Scope: `core/`.
- Validation standard:
  - Same approach as CORE-IFACE-SPOT, comparing against `core/core.go` implementation.

4) CORE-IFACE-WS (MUST)
- Intent: Freeze WebSocket interface surface.
- Scope: `core/`.
- Validation standard:
  - Go AST must contain WebSocket interface methods matching current signatures. No extra/removed methods.

5) CORE-MODELS-PRESENT (MUST)
- Intent: Freeze model types and doc comments.
- Scope: `core/`.
- Validation standard:
  - Types MUST exist: `Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline`.
  - Each type declaration must be preceded by a non-empty doc comment.

6) CORE-EVENTS-PRESENT (MUST)
- Intent: Freeze WS event types.
- Scope: `core/`.
- Validation standard:
  - Types MUST exist: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.

7) CORE-ERRORS-TYPE (MUST)
- Intent: Freeze canonical error type and codes mapping.
- Scope: `errs/` package.
- Validation standard:
  - Type `E` MUST be declared as a named type in package `errs` and used by exported constructors.
  - Canonical code enum/constants declared and exported.
  - Public APIs in `core/` MUST return `*errs.E` for error values.

8) CORE-CAPABILITIES-BITSET (MUST)
- Intent: Freeze capabilities presence and type.
- Scope: `core/`.
- Validation standard:
  - Type `ExchangeCapabilities` exists and is a bitset-friendly integer type (`uint64` or alias).
  - `Exchange.Capabilities()` method result type MUST be `ExchangeCapabilities`.

9) CORE-NO-FLOATS (MUST)
- Intent: Enforce decimal policy in critical paths.
- Scope: `core/`, `exchanges/**` (production code only).
- Validation standard:
  - Disallow identifiers of type `float32` or `float64` in:
    - Public structs (exported) fields.
    - Public function/method parameters and return types.
    - Marshal/Unmarshal implementations.

10) CORE-DECIMAL-BIGRAT (MUST)
- Intent: Numbers are represented by `*big.Rat`.
- Scope: `core/` models and any serialized representation.
- Validation standard:
  - All numeric price/size/amount/qty fields in the model types MUST have type `*big.Rat`.

11) CORE-SYMBOL-CANONICAL (MUST)
- Intent: Enforce canonical symbol form `BASE-QUOTE` uppercase.
- Scope: `core/symbols.go`, `exchanges/**`.
- Validation standard:
  - `core` exposes helpers (e.g., `ToCanonicalSymbol`, `ParseSymbol`). Functions MUST exist.
  - Exchanges MUST normalize to canonical before emitting public events or models.
  - Regex standard: canonical symbol must match `^[A-Z0-9]+-[A-Z0-9]+$`.

12) CORE-ENUMS-PRESENT (MUST)
- Intent: Finalize enums.
- Scope: `core/`.
- Validation standard:
  - Types present and exported: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market` with exhaustive constant sets.

13) CORE-MAP-ENUMS-EXHAUSTIVE (MUST)
- Intent: Complete enum mappings with no fallthrough/default success.
- Scope: `exchanges/**` mapping functions (e.g., `mapOrderStatus`, `mapTimeInForce`).
- Validation standard:
  - In mapping functions, `switch` over source enum MUST enumerate all values of the destination enum without `default` returning a success value.

14) CORE-WS-DECODERS-TYPED (MUST)
- Intent: WS decoders must produce correct typed events.
- Scope: `exchanges/**/ws*.go`.
- Validation standard:
  - For each WS topic implemented, there MUST be a decoder function that returns one of the exact event types: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.

---

### CI & Testing

15) CI-BUILD-AND-TEST (MUST)
- Intent: Basic green build.
- Scope: CI pipeline.
- Validation standard:
  - Steps: `go build ./...` and `go test ./...` must pass with zero failures.

16) CI-INTEGRATION-TESTS (MUST)
- Intent: Run integration tests for all exchanges.
- Scope: CI pipeline.
- Validation standard:
  - For each exchange package under `exchanges/*`, run integration tests; failures block merge.

17) CI-LIVE-GATED (INFO)
- Intent: Optional live/testnet suites gated by env.
- Scope: CI pipeline.
- Validation standard:
  - Only execute live suites when credentials are present; otherwise skip cleanly.

---

### Exchange Development

18) EXCHANGE-FILE-STRUCTURE (SHOULD)
- Intent: Canonical file naming for exchange adapters.
- Scope: `exchanges/<name>/`.
- Validation standard:
  - Files: `<name>.go`, `exchange/provider.go`, `infra/rest/client.go`, `infra/ws/client.go`, `routing/rest_router.go`, `routing/ws_router.go`, `README.md`.

19) EXCHANGE-PROTOCOL-VERSION (MUST)
- Intent: Exchanges declare supported protocol version.
- Scope: `exchanges/**`.
- Validation standard:
  - Each exchange exposes `SupportedProtocolVersion() string` returning an exact string equal to `core.ProtocolVersion`.

---

### How to Implement These Checks

- AST-based checks: use standard Go tooling (`go vet`, `staticcheck`).
- CI rules: encode in your CI config (GitHub Actions, GitLab CI) to detect path changes and enforce version bump coupling.

Example commands in CI:

```bash
go build ./...
go test ./...
```

All MUST rules are intended to be machine-verifiable and block merges when violated.
