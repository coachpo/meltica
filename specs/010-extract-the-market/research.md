# Phase 0 Research: Extract Market Data Routing into Universal Library (/lib/ws-routing)

## Decisions

1) Abstract Transport (L1 vs L2 boundary)
- Decision: Library consumes an abstract transport interface; it does not own network connections.
- Rationale: Preserves layer boundaries (CQ‑02); simplifies reuse and testing.
- Alternatives: Own the WS connection (more invasive, violates boundary); hybrid helper (adds surface without need).

2) Event/Message Schemas
- Decision: Preserve existing schemas unchanged.
- Rationale: Minimizes migration risk; preserves contract tests; aligns with spec success criteria.
- Alternatives: Neutral renames (introduces churn); redesign shapes (higher risk without benefit).

3) Observability Baseline
- Decision: Structured logging baseline via vendor‑neutral interface in `/lib/ws-routing/telemetry`.
- Rationale: Constitution CQ‑09; keep domain and vendor decoupled.
- Alternatives: Metrics/tracing built‑in (scope creep; optional via extension points).

4) Ordering & Concurrency
- Decision: Preserve per‑symbol ordering; allow parallelism across symbols.
- Rationale: Meets trading expectations; balances latency and correctness.
- Alternatives: Global order (unnecessary), no order (breaks consumers).

5) Backpressure Policy
- Decision: Block upstream (no drops); preserve per‑symbol order.
- Rationale: Lossless guarantees favored; easier reasoning under load.
- Alternatives: Drop oldest/newest (data loss), fail fast (destabilizes runtimes).

6) Admin HTTP Endpoints Scope
- Decision: Provide minimal read‑only endpoints in `lib/ws-routing/api`: GET `/admin/ws-routing/v1/health`, GET `/admin/ws-routing/v1/subscriptions`.
- Rationale: Operational visibility without domain coupling.
- Alternatives: Write operations (pause/reload) deferred to future phases.

7) Deprecation Shim Removal
- Decision: Remove `market_data/framework/router/shim.go` immediately.
- Rationale: Spec explicitly authorizes breaking import paths; narrow consumer surface.
- Alternatives: One‑release shim (GOV‑07) increases maintenance; exception documented and to be approved.

8) Performance Baselines
- Decision: Enforce PERF‑07 baselines with `-benchmem`; preserve current benchmarks under new paths.
- Rationale: Prevent regressions in hot routes.

9) Test Strategy
- Decision: Move unit tests to new packages; keep integration under `tests/integration/market_data` and update imports; add adapter conformance helper under `internal/testhelpers`.
- Rationale: TS‑10 boundary coverage; maintain parity and reduce duplication.

## Open Questions (resolved in this plan)
- Admin authentication: NONE (local dev/CI only); production deployments wire auth at composition root.
- Metrics/tracing: OPTIONAL via extension points; not baseline.
