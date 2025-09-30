### Protocol Validation Rules (v1.0)

This document defines deterministic validation rules derived from `ABSTRACTIONS_GUIDELINES.md`. Each rule lists an ID, intent, scope, and an objective validation standard describing exactly how to judge pass/fail.

- Severity:
  - MUST: failing blocks CI.
  - SHOULD: failing warns; may block in strict mode.
  - INFO: advisory.
- Unless noted, rules apply to production code (excluding `**/test/**`, `**/*_test.go`).
- Paths use a hypothetical Go-based layout: `core/`, `protocol/`, `providers/<name>/`, `conformance/`, `internal/meltilint/`, `cmd/xprovgen/`. Adjust globs if your repo differs.

---

### Phase 1–2: Core API, Models, Symbols, Decimals, Enums

1) CORE-IFACE-PROVIDER (MUST)
- Intent: Freeze `Provider` interface surface.
- Scope: `core/`.
- Validation standard:
  - Using Go AST, find interface `Provider` in `core` package.
  - Its method set MUST be exactly:
    - `Capabilities() ProviderCapabilities`
    - `Spot() SpotAPI`
    - `Futures() FuturesAPI`
    - `WS() WS`
  - No extra methods. Method names, parameter and result types (including pointer/value, exported names, package qualifiers) MUST match exactly.

2) CORE-IFACE-SPOT (MUST)
- Intent: Freeze `SpotAPI` surface.
- Scope: `core/`.
- Validation standard:
  - Go AST must contain `type SpotAPI interface { ... }` with method names and signatures exactly matching the normative spec file `core/spot.go` (reference file of truth in repo). Validation compares a normalized signature hash of method set to the golden file.

3) CORE-IFACE-FUTURES (MUST)
- Intent: Freeze `FuturesAPI` surface.
- Scope: `core/`.
- Validation standard:
  - Same approach as CORE-IFACE-SPOT, comparing against `core/futures.go` golden.

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
- Scope: `core/errs` (or `errs` package in `core`).
- Validation standard:
  - Type `E` MUST be declared as a named type in package `errs` and used by exported constructors.
  - Canonical code enum/constants declared and exported (e.g., `Code` type with constants).
  - Public APIs in `core/` MUST return `*errs.E` for error values (AST type check on function results), not wrapped opaque errors.

8) CORE-CAPABILITIES-BITSET (MUST)
- Intent: Freeze capabilities presence and type.
- Scope: `core/`.
- Validation standard:
  - Type `ProviderCapabilities` exists and is a bitset-friendly integer type (`uint64` or alias). Exported bit constants exist for each capability mentioned in the spec.
  - `Provider.Capabilities()` method result type MUST be `ProviderCapabilities`.

9) CORE-NO-FLOATS (MUST)
- Intent: Enforce decimal policy in critical paths.
- Scope: `core/`, `providers/**` (production code only).
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
- Intent: Serialization uses `core.FormatDecimal`.
- Scope: `core/` JSON marshaling code and `providers/**` adapters' serialization helpers.
- Validation standard:
  - Any `MarshalJSON` involving `*big.Rat` MUST call `core.FormatDecimal` for string output. Static analysis: within `MarshalJSON`, if a `*big.Rat` field is serialized, search for a call to `core.FormatDecimal` in the same function; absence is a fail.

12) CORE-SYMBOL-CANONICAL (MUST)
- Intent: Enforce canonical symbol form `BASE-QUOTE` uppercase.
- Scope: `core/symbols.go`, `providers/**`.
- Validation standard:
  - `core` exposes helpers (e.g., `ToCanonicalSymbol`, `ParseSymbol`). Functions MUST exist.
  - Providers MUST normalize to canonical before emitting public events or models. Heuristic static check: in providers, before constructing public `Instrument` or symbol-bearing events, presence of a call to `core.ToCanonicalSymbol` (or equivalent helper) is required.
  - Regex standard: canonical symbol must match `^[A-Z0-9]+-[A-Z0-9]+$`.

