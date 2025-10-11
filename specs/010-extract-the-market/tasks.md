---
description: "Task list for Extract Market Data Routing into Universal Library (/lib/ws-routing)"
---

# Tasks: Extract Market Data Routing into Universal Library (/lib/ws-routing)

**Input**: Design documents from `/specs/010-extract-the-market/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Phase 1: Setup (Shared Infrastructure)

- [X] T001 Create library structure under lib/ws-routing: connection/, handler/, middleware/, router/, session/, telemetry/, internal/ (add doc.go in each)
- [X] T002 Add go package stubs: define the AbstractTransport interface in connection/transport.go and create telemetry/logger.go (vendor-neutral interface), session/session.go, router/router.go, handler/{registry.go,middleware.go}
- [X] T003 Update Makefile to run `go test ./... -race` locally and add a `bench` target (`go test -bench . -benchmem ./...`).

---

## Phase 2: Foundational (Blocking Prerequisites)

- [X] T004 Implement telemetry logging baseline in lib/ws-routing/telemetry/logger.go (Debug/Info/Warn/Error with fields)
- [X] T005 Extract session orchestration into lib/ws-routing/session/session.go (accept AbstractTransport; no network ownership)
- [X] T006 Extract routing engine into lib/ws-routing/router/router.go (per-symbol ordering; concurrency across symbols; backpressure=block upstream)
- [X] T007 Move handler registry and middleware: market_data/framework/handler/{registry.go,middleware.go} → lib/ws-routing/handler/ (neutral names)
- [X] T008 [PERF-07] Add router hot-path benchmarks in lib/ws-routing/router/bench_test.go with enforcement: zero allocations, median latency <500ns, P99 <2µs; run with -benchmem flag
- [X] T009 [FR-008][SC-010] Add domain-agnostic export validation: create test/script in tests/architecture/ that greps lib/ws-routing for domain terms (market, trade, ticker, etc.) in exported identifiers/docs; add linter rule if feasible; document manual review checklist

**⚠️ CRITICAL**: Complete Phase 2 before any user story work

---

## Phase 3: User Story 1 - Universal Routing Library Available (Priority: P1) 🎯 MVP

**Goal**: Provide a reusable, business-agnostic routing library under `/lib/ws-routing` with logging, ordering, and backpressure.

**Independent Test**: Initialize session with mock AbstractTransport; register middleware/handlers; verify per-symbol ordering and zero drops under slow handler.

- [X] T012 [P][US1] Unit tests for AbstractTransport mock in tests/unit/wsrouting/transport_mock_test.go
- [X] T013 [P][US1] Unit tests for telemetry logger in lib/ws-routing/telemetry/logger_test.go
- [X] T014 [US1] Ordering + backpressure tests in lib/ws-routing/router/router_ordering_test.go
- [X] T015 [TS-10][US1] Session lifecycle exhaustive tests in lib/ws-routing/session/session_lifecycle_test.go: cover negative paths (nil transport, double-start, subscribe before start, unsubscribe unknown topic, close idempotency, concurrent lifecycle calls)
- [X] T016 [TS-10][US1] Handler/middleware exhaustive tests in lib/ws-routing/handler/exhaustive_test.go: validate all handler types switch coverage, middleware chain edge cases (nil handler, empty chain, panic recovery), and explicit enum handling for all event types
- [X] T041 [US1][Contracts] Admin API contract tests in tests/contract/wsrouting/admin_api_contract_test.go aligned with contracts/admin-api.yaml

### Implementation for User Story 1

- [X] T017 [P][US1] Implement session lifecycle in lib/ws-routing/session/session.go (start/subscribe/publish/close)
- [X] T018 [P][US1] Implement router dispatch in lib/ws-routing/router/router.go (per-symbol order, parallel across symbols, blocking backpressure)
- [X] T019 [P][US1] Implement handler registry and middleware composition in lib/ws-routing/handler/
- [X] T020 [US1] Wire telemetry logging in session/router paths (no vendor SDKs)
- [X] T042 [US1] Implement admin API handlers in lib/ws-routing/api/ (health and subscriptions endpoints with structured logging)

**Checkpoint**: Library builds; tests pass for lib/ws-routing; T008 PERF-07 benchmarks meet baselines

---

## Phase 4: User Story 2 - Domain Adapters Updated (Priority: P2)

**Goal**: Update domain adapters and integration tests to consume `/lib/ws-routing`; remove legacy framework shim and directory.

**Independent Test**: Build succeeds with zero references to `market_data/framework`; integration tests in tests/integration/market_data continue to pass.

### Tests for User Story 2 (requested)

- [X] T021 [P][US2] Add adapter conformance helper in internal/testhelpers/wsrouting/conformance.go
- [X] T022 [US2] Update tests/integration/market_data/ws_routing_parity_test.go to use conformance helper and new imports
- [X] T023 [P][US2] Update tests/integration/market_data/smoke_test.go imports to lib/ws-routing

### Implementation for User Story 2

- [X] T024 [US2] Rename domain processors path: move `market_data/processors` → `exchanges/processors` and update all references/imports accordingly
- [X] T025 [US2] Repo-wide import update: replace `github.com/coachpo/meltica/market_data/framework` → `github.com/coachpo/meltica/lib/ws-routing`
- [X] T026 [US2] Delete market_data/framework/router/shim.go and related tests (helpers_test.go, acceptance_test.go, integration_test.go)
- [X] T027 [US2][GOV] Document the GOV-07 breaking change details in BREAKING_CHANGES_v2.md, communicate the import-path break to downstream consumers, and record second maintainer approval before merge
- [X] T028 [US2] Delete entire market_data/framework directory after migration; move/rename neutral files already ported
- [X] T029 [US2] Update go.mod/go.sum and run `go mod tidy` after removals
- [X] T030 [US2] Preserve benchmarks: move/adapt market_data/events_benchmark_test.go to reference lib/ws-routing where applicable, or add new bench in lib path

**Checkpoint**: No references to market_data/framework; integration tests pass

---

## Phase 5: User Story 3 - Documentation and Tooling Converge (Priority: P3)

**Goal**: Documentation and tooling reflect new package location; CI runs full test suite with race/bench.

**Independent Test**: Contributors can locate /lib/ws-routing in ARCHITECTURE.md/README.md; BREAKING_CHANGES_v2.md lists import breakage; `make test` runs `go test ./... -race` successfully.

### Implementation for User Story 3

- [X] T031 [US3] Update ARCHITECTURE.md with /lib/ws-routing boundaries and per-symbol ordering/backpressure
- [X] T032 [US3] Update README.md examples/imports to /lib/ws-routing
- [X] T033 [US3] Update BREAKING_CHANGES_v2.md with import path change and shim removal; fold any MIGRATION.md notes
- [X] T034 [US3] Ensure CI pipeline invokes the updated `make test` target (with `go test ./... -race`) and records results
- [X] T035 [US3] Repo-wide grep to confirm zero `market_data/framework` references remain
- [X] T036 [US3] Verify `MIGRATION.md` is absent after folding notes into BREAKING_CHANGES_v2.md; remove it only if still present
- [X] T037 [US3][SC-002] Run coverage verification (`go test ./... -cover`) and confirm thresholds: ≥80% overall, ≥70% per package; document results

**Checkpoint**: Docs/tooling reflect new library; CI green

---

## Phase N: Polish & Cross-Cutting Concerns

- [X] T038 [P] Run static analysis and architecture linter (`make lint-layers`); fix violations
- [X] T039 Optimize router hot path if needed to hit PERF-07 (profile and reduce allocations)
- [X] T040 [P] Final code cleanup, comments, and package docs

---

## Dependencies & Execution Order

- Setup (Phase 1) → Foundational (Phase 2) → US1 (P1) → US2 (P2) → US3 (P3) → Polish
- US2 depends on US1 completion; US3 depends on US1 and US2

## Parallel Opportunities

- [P] tasks within distinct packages: handler/, middleware/, telemetry/, api/ can proceed in parallel after T002–T007
- Test tasks T012, T013, T015 can run in parallel; implementation tasks T017–T019 can run in parallel
- US2 test updates T022, T024 can proceed in parallel

## Implementation Strategy

- MVP: Complete US1 (library availability with ordering/backpressure + minimal tests/benches)
- Incremental: US2 (adapters + integration), then US3 (docs/tooling); finalize with polish and perf
