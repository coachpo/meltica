### Phase 1: Freeze the Protocol Surface (Core)
- Steps
  - Document and finalize `core` interfaces: `Provider`, `SpotAPI`, `FuturesAPI`, `WS`, `Subscription`.
  - Lock domain models: `Instrument`, `OrderRequest`, `Order`, `Position`, `Ticker`, `OrderBook`, `Trade`, `Kline`.
  - Lock WS events: `TradeEvent`, `TickerEvent`, `DepthEvent`, `OrderEvent`, `BalanceEvent`.
  - Lock error type and codes in `errs.E` and mapping rules.
  - Add `ProviderCapabilities` to `Provider` and define capability bitset constants.
- Outcomes
  - Stable API in `core/` with doc-comments as normative spec.
  - `Provider.Capabilities()` returns coherent capability info.
  - All models/events and error codes are frozen for v1.0.

### Phase 2: Symbols, Decimals, Enums (Normalization Rules)
- Steps
  - Codify canonical symbol `BTC-USDT`; keep adapters’ conversion helpers in `core/symbols.go`.
  - Enforce decimal policy: numbers use `*big.Rat`; serialization with `core.FormatDecimal`.
  - Finalize enums: `OrderSide`, `OrderType`, `TimeInForce`, `OrderStatus`, `Market`.
- Outcomes
  - One canonical symbol format across providers.
  - Zero floats in serialization paths; consistent precision.
  - Shared enums with exact mapping requirements.

### Phase 3: Protocol Docs and JSON Schemas
- Steps
  - Create `protocol/` with:
    - Contract docs: symbols, decimals, orders, WS topics, error normalization.
    - JSON Schemas in `protocol/schemas/` for models and WS events.
    - Golden vectors in `protocol/vectors/` (JSON examples per schema).
- Outcomes
  - Machine- and human-readable protocol spec.
  - Canonical examples for adapters to validate against.

### Phase 4: Conformance Kit
- Steps
  - Add `conformance/` harness:
    - Offline unit suites: JSON mapping, enums, decimals, error mapping.
    - WS public/private decoding to typed events.
    - Capability-aware gating and clear skips.
  - Expose a `RunAll(t, factory, opts)` entry.
- Outcomes
  - One command to validate any adapter against the protocol.
  - Deterministic pass/fail with actionable messages.

### Phase 5: Adapter Scaffolding Tool
- Steps
  - Implement `cmd/xprovgen` to generate `providers/<name>/`:
    - Files: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go`, `README.md`, `conformance_test.go`, `golden_test.go`.
    - Stubs for symbol conversion, error/status mapping, `instCache`, `Capabilities()`, `Register()`.
- Outcomes
  - New providers bootstrapped consistently within minutes.
  - Ready-to-run conformance tests from day one.

### Phase 6: Static Checks and Linting
- Steps
  - Add `internal/protolint` with checks:
    - `Capabilities()` present and consistent with implemented APIs.
    - Complete enum mappings (`OrderStatus`, `TimeInForce`) with no fallthrough.
    - No `float32/float64` usage in critical paths.
    - WS decoders produce the correct typed events.
    - Errors return `*errs.E` with `Provider` and canonical `Code`.
  - Integrate with `golangci-lint`.
- Outcomes
  - Protocol compliance enforced at compile/lint time.
  - Fewer regressions and review burden.

### Phase 7: CI Gates and Matrices
- Steps
  - CI jobs:
    - `go build ./...`, unit + golden tests.
    - Conformance offline suite for all adapters.
    - Optional live/testnet suites gated by env (e.g., `MELTICA_CONFORMANCE=1` + creds).
  - PR rules:
    - Fail if protocol files change without protocol version bump.
    - Fail if claimed capabilities don’t pass respective suites.
- Outcomes
  - Automated enforcement of the protocol for every change.
  - Safe evolution with clear red/green signals.

### Phase 8: Versioning and Change Control
- Steps
  - Add `protocol/version.go` with `ProtocolVersion = "1.0"`.
  - SemVer for protocol and adapters.
  - On breaking changes:
    - Update docs/schemas/vectors.
    - Add migration notes.
    - Bump `ProtocolVersion` and require adapters to declare support.
- Outcomes
  - Predictable upgrades and compatibility signaling.
  - Consumers can pin protocol versions confidently.

### Phase 9: Provider Development Guide
- Steps
  - Author `docs/implementing-provider.md`:
    - Minimal file set and API checklist.
    - Examples for error/status mapping and WS decoding.
    - How to pass offline conformance; how to enable optional live tests.
- Outcomes
  - Clear, repeatable onboarding for new adapters.
  - Reduced support load and consistent quality bar.

### Phase 10: Rollout and Backcompat Plan
- Steps
  - Tag protocol v1.0 and adapters updated to declare support.
  - Run conformance on Binance/OKX to baseline green.
  - Announce freeze; provide migration guidance for consumers.
- Outcomes
  - Protocol v1.0 published with at least two passing reference adapters.
  - Stable platform for future adapters (e.g., Bybit, Kraken).

### Acceptance criteria per phase
- Phase 1–2: `core/` types/interfaces frozen; adapters compile without changes; enums/scales documented.
- Phase 3: Schemas validate current payloads; vectors cover all event/model variants.
- Phase 4: Conformance suite fails meaningfully on deliberate violations; both Binance/OKX pass.
- Phase 5: `xprovgen` generates an adapter that passes offline suites with minimal edits.
- Phase 6: Lint fails on protocol violations; no false positives on current adapters.
- Phase 7: CI blocks protocol changes without version bump; capability mismatches fail.
- Phase 8: Version bump process proven with a simulated breaking change.
- Phase 9: A new engineer can implement a toy provider and pass offline conformance in a day.
- Phase 10: Tagged release; green badges for Binance/OKX; migration notes published.

- In short: you’ll have a locked protocol, schemas, conformance tests, scaffolding, and CI/lint guardrails ensuring every new adapter adheres to the universal API with predictable outcomes.