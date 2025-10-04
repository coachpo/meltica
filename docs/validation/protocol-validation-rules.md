### Protocol Validation Rules (v1.0)

This document defines deterministic validation rules derived from `ABSTRACTIONS_GUIDELINES.md`. Each rule lists an ID, intent, scope, and an objective validation standard describing exactly how to judge pass/fail.

- Severity:
  - MUST: failing blocks CI.
  - SHOULD: failing warns; may block in strict mode.
  - INFO: advisory.
- Unless noted, rules apply to production code (excluding `**/test/**`, `**/*_test.go`).
- Paths use the actual Go-based layout: `core/`, `exchanges/<name>/`, `transport/`, `config/`, `errs/`, `cmd/`. Adjust globs if your repo differs.

---

### Phase 1–2: Core API, Models, Symbols, Decimals, Enums

1) CORE-IFACE-EXCHANGE (MUST)
- Intent: Freeze `Exchange` interface surface.
- Scope: `core/`.
- Validation standard:
  - Using Go AST, find interface `Exchange` in `core` package.
  - Its method set MUST be exactly:
    - `Name() string`
    - `Capabilities() ExchangeCapabilities`
    - `SupportedProtocolVersion() string`
    - `Spot(ctx context.Context) SpotAPI`
    - `LinearFutures(ctx context.Context) FuturesAPI`
    - `InverseFutures(ctx context.Context) FuturesAPI`
    - `WS() WS`
    - `Close() error`
  - No extra methods. Method names, parameter and result types (including pointer/value, exported names, package qualifiers) MUST match exactly.

2) CORE-IFACE-SPOT (MUST)
- Intent: Freeze `SpotAPI` surface.
- Scope: `core/`.
- Validation standard:
  - Go AST must contain `type SpotAPI interface { ... }` with method names and signatures exactly matching the normative spec file `core/core.go` (reference file of truth in repo). Validation compares a normalized signature hash of method set to the golden file.

3) CORE-IFACE-FUTURES (MUST)
- Intent: Freeze `FuturesAPI` surface.
- Scope: `core/`.
- Validation standard:
  - Same approach as CORE-IFACE-SPOT, comparing against `core/core.go` golden.

4) CORE-IFACE-WS (MUST)
- Intent: Freeze `WS` and `Subscription` surfaces.
- Scope: `core/`.
- Validation standard:
  - Go AST must contain `type WS interface { ... }` and `type Subscription interface { ... }` matching golden signatures. No extra/removed methods.

5) CORE-MODELS-PRESENT (MUST)
- Intent: Freeze model types and doc comments as spec.
- Scope: `core/`.
- Validation standard:
  - Types MUST exist: `Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline`.
  - Each type declaration must be preceded by a non-empty doc comment (AST `Doc` present).

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
  - Canonical code enum/constants declared and exported (e.g., `Code` type with constants).
  - Public APIs in `core/` MUST return `*errs.E` for error values (AST type check on function results), not wrapped opaque errors.

8) CORE-CAPABILITIES-BITSET (MUST)
- Intent: Freeze capabilities presence and type.
- Scope: `core/`.
- Validation standard:
  - Type `ExchangeCapabilities` exists and is a bitset-friendly integer type (`uint64` or alias). Exported bit constants exist for each capability mentioned in the spec.
  - `Exchange.Capabilities()` method result type MUST be `ExchangeCapabilities`.

9) CORE-NO-FLOATS (MUST)
- Intent: Enforce decimal policy in critical paths.
- Scope: `core/`, `exchanges/**` (production code only).
- Validation standard:
  - Disallow identifiers of type `float32` or `float64` in:
    - Public structs (exported) fields.
    - Public function/method parameters and return types.
    - Marshal/Unmarshal implementations (`MarshalJSON`, `UnmarshalJSON`).
  - Allowed in tests and local variables inside non-exported functions only if not used in serialization.

10) CORE-DECIMAL-BIGRAT (MUST)
- Intent: Numbers are represented by `*big.Rat`.
- Scope: `core/` models and any serialized representation.
- Validation standard:
  - All numeric price/size/amount/qty fields in the model types MUST have type `*big.Rat` (AST field type check by name match: `Price`, `Size`, `Amount`, `Qty`, `AveragePrice`, etc., per golden allowlist).

11) CORE-FORMATDECIMAL-USE (MUST)
- Intent: Serialization uses `numeric.Format`.
- Scope: `core/` JSON marshaling code and `exchanges/**` adapters' serialization helpers.
- Validation standard:
  - Any `MarshalJSON` involving `*big.Rat` MUST call `numeric.Format` for string output. Static analysis: within `MarshalJSON`, if a `*big.Rat` field is serialized, search for a call to `numeric.Format` in the same function; absence is a fail.

12) CORE-SYMBOL-CANONICAL (MUST)
- Intent: Enforce canonical symbol form `BASE-QUOTE` uppercase.
- Scope: `core/symbols.go`, `exchanges/**`.
- Validation standard:
  - `core` exposes helpers (e.g., `ToCanonicalSymbol`, `ParseSymbol`). Functions MUST exist.
  - Exchanges MUST normalize to canonical before emitting public events or models. Heuristic static check: in exchanges, before constructing public `Instrument` or symbol-bearing events, presence of a call to `core.ToCanonicalSymbol` (or equivalent helper) is required.
  - Regex standard: canonical symbol must match `^[A-Z0-9]+-[A-Z0-9]+$`.

