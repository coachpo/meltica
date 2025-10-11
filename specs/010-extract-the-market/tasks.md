---
description: "Task list for Extract Market Data Routing into Universal Library (/lib/ws-routing)"
---

# Tasks: Extract Market Data Routing into Universal Library (/lib/ws-routing)

**Input**: Design documents from `/specs/010-extract-the-market/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md, contracts/

## Phase 1: Setup (Shared Infrastructure)

- [ ] T001 Create library structure under lib/ws-routing: api/, connection/, handler/, middleware/, router/, session/, telemetry/, internal/ (add doc.go in each)
- [ ] T002 Add go package stubs: connection/transport.go (AbstractTransport interface), telemetry/logger.go (vendor-neutral interface), session/session.go, router/router.go, handler/{registry.go,middleware.go}, api/routes.go
- [ ] T003 Update Makefile: ensure `test` target runs `go test ./... -race`; add `bench` target `go test -bench . -benchmem ./...`

---

## Phase 2: Foundational (Blocking Prerequisites)

- [ ] T004 Define AbstractTransport in lib/ws-routing/connection/transport.go (Receive<-Event, Send(Event) error, Close() error)
- [ ] T005 Implement telemetry logging baseline in lib/ws-routing/telemetry/logger.go (Debug/Info/Warn/Error with fields)
- [ ] T006 Extract session orchestration into lib/ws-routing/session/session.go (accept AbstractTransport; no network ownership)
- [ ] T007 Extract routing engine into lib/ws-routing/router/router.go (per-symbol ordering; concurrency across symbols; backpressure=block upstream)
- [ ] T008 Move handler registry and middleware: market_data/framework/handler/{registry.go,middleware.go} → lib/ws-routing/handler/ (neutral names)
- [ ] T009 Port admin HTTP read-only endpoints: market_data/framework/api/{router.go,handlers_handler.go,metrics_handler.go,sessions_handler.go} → lib/ws-routing/api (implement GET /admin/ws-routing/v1/health, /subscriptions)
- [ ] T010 Update go.mod/go.sum and run `go mod tidy`
- [ ] T011 Ensure PERF-07 baselines: add router benchmarks in lib/ws-routing/router/bench_test.go (zero allocs; median <500ns; P99 <2µs)

**⚠️ CRITICAL**: Complete Phase 2 before any user story work

---

## Phase 3: User Story 1 - Universal Routing Library Available (Priority: P1) 🎯 MVP

**Goal**: Provide a reusable, business-agnostic routing library under `/lib/ws-routing` with logging, ordering, backpressure, and admin endpoints.

**Independent Test**: Initialize session with mock AbstractTransport; register middleware/handlers; verify per-symbol ordering and zero drops under slow handler; admin endpoints return health and subscriptions.

### Tests for User Story 1 (requested)

- [ ] T012 [P][US1] Unit tests for AbstractTransport mock in tests/unit/wsrouting/transport_mock_test.go
- [ ] T013 [P][US1] Unit tests for telemetry logger in lib/ws-routing/telemetry/logger_test.go
- [ ] T014 [US1] Ordering + backpressure tests in lib/ws-routing/router/router_ordering_test.go
- [ ] T015 [P][US1] Admin API tests in lib/ws-routing/api/api_test.go
- [ ] T016 [US1] Benchmarks in lib/ws-routing/router/bench_test.go (-benchmem; zero allocs enforced)

### Implementation for User Story 1

- [ ] T017 [P][US1] Implement session lifecycle in lib/ws-routing/session/session.go (start/subscribe/publish/close)
- [ ] T018 [P][US1] Implement router dispatch in lib/ws-routing/router/router.go (per-symbol order, parallel across symbols, blocking backpressure)
- [ ] T019 [P][US1] Implement handler registry and middleware composition in lib/ws-routing/handler/
- [ ] T020 [US1] Implement admin HTTP endpoints in lib/ws-routing/api/routes.go (health, subscriptions)
- [ ] T021 [US1] Wire telemetry logging in session/router paths (no vendor SDKs)

**Checkpoint**: Library builds; tests/benches pass for lib/ws-routing

---

## Phase 4: User Story 2 - Domain Adapters Updated (Priority: P2)

**Goal**: Update domain adapters and integration tests to consume `/lib/ws-routing`; remove legacy framework shim and directory.

**Independent Test**: Build succeeds with zero references to `market_data/framework`; integration tests in tests/integration/market_data continue to pass.

### Tests for User Story 2 (requested)

- [ ] T022 [P][US2] Add adapter conformance helper in internal/testhelpers/wsrouting/conformance.go
- [ ] T023 [US2] Update tests/integration/market_data/ws_routing_parity_test.go to use conformance helper and new imports
- [ ] T024 [P][US2] Update tests/integration/market_data/smoke_test.go imports to lib/ws-routing

### Implementation for User Story 2

- [ ] T025 [US2] Repo-wide import update: replace `github.com/coachpo/meltica/market_data/framework` → `github.com/coachpo/meltica/lib/ws-routing`
- [ ] T026 [US2] Delete market_data/framework/router/shim.go and related tests (helpers_test.go, acceptance_test.go, integration_test.go)
- [ ] T027 [US2] Delete entire market_data/framework directory after migration; move/rename neutral files already ported
- [ ] T028 [US2] Update go.mod/go.sum and run `go mod tidy` after removals
- [ ] T029 [US2] Preserve benchmarks: move/adapt market_data/events_benchmark_test.go to reference lib/ws-routing where applicable, or add new bench in lib path

**Checkpoint**: No references to market_data/framework; integration tests pass

---

## Phase 5: User Story 3 - Documentation and Tooling Converge (Priority: P3)

**Goal**: Documentation and tooling reflect new package location; CI runs full test suite with race/bench.

**Independent Test**: Contributors can locate /lib/ws-routing in ARCHITECTURE.md/README.md; BREAKING_CHANGES_v2.md lists import breakage; `make test` runs `go test ./... -race` successfully.

### Implementation for User Story 3

- [ ] T030 [US3] Update ARCHITECTURE.md with /lib/ws-routing boundaries and per-symbol ordering/backpressure
- [ ] T031 [US3] Update README.md examples/imports to /lib/ws-routing
- [ ] T032 [US3] Update BREAKING_CHANGES_v2.md with import path change and shim removal; fold any MIGRATION.md notes
- [ ] T033 [US3] Ensure Makefile `test` target uses `go test ./... -race`; add CI step to run tests
- [ ] T034 [US3] Repo-wide grep to confirm zero `market_data/framework` references remain

**Checkpoint**: Docs/tooling reflect new library; CI green

---

## Phase N: Polish & Cross-Cutting Concerns

- [ ] T035 [P] Run static analysis and architecture linter (`make lint-layers`); fix violations
- [ ] T036 Optimize router hot path if needed to hit PERF-07 (profile and reduce allocations)
- [ ] T037 [P] Final code cleanup, comments, and package docs

---

## Dependencies & Execution Order

- Setup (Phase 1) → Foundational (Phase 2) → US1 (P1) → US2 (P2) → US3 (P3) → Polish
- US2 depends on US1 completion; US3 depends on US1 and US2

## Parallel Opportunities

- [P] tasks within distinct packages: handler/, middleware/, telemetry/, api/ can proceed in parallel after T004–T007
- Test tasks T012, T013, T015 can run in parallel; implementation tasks T017–T019 can run in parallel
- US2 test updates T022, T024 can proceed in parallel

## Implementation Strategy

- MVP: Complete US1 (library availability with ordering/backpressure/admin + minimal tests/benches)
- Incremental: US2 (adapters + integration), then US3 (docs/tooling); finalize with polish and perf
