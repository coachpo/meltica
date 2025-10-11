# Implementation Plan: Extract Market Data Routing into Universal Library (/lib/ws-routing)

**Branch**: `010-extract-the-market` | **Date**: 2025-10-11 | **Spec**: specs/010-extract-the-market/spec.md
**Input**: Feature specification from `/specs/010-extract-the-market/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Extract the market data routing framework into a business‑agnostic library under `/lib/ws-routing` with packages: `session`, `connection` (abstract transport interfaces only), `router`, `handler`, `middleware`, `telemetry` (logging baseline), and `api` (admin HTTP endpoints). Update all call sites to import the new paths, remove `market_data/framework/router/shim.go` and the entire `market_data/framework` directory, and preserve event schemas, per‑symbol ordering, and hot‑path performance baselines. Provide structured logging interfaces (vendor‑neutral), backpressure via upstream blocking (no drops), adapter conformance test helper, coverage parity, and preserved benchmarks targeting new lib paths.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25  
**Primary Dependencies**: stdlib + vendor‑neutral logging interface in `/lib/ws-routing/telemetry`; gorilla/websocket only at L1 transport (external to this lib); goccy/go-json used by callers; internal `errs` package for typed errors  
**Storage**: N/A  
**Testing**: `go test ./... -race`; integration tests behind `//go:build integration`; coverage ≥80% repo, ≥70% per package; boundary tests exhaustive (TS‑10)  
**Target Platform**: macOS/Linux dev, Linux CI  
**Project Type**: Single repository, library under `/lib/ws-routing`  
**Performance Goals**: Routing dispatch zero allocations; median < 500ns, P99 < 2µs per event (PERF‑07)  
**Constraints**: Domain‑agnostic code under `/lib`; structured logging baseline; stable package names; per‑symbol ordering; upstream backpressure (no drops)  
**Scale/Scope**: 100+ concurrent symbols per connection; adapters across market_data and future domains

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

[Gates determined based on constitution file]
- Confirm the proposed work prioritizes new capabilities/architectural improvements over backward compatibility, and explicitly flag breaking changes in the spec with migration impacts documented.
- Verify reusable infrastructure remains under `/lib` and that `/lib/**` code does not import domain packages.
- If refactors change import paths, include migration notes and a one-release deprecation shim in the rollout plan (GOV-08).
- If touching routing or boundary contracts, define PERF-07 routing hot-path benchmarks and add boundary test plans (TS-10).
- Ensure observability uses `/lib` logging/metrics interfaces; no vendor SDKs in domain or routing code.

Result:
- Innovation over compatibility: Explicitly breaking import paths; spec calls it out; event schemas unchanged. ✓
- `/lib` residency and no domain imports in `/lib/**`: enforced by structure and linter. ✓
- Deprecation shim (GOV‑07/GOV‑08): Exception requested to remove shim per spec; justification: immediate consolidation, low external surface, documented in BREAKING_CHANGES_v2.md with version bump. Approval required by 2 maintainers. ⚠ Exception
- PERF‑07 baselines and TS‑10 boundary tests: included; benchmarks preserved and enforced. ✓
- Observability via vendor‑neutral logging interface only: ✓

## Project Structure

### Documentation (this feature)

```
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```
lib/
└── ws-routing/
    ├── api/                 # admin HTTP endpoints (health, subscriptions)
    ├── connection/          # abstract transport interfaces (no network ownership)
    ├── handler/             # handler contracts and registry helpers
    ├── middleware/          # middleware contracts and chain helpers
    ├── router/              # routing engine and dispatch/concurrency
    ├── session/             # session lifecycle (with abstract transport)
    ├── telemetry/           # structured logging interfaces (vendor-neutral)
    └── internal/...

market_data/
└── processors/              # domain-specific processors (unchanged)

tests/
├── integration/market_data/ # updated to import lib/ws-routing
└── unit/                    # moved/adapted unit tests for new lib paths
```

**Structure Decision**: Single repository with a universal library under `/lib/ws-routing`, domain adapters in `market_data/**`, and tests relocated to reference the new library. Admin HTTP endpoints reside in `lib/ws-routing/api`.

## Complexity Tracking

*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| GOV‑07/08 shim removal | Spec mandates immediate removal of shim; controlled downstream, documented break with version bump | Maintaining shim prolongs dual paths, increases risk and maintenance cost with limited benefit in our ecosystem |
