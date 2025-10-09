# Tasks: Lightweight Real-Time WebSocket Framework

**Input**: Design documents from `/specs/002-build-a-lightweight/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Automated tests were not explicitly requested; performance benchmarks and manual validation steps are included where required by acceptance criteria.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [X] T001 [Setup] Add `github.com/goccy/go-json` dependency in `go.mod`, run `go mod tidy`, and commit updated `go.sum`.
- [X] T002 [Setup] Scaffold directories `core/stream`, `market_data/framework/{connection,parser,handler,telemetry,api}`, `internal/benchmarks/market_data/framework`, and `tests/market_data/framework/{unit,integration,benchmarks}` with placeholder `doc.go` files to establish package boundaries.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T003 [Foundation] Define canonical streaming interfaces (engine, session, handler contracts) in `core/stream/interfaces.go` aligned with CQ-01.
- [X] T004 [Foundation] Implement shared framework types (`ConnectionSession`, `MessageEnvelope`, `HandlerOutcome`, `MetricsSnapshot`) in `market_data/framework/types.go` per data-model.md.
- [X] T005 [Foundation] Build pooled resource manager (`PoolHandle` with decoder/buffer pools) in `market_data/framework/connection/pool.go` using `sync.Pool` and `goccy/go-json` decoder pooling.
- [X] T006 [Foundation] Implement optional handshake authentication hook wiring in `market_data/framework/connection/auth.go`, exposing configuration on the engine so integrators can verify clients during the dial sequence (FR-008).
- [X] T007 [Foundation] Create base HTTP router and error mapping in `market_data/framework/api/router.go`, wiring `errs` codes and context-aware middleware for all forthcoming endpoints.

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Operate high-volume real-time feeds (Priority: P1) 🎯 MVP

**Goal**: Provide a pooled WebSocket engine that sustains thousands of concurrent connections with sub-10 ms processing latency.

**Independent Test**: Deploy the engine against a representative market feed and sustain 5,000 concurrent clients for 30 minutes while benchmarks confirm median processing latency <10 ms and connection churn <1%.

- [X] T008 [US1] Implement `Engine` connection lifecycle (dial, run loops, graceful shutdown) in `market_data/framework/connection/engine.go` using `gorilla/websocket` and context propagation.
- [X] T009 [P] [US1] Implement heartbeat, read-limit, and backpressure monitor in `market_data/framework/connection/heartbeat.go`, enforcing configurable throttling (FR-007).
- [X] T010 [P] [US1] Implement pooled JSON decode stage with `goccy/go-json` in `market_data/framework/parser/json_pipeline.go`, borrowing decoders and populating `MessageEnvelope` (FR-002, FR-005).
- [X] T011 [US1] Implement validation and invalid-message tracking pipeline in `market_data/framework/parser/validation_stage.go`, emitting `errs.CodeInvalid` events while keeping connections open (FR-003).
- [X] T012 [US1] Add session bootstrap endpoint handler for `POST /v1/stream/sessions` in `market_data/framework/api/sessions_handler.go`, invoking the engine and returning `Session` payload per contract.
- [X] T013 [P] [US1] Author high-throughput benchmark `internal/benchmarks/market_data/framework/engine_bench_test.go` exercising pooled decoding and connection fan-out to validate PERF-01/03.

**Checkpoint**: User Story 1 implemented and benchmarked; framework can ingest real-time feeds independently.

---

## Phase 4: User Story 2 - Integrate custom business handlers (Priority: P2)

**Goal**: Allow teams to register custom validation/business logic and receive decoded payloads without touching framework internals.

**Independent Test**: Register a sample handler that enriches messages; confirm it receives decoded payloads, can emit outcomes, and framework routes handler errors without blocking other clients.

- [X] T014 [US2] Implement handler registry and lifecycle management in `market_data/framework/handler/registry.go`, supporting registration, lookup, and versioning.
- [X] T015 [P] [US2] Implement handler middleware and context propagation hooks in `market_data/framework/handler/middleware.go`, enabling optional auth hook execution.
- [X] T016 [US2] Integrate handler dispatch into connection engine in `market_data/framework/connection/dispatch.go`, ensuring handler outcomes control message flow (FR-004).
- [X] T017 [P] [US2] Update `specs/002-build-a-lightweight/quickstart.md` with a step-by-step handler onboarding checklist so teams can scaffold new handlers in under one day (SC-003).
- [X] T018 [P] [US2] Expose handler registration API `POST /v1/stream/handlers` in `market_data/framework/api/handlers_handler.go`, validating payload against contract schemas.

**Checkpoint**: User Stories 1 and 2 deliver a pluggable streaming core with custom logic support.

---

## Phase 5: User Story 3 - Observe and tune throughput (Priority: P3)

**Goal**: Provide real-time metrics, logs, and alerts so SRE teams can monitor throughput, latency, and pooling efficiency.

**Independent Test**: Run the framework under staged load, verify metrics exports report throughput/latency/resource reuse, and confirm alert hooks fire within 60 seconds when thresholds are breached.

### Implementation for User Story 3

- [X] T019 [US3] Implement rolling metrics aggregator and snapshots in `market_data/framework/telemetry/metrics.go`, capturing per-session throughput, latency percentiles, and allocation counters (FR-006).
- [X] T020 [P] [US3] Implement telemetry hooks and structured logging in `market_data/framework/telemetry/logging.go`, exposing callbacks for operators to attach monitors.
- [X] T021 [US3] Implement metrics endpoint handler `GET /v1/stream/sessions/{sessionId}/metrics` in `market_data/framework/api/metrics_handler.go`, returning `MetricsSnapshot` per contract.

**Checkpoint**: All user stories independently functional with monitoring and tuning capabilities.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [X] T022 [Polish] Run `gofmt`/`goimports` across new packages and ensure package-level documentation complies with CQ-07.
- [X] T023 [Polish] Execute `go test -race ./...` and run `go test -bench . ./internal/benchmarks/market_data/framework` to verify concurrency safety and performance targets.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** → prerequisite for Foundational work.
- **Foundational (Phase 2)** → must complete before starting any user story phases.
- **User Story Phases (3–5)** → may begin after Phase 2; execute in priority order (P1 → P2 → P3) for MVP delivery, though different teams can tackle them in parallel once prerequisites are ready.
- **Polish (Phase 6)** → runs after desired user stories reach checkpoints.

### User Story Dependencies

- **US1**: Depends on Foundational phase only.
- **US2**: Depends on Foundational (including handshake auth hook) and completion of US1 engine hooks.
- **US3**: Depends on Foundational and instrumentation points created during US1.

### Within Each User Story

- US1: Complete T008 before T009–T012; benchmark (T013) can run once engine and pipelines exist.
- US2: Registry (T014) precedes middleware (T015) and dispatch integration (T016); quickstart onboarding checklist update (T017) and API handler (T018) depend on registry.
- US3: Metrics aggregator (T019) precedes telemetry hooks (T020) and metrics endpoint (T021).

## Parallel Execution Examples

### User Story 1
- Run T009 and T010 in parallel once T008 establishes the engine skeleton.
- Run T013 benchmark in parallel after T010 and T011 are complete.

### User Story 2
- Execute T015 in parallel with T014 once interfaces are declared; begin T017 and T018 after T014.

### User Story 3
- Execute T020 in parallel with T019 once shared metric structs exist.

## Implementation Strategy

### MVP First (User Story 1 Only)
1. Complete Phases 1 and 2.
2. Deliver Phase 3 (US1) and validate benchmarks → deployable MVP.

### Incremental Delivery
1. MVP (US1) provides high-throughput streaming.
2. US2 adds custom handler extensibility without disturbing US1.
3. US3 layers observability for operations teams.

### Parallel Team Strategy
- After Foundational phase, dedicate squads per story: one on connection engine (US1), one on handler extensibility (US2), one on telemetry (US3). Coordination points highlighted in dependencies above.