13) CORE-ENUMS-PRESENT (MUST)
- Intent: Finalize enums.
- Scope: `core/`.
- Validation standard:
  - Types present and exported: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market` with exhaustive constant sets matching golden values. Validate by enumerating iota/const sets and comparing names against a golden allowlist.

14) CORE-MAP-ENUMS-EXHAUSTIVE (MUST)
- Intent: Complete enum mappings with no fallthrough/default success.
- Scope: `exchanges/**` mapping functions (e.g., `mapOrderStatus`, `mapTimeInForce`).
- Validation standard:
  - In mapping functions, `switch` over source enum MUST enumerate all values of the destination enum without `default` returning a success value. If `default` exists, it MUST return an error of type `*errs.E` with canonical code (see rule CORE-ERRORS-TYPE). Missing cases fail the check.

15) CORE-WS-DECODERS-TYPED (MUST)
- Intent: WS decoders must produce correct typed events.
- Scope: `exchanges/**/ws*.go`.
- Validation standard:
  - For each WS topic implemented, there MUST be a decoder function that returns one of the exact event types: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.
  - Static check: exported decoder functions named with suffix `ToEvent` or inside `ws` files must return a concrete `core.*Event` type, not `interface{}` or `map[string]any`.

---

### Phase 7: CI Gates and Matrices

16) CI-BUILD-AND-TEST (MUST)
- Intent: Basic green build.
- Scope: CI pipeline.
- Validation standard:
  - Steps: `go build ./...` and `go test ./...` must pass with zero failures.

17) CI-INTEGRATION-TESTS (MUST)
- Intent: Run integration tests for all exchanges.
- Scope: CI pipeline.
- Validation standard:
  - For each exchange package under `exchanges/*`, run integration tests; failures block merge.

18) CI-LIVE-GATED (INFO)
- Intent: Optional live/testnet suites gated by env.
- Scope: CI pipeline.
- Validation standard:
  - Only execute live suites when credentials are present; otherwise skip cleanly.

19) CI-PROTOCOL-VERSION-GUARD (MUST)
- Intent: Block protocol changes without version bump.
- Scope: PR diffs.
- Validation standard:
  - If any files under `core/` change on a PR, `core/core.go` MUST also change by increasing `ProtocolVersion` (SemVer). Otherwise fail.

20) CI-CAPABILITY-MISMATCH (MUST)
- Intent: Fail when claimed capabilities don't pass suites.
- Scope: CI results + exchange `Capabilities()`.
- Validation standard:
  - For each capability bit set by an exchange, all tests tagged for that capability must pass. If any test for a claimed capability fails, block merge.

---

### Phase 8: Versioning and Change Control

21) VER-FILE-PRESENT (MUST)
- Intent: Presence of version constant.
- Scope: `core/core.go`.
- Validation standard:
  - File defines `const ProtocolVersion = "1.2.0"` (or higher). AST check on constant.

22) VER-SEMVER-FORMAT (MUST)
- Intent: SemVer compliance.
- Scope: `core/core.go`.
- Validation standard:
  - `ProtocolVersion` MUST match regex: `^\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$`.

23) VER-BREAKING-REQUIRES-UPDATES (MUST)
- Intent: On breaking changes, require docs updates.
- Scope: PR diffs.
- Validation standard:
  - If `ProtocolVersion` major/minor is incremented, PR MUST also change at least one file under `docs/`.

---

### Phase 9: Exchange Development Guide

24) DOCS-EXCHANGE-GUIDE (SHOULD)
- Intent: Onboarding doc present.
- Scope: `docs/implementing-a-provider.md`.
- Validation standard:
  - File exists and contains required sections (heading presence): Minimal file set, API checklist, error/status mapping, WS decoding examples, integration testing, enabling live tests.

---

### Phase 10: Rollout and Backcompat

25) REL-ADAPTER-DECLARES-VERSION (MUST)
- Intent: Exchanges declare supported protocol version.
- Scope: `exchanges/**`.
- Validation standard:
  - Each exchange exposes `SupportedProtocolVersion() string` returning an exact string equal to `core.ProtocolVersion` or a declared compatible version range. Fails if mismatched.

26) REL-TAGS-REQUIRED (INFO)
- Intent: Releases are tagged.
- Scope: CI/CD.
- Validation standard:
  - On release workflow, require annotated tag presence that matches `ProtocolVersion` and exchange versions.

---

### Cross-cutting Conventions

27) FILE-NAMING-CANONICAL (SHOULD)
- Intent: Canonical file naming for exchange adapters.
- Scope: `exchanges/<name>/`.
- Validation standard:
  - Files: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go`, `README.md`.

28) PACKAGE-NAMES-CANONICAL (SHOULD)
- Intent: Idiomatic Go package names.
- Scope: Go code.
- Validation standard:
  - Lowercase, no underscores; linter enforces.

---

### How to Implement These Checks

- AST-based checks: use standard Go tooling (`go vet`, `staticcheck`).
- CI rules: encode in your CI config (GitHub Actions, GitLab CI) to detect path changes and enforce version bump coupling.

Example regexes:

```regex
CANONICAL_SYMBOL := ^[A-Z0-9]+-[A-Z0-9]+$
SEMVER := ^\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$
```

Example commands in CI:

```bash
go build ./...
go test ./...
```

All MUST rules are intended to be machine-verifiable and block merges when violated.