13) CORE-ENUMS-PRESENT (MUST)
- Intent: Finalize enums.
- Scope: `core/`.
- Validation standard:
  - Types present and exported: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market` with exhaustive constant sets matching golden values. Validate by enumerating iota/const sets and comparing names against a golden allowlist.

14) CORE-MAP-ENUMS-EXHAUSTIVE (MUST)
- Intent: Complete enum mappings with no fallthrough/default success.
- Scope: `providers/**` mapping functions (e.g., `mapOrderStatus`, `mapTimeInForce`).
- Validation standard:
  - In mapping functions, `switch` over source enum MUST enumerate all values of the destination enum without `default` returning a success value. If `default` exists, it MUST return an error of type `*errs.E` with canonical code (see rule CORE-ERRORS-TYPE). Missing cases fail the check.

15) CORE-WS-DECODERS-TYPED (MUST)
- Intent: WS decoders must produce correct typed events.
- Scope: `providers/**/ws*.go`.
- Validation standard:
  - For each WS topic implemented, there MUST be a decoder function that returns one of the exact event types: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.
  - Static check: exported decoder functions named with suffix `ToEvent` or inside `ws` files must return a concrete `core.*Event` type, not `interface{}` or `map[string]any`.

---

### Phase 3: Protocol Docs and JSON Schemas

16) PROTO-DOCS-PRESENT (SHOULD)
- Intent: Human-readable protocol docs exist.
- Scope: `protocol/`.
- Validation standard:
  - Files present: `protocol/README.md` or `protocol/CONTRACT.md` documenting symbols, decimals, orders, WS topics, error normalization (presence check by headings).

17) PROTO-SCHEMAS-COMPLETE (MUST)
- Intent: JSON Schemas exist for all models and WS events.
- Scope: `protocol/schemas/`.
- Validation standard:
  - Schema files exist for: models (`Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline`) and events (`TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`). Naming convention: `<Type>.schema.json`.
  - Each schema MUST be valid Draft 2020-12 (detected via `$schema`).

18) PROTO-VECTORS-VALIDATE (MUST)
- Intent: Golden vectors validate against schemas.
- Scope: `protocol/vectors/**.json`.
- Validation standard:
  - For every schema `<Type>.schema.json`, there exists at least one vector file `protocol/vectors/<Type>*.json`.
  - CI must validate vectors with a JSON Schema validator; zero errors permitted.

---

### Phase 4: Conformance Kit

19) CONF-HARNESS-ENTRY (MUST)
- Intent: Provide standard entry point.
- Scope: `conformance/`.
- Validation standard:
  - Function `RunAll(t *testing.T, factory ProviderFactory, opts Options)` exported in package `conformance` (AST check by exact signature).

20) CONF-SUITES-OFFLINE (MUST)
- Intent: Include offline suites.
- Scope: `conformance/`.
- Validation standard:
  - Test files exist for: JSON mapping, enums, decimals, error mapping, WS public/private decoding.
  - `go test ./conformance/...` MUST pass with no failures.

21) CONF-CAPABILITY-GATING (MUST)
- Intent: Tests respect capabilities.
- Scope: `conformance/` tests.
- Validation standard:
  - Tests reference `Provider.Capabilities()` and skip subtests when capability bits indicate unsupported features. Presence check: look for bit tests on `ProviderCapabilities` in test sources.

---

### Phase 5: Adapter Scaffolding Tool

22) SCAFF-CMD-PRESENT (SHOULD)
- Intent: Generator exists.
- Scope: `cmd/xprovgen/`.
- Validation standard:
  - A `main.go` exists that writes files into `providers/<name>/` when invoked with `--name`.

23) SCAFF-FILES-GENERATED (SHOULD)
- Intent: Minimal file set is generated.
- Scope: `providers/<name>/` after generation.
- Validation standard:
  - Files present: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go`, `README.md`, `conformance_test.go`, `golden_test.go`.

---

### Phase 6: Static Checks and Linting (Protolint)

24) LINT-CAPABILITIES-CONSISTENT (MUST)
- Intent: `Capabilities()` consistent with implemented APIs.
- Scope: `providers/**`.
- Validation standard:
  - If provider implements methods that require a capability (e.g., create order), the corresponding bit MUST be set by `Capabilities()`.
  - Static heuristic: presence of exported methods on adapter structs mapped to core APIs implies required bits. Missing bits fail.

25) LINT-ENUM-MAPPING-COMPLETE (MUST)
- Intent: No missing enum mappings.
- Scope: `providers/**`.
- Validation standard:
  - For each mapping function, verify `switch` cases cover all destination enums (from core). No unhandled values.

26) LINT-NO-FLOATS-CRITICAL (MUST)
- Intent: No `float32/float64` in critical paths.
- Scope: `core/`, `providers/**` (excl. tests).
- Validation standard:
  - Fails if any exported field or API uses floats (see rule CORE-NO-FLOATS for details).

27) LINT-WS-DECODER-TARGET (MUST)
- Intent: WS decoders return correct events.
- Scope: `providers/**/ws*.go`.
- Validation standard:
  - Decoder functions’ return types MUST be concrete `core.*Event`.

28) LINT-ERRORS-CANONICAL (MUST)
- Intent: Errors use `*errs.E` with provider and canonical code.
- Scope: `providers/**`.
- Validation standard:
  - Functions returning `error` must, on non-nil returns, construct or wrap a `*errs.E` that includes the `Provider` identifier and a canonical `Code`. Detect by type assertion in code or helper constructors usage.

---

### Phase 7: CI Gates and Matrices

29) CI-BUILD-AND-TEST (MUST)
- Intent: Basic green build.
- Scope: CI pipeline.
- Validation standard:
  - Steps: `go build ./...` and `go test ./...` must pass with zero failures.

30) CI-CONFORMANCE-OFFLINE (MUST)
- Intent: Run offline conformance for all adapters.
- Scope: CI pipeline.
- Validation standard:
  - For each provider package under `providers/*`, run offline conformance via `conformance.RunAll` with capability gating; failures block merge.

31) CI-LIVE-GATED (INFO)
- Intent: Optional live/testnet suites gated by env.
- Scope: CI pipeline.
- Validation standard:
  - Only execute live suites when `MELTICA_CONFORMANCE=1` and credentials are present; otherwise skip cleanly.

32) CI-PROTOCOL-VERSION-GUARD (MUST)
- Intent: Block protocol changes without version bump.
- Scope: PR diffs.
- Validation standard:
  - If any files under `core/`, `protocol/schemas/`, or `protocol/vectors/` change on a PR, `protocol/version.go` MUST also change by increasing `ProtocolVersion` (SemVer). Otherwise fail.

33) CI-CAPABILITY-MISMATCH (MUST)
- Intent: Fail when claimed capabilities don’t pass suites.
- Scope: CI results + provider `Capabilities()`.
- Validation standard:
  - For each capability bit set by a provider, all tests tagged for that capability must pass. If any test for a claimed capability fails, block merge.

---

### Phase 8: Versioning and Change Control

34) VER-FILE-PRESENT (MUST)
- Intent: Presence of version constant.
- Scope: `protocol/version.go`.
- Validation standard:
  - File defines `const ProtocolVersion = "1.0"` (or higher). AST check on constant.

35) VER-SEMVER-FORMAT (MUST)
- Intent: SemVer compliance.
- Scope: `protocol/version.go`.
- Validation standard:
  - `ProtocolVersion` MUST match regex: `^\d+\.\d+(\.\d+)?(-[0-9A-Za-z.-]+)?$`.

36) VER-BREAKING-REQUIRES-UPDATES (MUST)
- Intent: On breaking changes, require docs/schemas/vectors updates.
- Scope: PR diffs.
- Validation standard:
  - If `ProtocolVersion` major/minor is incremented, PR MUST also change at least one file under each of: `protocol/` docs, `protocol/schemas/`, and `protocol/vectors/`.

---

### Phase 9: Provider Development Guide

37) DOCS-PROVIDER-GUIDE (SHOULD)
- Intent: Onboarding doc present.
- Scope: `docs/implementing-provider.md`.
- Validation standard:
  - File exists and contains required sections (heading presence): Minimal file set, API checklist, error/status mapping, WS decoding examples, offline conformance, enabling live tests.

---

### Phase 10: Rollout and Backcompat

38) REL-ADAPTER-DECLARES-VERSION (MUST)
- Intent: Adapters declare supported protocol version.
- Scope: `providers/**`.
- Validation standard:
  - Each provider exposes `SupportedProtocolVersion() string` returning an exact string equal to `protocol.ProtocolVersion` or a declared compatible version range. Fails if mismatched.

39) REL-TAGS-REQUIRED (INFO)
- Intent: Releases are tagged.
- Scope: CI/CD.
- Validation standard:
  - On release workflow, require annotated tag presence that matches `ProtocolVersion` and adapter versions.

---

### Cross-cutting Conventions

40) CODEGEN-NO-DRIFT (SHOULD)
- Intent: Generated code kept in sync.
- Scope: Any generated files with `// Code generated` header.
- Validation standard:
  - CI step re-runs generators; diff must be clean.

41) FILE-NAMING-CANONICAL (SHOULD)
- Intent: Canonical file naming for schemas and vectors.
- Scope: `protocol/schemas/`, `protocol/vectors/`.
- Validation standard:
  - Schemas: `<Type>.schema.json`; Vectors: `<Type>.<case>.json`.

42) PACKAGE-NAMES-CANONICAL (SHOULD)
- Intent: Idiomatic Go package names.
- Scope: Go code.
- Validation standard:
  - Lowercase, no underscores; linter enforces.

---

### How to Implement These Checks

- AST-based checks: build a small linter under `internal/meltilint` using `go/ast` and `go/types`.
- Schema validation: use `ajv` or `gojsonschema` in CI to verify `protocol/vectors` against `protocol/schemas`.
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
go test ./conformance -run TestOffline
node scripts/validate-schemas.js # or go run internal/tools/validate_schemas
```

All MUST rules are intended to be machine-verifiable and block merges when violated.


