---
description: "Task list for WebSocket Routing Framework Extraction"
---

# Tasks: WebSocket Routing Framework Extraction

**Input**: Design documents from `/specs/009-goals-extract-the/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), research.md, data-model.md, contracts/

**Tests**: Contract, unit, integration, and smoke tests are required per specification to prove parity and coverage.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, Shared)
- Include exact file paths in descriptions

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish shared directories and fixtures needed by all stories.

- [X] T001 [Shared] Create `/lib/ws-routing/` directory with stub `doc.go`, `session.go`, `api.go`, and README placeholder to host the extracted framework.
- [X] T002 [P] [Shared] Copy deterministic WebSocket fixtures from `market_data/testdata/` into `tests/contract/ws-routing/testdata/` for reuse across contract and integration tests.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository-wide guardrails that must be in place before implementing any user story.

- [X] T003 [Shared] Update `internal/linter` rules to recognize `/lib/ws-routing` as framework code and block imports from `market_data/**` within that package.
- [X] T004 [Shared] Extend `Makefile`, `tests/architecture/`, and CI configs to discover `tests/contract/ws-routing/` and ensure the new contract suite runs under `go test`.

**Checkpoint**: Foundational guardrails in place — user story work can begin.

---

## Phase 3: User Story 1 - Shareable WebSocket Framework (Priority: P1) 🎯 MVP

**Goal**: Provide a reusable, domain-agnostic WebSocket routing framework under `/lib/ws-routing` with a stable API.

**Independent Test**: `go test ./lib/ws-routing/...` and `go test ./tests/contract/ws-routing -run TestAPISemantics` succeed using only the new framework without market data packages.

### Tests for User Story 1 (write first, expect to fail) ⚠️

- [X] T005 [P] [US1] Scaffold contract tests in `tests/contract/ws-routing/api_contract_test.go` that exercise Init/Start/Subscribe/Publish/UseMiddleware per `contracts/ws-routing-openapi.yaml`.
- [X] T006 [P] [US1] Add unit tests in `lib/ws-routing/session_test.go` covering parser errors, backoff transitions, and middleware ordering using fixtures from `tests/contract/ws-routing/testdata/`.

### Implementation for User Story 1

- [X] T007 [US1] Implement `FrameworkSession`, backoff configuration, and typed error plumbing in `lib/ws-routing/session.go`.
- [X] T008 [US1] Move generic routing pipeline logic from `market_data/framework/router/` into `lib/ws-routing/router.go`, stripping domain-specific dependencies.
- [X] T009 [US1] Provide exported API functions (`Init`, `Start`, `Subscribe`, `Publish`, `UseMiddleware`) in `lib/ws-routing/api.go`, wiring them to the new core components.
- [X] T010 [US1] Implement middleware chain management in `lib/ws-routing/middleware/chain.go` and ensure thread-safe invocation within the session lifecycle.

**Checkpoint**: Framework API green — contract and unit tests pass without domain code.

---

## Phase 4: User Story 2 - Market Data Adapters on Framework (Priority: P2)

**Goal**: Migrate market data adapters to the new framework, eliminate direct imports of internal router code, and maintain runtime parity.

**Independent Test**: `go test ./tests/integration/market_data -run TestWSRoutingParity && go test ./market_data/...` succeed, proving message parity and adapter isolation.

### Tests for User Story 2 (write first, expect to fail) ⚠️

- [X] T011 [P] [US2] Add failing parity test skeleton in `tests/integration/market_data/ws_routing_parity_test.go` comparing legacy vs. framework outputs for shared fixtures.
- [X] T012 [P] [US2] Add architecture test `tests/architecture/market_data_ws_routing_test.go` ensuring `market_data/**` only depends on `lib/ws-routing` public API.

### Implementation for User Story 2

- [X] T013 [US2] Refactor `market_data/framework/api/router.go` to initialize and register routes through `lib/ws-routing` sessions.
- [X] T014 [US2] Update `market_data/framework/api/sessions_handler.go` (and related handlers) to invoke `Subscribe`/`Publish` from `lib/ws-routing` while preserving message schemas.
- [X] T015 [US2] Remove migrated generic router code from `market_data/framework/router/*.go`, leaving only domain-specific adapters and handlers.
- [X] T016 [US2] Introduce `market_data/framework/router/shim.go` that re-exports `lib/ws-routing` API with deprecation warnings for one-release support.
- [X] T017 [US2] Update router tests under `market_data/framework/router/` (e.g., `acceptance_test.go`, `integration_test.go`) to validate parity via the new framework API.
- [X] T018 [P] [US2] Implement smoke test `tests/integration/market_data/smoke_test.go` following quickstart steps to exercise reconnect/backoff scenarios.
- [X] T019 [US2] Execute the new parity and smoke tests, capturing baseline outputs referenced in `specs/009-goals-extract-the/quickstart.md`.
- [X] T020 [US2] Compare pre- and post-migration structured logs from the smoke test to confirm logging parity; record findings in `specs/009-goals-extract-the/quickstart.md`.

**Checkpoint**: Market data adapters run exclusively on `/lib/ws-routing` with parity verified.

---

## Phase 5: User Story 3 - Migration Confidence and Guidance (Priority: P3)

**Goal**: Deliver documentation, examples, and migration guidance that enable downstream consumers to adopt the new framework safely.

**Independent Test**: Following `/lib/ws-routing/README.md` and the example app allows a consumer to replicate the quickstart flow without referencing legacy APIs.

### Implementation for User Story 3

- [X] T021 [US3] Author `/lib/ws-routing/README.md` documenting API usage, configuration, and linking to contract tests and quickstart steps.
- [X] T022 [US3] Provide runnable example in `/lib/ws-routing/examples/basic/main.go` demonstrating init/start/subscribe/publish/middleware usage.
- [X] T023 [US3] Document migration notes and deprecation timeline in `docs/migrations/ws-routing.md` and cross-link from `BREAKING_CHANGES_v2.md`.
- [X] T024 [US3] Update `market_data/README.md` and `specs/009-goals-extract-the/quickstart.md` to highlight the new framework adoption path and deprecation shim guidance.

**Checkpoint**: Documentation and migration artifacts published; downstream teams can adopt the framework.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final cleanup and validation across the repository.

- [X] T025 [P] [Shared] Run `gofmt`, `goimports`, and `go mod tidy` to normalize formatting and dependency metadata for new packages.
- [X] T026 [Shared] Execute `go test ./... -race`, `make lint-layers`, and the new integration smoke tests, recording results in `specs/009-goals-extract-the/quickstart.md`.
- [X] T027 [Shared] Reconcile documentation duplicates and finalize clarifications in `specs/009-goals-extract-the/spec.md`, ensuring deprecation shim messaging is consistent.
- [X] T028 [Shared] Capture architecture council approval for registering `/lib/ws-routing` (meeting notes or ticket link) in `specs/009-goals-extract-the/research.md`.

---

## Dependencies & Execution Order

1. Setup (Phase 1) must complete before Foundational tasks.
2. Foundational guardrails (Phase 2) must finish before any user story phases.
3. User Story phases proceed in priority order: US1 → US2 → US3.
4. Polish phase runs after all user stories reach their checkpoints.

## Parallel Execution Examples

- During Phase 1, T001 and T002 touch different targets and can proceed in parallel once directory creation is staged.
- In US1, T005 and T006 are independent test scaffolds and may run concurrently.
- In US2, T011 and T012 operate on distinct test suites and can be developed simultaneously; once adapters land, T017 can update existing tests while T018 runs smoke scenarios, and T020 validates logging parity.
- Polish task T025 can start while integration validations (T026) are queued, provided code changes are complete.

## Implementation Strategy

### MVP First (User Story 1 Only)
1. Complete Setup and Foundational phases.
2. Finish User Story 1 (framework API + tests) and verify contract/unit suites.
3. Pause for review before integrating domain adapters.

### Incremental Delivery
1. Deliver US1 to unlock reuse for other domains.
2. Migrate market data adapters (US2) to prove production parity.
3. Publish migration tooling and docs (US3) to support downstream consumers.

### Parallel Team Strategy
- Developer A: Focus on framework extraction (US1) while Developer B prepares market data parity tests (T011/T012).
- After US1 checkpoint, split US2 tasks—one developer updating adapters (T013-T016), another enhancing tests and verification (T017-T020).
- Documentation specialist handles US3 tasks concurrently once API stabilizes.
