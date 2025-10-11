# Tasks: Four-Layer Architecture Implementation

**Input**: Design documents from `/specs/008-architecture-requirements-req/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Included - Feature specification requires testability (FR-009) and independent layer testing (US2)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- File paths are absolute from repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and directory structure

- [X] T001 [P] Create core/layers/ directory structure for interface definitions
- [X] T002 [P] Create core/layers/categories/ directory for market type-specific interfaces
- [X] T003 [P] Create internal/linter/ directory for static analysis tool
- [X] T004 [P] Create tests/architecture/ directory for conformance tests
- [X] T005 [P] Add golang.org/x/tools dependency if not already present (check go.mod)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core interfaces and infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 [P] [Foundation] Define base Connection interface in core/layers/connection.go
- [X] T007 [P] [Foundation] Define base Routing interface in core/layers/routing.go
- [X] T008 [P] [Foundation] Define base Business interface in core/layers/business.go
- [X] T009 [P] [Foundation] Define base Filter interface in core/layers/filter.go
- [X] T010 [P] [Foundation] Define WSConnection interface extending Connection in core/layers/connection.go
- [X] T011 [P] [Foundation] Define RESTConnection interface extending Connection in core/layers/connection.go
- [X] T012 [P] [Foundation] Define WSRouting interface extending Routing in core/layers/routing.go
- [X] T013 [P] [Foundation] Define RESTRouting interface extending Routing in core/layers/routing.go
- [X] T014 [P] [Foundation] Define SpotConnection interface in core/layers/categories/spot.go
- [X] T015 [P] [Foundation] Define FuturesConnection interface in core/layers/categories/futures.go
- [X] T016 [P] [Foundation] Define OptionsConnection interface in core/layers/categories/options.go
- [X] T017 [P] [Foundation] Define SpotRouting interface in core/layers/categories/spot.go
- [X] T018 [P] [Foundation] Define FuturesRouting interface in core/layers/categories/futures.go
- [X] T019 [P] [Foundation] Define OptionsRouting interface in core/layers/categories/options.go
- [X] T020 [P] [Foundation] Add godoc comments to all layer interfaces documenting contracts and guarantees (per CQ-07: Document expected behavior, include usage examples for non-obvious methods)
- [X] T020b [P] [Foundation] Add godoc comments to all exported types and functions in core/layers/ package (per CQ-07)
- [X] T021 [P] [Foundation] Create internal/observability/ package for cross-cutting concerns (logging, metrics)
- [X] T022 [Foundation] Define Logger interface with global accessor in internal/observability/logger.go
- [X] T023 [Foundation] Define Metrics interface with global accessor in internal/observability/metrics.go

**Checkpoint**: Foundation ready - interfaces defined, user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Layer Identification and Navigation (Priority: P1) 🎯 MVP

**Goal**: Developers can quickly identify which layer a component belongs to and understand dependencies through clear interfaces and automated boundary checking

**Independent Test**: Navigate to any package (e.g., exchanges/binance/infra/ws/) and verify layer membership is immediately apparent; run static analyzer and confirm it correctly identifies layer and validates dependencies

### Tests for User Story 1

**NOTE: Write these tests FIRST, ensure they FAIL before implementation**

- [X] T024 [P] [US1] Create layer boundary test in tests/architecture/layer_boundaries_test.go that verifies no imports violate hierarchy
- [X] T025 [P] [US1] Create interface contract test framework in tests/architecture/interface_contracts_test.go with table-driven pattern
- [X] T026 [P] [US1] Add contract test for Connection interface (idempotency, cleanup, deadline semantics)
- [X] T027 [P] [US1] Add contract test for Routing interface (subscribe idempotency, unsubscribe safety, message ordering)

### Implementation for User Story 1

- [X] T028 [P] [US1] Implement static analyzer main structure in internal/linter/analyzer.go using golang.org/x/tools/go/analysis
- [X] T029 [US1] Define layer assignment rules (package glob patterns → layer) in internal/linter/rules.go
- [X] T030 [US1] Implement import validation logic in analyzer (check imports against allowed dependencies)
- [X] T031 [US1] Add analyzer error reporting with fix suggestions
- [X] T032 [US1] Create analyzer testdata fixtures in internal/linter/testdata/ with valid and invalid examples
- [X] T033 [P] [US1] Write unit tests for analyzer in internal/linter/analyzer_test.go
- [X] T034 [P] [US1] Update Makefile to add `make lint-layers` target that runs static analyzer
- [X] T035 [P] [US1] Add architecture overview doc to exchanges/sys_arch_overview.md (update existing) with layer interface references
- [X] T036 [US1] Test analyzer on existing codebase - verify it correctly identifies layers and reports current violations (expected)

**Checkpoint**: At this point, developers can identify layers via interfaces and static analysis validates boundaries

---

## Phase 4: User Story 4 - Incremental Migration (Priority: P1)

**Goal**: Existing Binance code migrated to use layer interfaces incrementally without breaking functionality, proving the migration path works

**Independent Test**: Run existing Binance tests (they should pass); verify migrated components implement layer interfaces; confirm legacy and migrated components interoperate

### Tests for User Story 4

- [X] T037 [P] [US4] Create migration validation test in tests/architecture/migration_test.go that checks wrapped implementations satisfy interfaces
- [X] T038 [P] [US4] Add integration test for mixed legacy/migrated components in tests/integration/binance_migration_test.go

### Implementation for User Story 4

**Sub-phase 4a: Migrate L1 Connection Layer (Bottom-up strategy)**

- [X] T039 [P] [US4] Create connection adapter for Binance WebSocket client in exchanges/binance/infra/ws/adapter.go implementing layers.WSConnection
- [X] T040 [P] [US4] Create connection adapter for Binance REST client in exchanges/binance/infra/rest/adapter.go implementing layers.RESTConnection
- [X] T041 [US4] Update Binance WebSocket client to optionally expose layer interface via AsLayerInterface() method
- [X] T042 [US4] Update Binance REST client to optionally expose layer interface via AsLayerInterface() method
- [X] T043 [US4] Run existing Binance infra tests - verify adapters work correctly

**Sub-phase 4b: Migrate L2 Routing Layer**

- [X] T044 [P] [US4] Create routing adapter for Binance WSRouter in exchanges/binance/routing/adapter.go implementing layers.WSRouting
- [X] T045 [P] [US4] Create routing adapter for Binance RESTRouter in exchanges/binance/routing/rest_adapter.go implementing layers.RESTRouting
- [X] T046 [US4] Update Binance routing constructors to accept layers.Connection instead of concrete types
- [X] T047 [US4] Update routing internal calls to use interface methods
- [X] T048 [US4] Run existing Binance routing tests - verify adapters work correctly

**Sub-phase 4c: Migrate L3 Business Layer**

- [X] T049 [P] [US4] Create business adapter for Binance session/bridge in exchanges/binance/bridge/adapter.go implementing layers.Business
- [X] T050 [US4] Update Binance business constructors to accept layers.Routing instead of concrete types
- [X] T051 [US4] Update business internal calls to use interface methods
- [X] T052 [US4] Run existing Binance business/bridge tests - verify adapters work correctly

**Sub-phase 4d: Migrate L4 Filter Layer**

- [X] T053 [US4] Update pipeline coordinator in pipeline/coordinator.go to use layer interfaces
- [X] T054 [US4] Update pipeline stages in pipeline/stages.go to use layer interfaces
- [X] T055 [US4] Run existing pipeline tests - verify interface usage works correctly

**Sub-phase 4e: End-to-End Migration Validation**

- [X] T056 [US4] Run full Binance acceptance tests (acceptance_test.go) with migrated components
- [X] T057 [US4] Run migration validation tests from T037-T038
- [X] T058 [US4] Document migration experience in specs/008-architecture-requirements-req/migration-notes.md (lessons learned, issues encountered)
- [X] T058b [US4] Add godoc comments to all exported adapter types and methods in exchanges/binance/*/adapter.go files

**Checkpoint**: At this point, Binance is fully migrated and serves as reference implementation; migration path proven viable

---

## Phase 5: User Story 2 - Independent Layer Testing (Priority: P2)

**Goal**: Developers can test layers in isolation using mock interfaces without complex setup, improving test velocity and clarity

**Independent Test**: Write a unit test for Business layer that uses mock Routing and Connection - verify test requires no real connections or external dependencies

### Tests for User Story 2

- [X] T059 [P] [US2] Create example mock Connection in tests/architecture/mocks/connection_mock.go
- [X] T060 [P] [US2] Create example mock Routing in tests/architecture/mocks/routing_mock.go
- [X] T061 [P] [US2] Create example mock Business in tests/architecture/mocks/business_mock.go
- [X] T062 [US2] Write example isolated test for Routing layer using mock Connection in tests/architecture/examples/routing_isolated_test.go
- [X] T063 [US2] Write example isolated test for Business layer using mock Routing in tests/architecture/examples/business_isolated_test.go
- [X] T064 [US2] Write example isolated test for Filter layer using mock Business in tests/architecture/examples/filter_isolated_test.go

### Implementation for User Story 2

- [X] T065 [P] [US2] Add testing guidance to quickstart.md section "Testing" with mock examples
- [X] T066 [P] [US2] Add contract test for Business interface in tests/architecture/interface_contracts_test.go
- [X] T067 [P] [US2] Add contract test for Filter interface in tests/architecture/interface_contracts_test.go
- [X] T067b [US2] Add performance assertions to all contract tests verifying isolated layer test execution completes in <100ms (validates SC-003)
- [X] T068 [US2] Update existing Binance routing tests to demonstrate isolation (use mocks for connections)
- [X] T069 [US2] Update existing Binance business tests to demonstrate isolation (use mocks for routing)
- [X] T070 [US2] Document testing patterns in quickstart.md with before/after examples showing complexity reduction

**Checkpoint**: At this point, testing documentation is complete with examples; developers can easily test layers in isolation

---

## Phase 6: User Story 3 - Consistent Exchange Integration (Priority: P3)

**Goal**: Developers integrating new exchanges have clear templates and patterns to follow, ensuring all exchanges maintain four-layer structure

**Independent Test**: Use the template to create a skeleton exchange (e.g., "testexchange"); verify all four layers present and static analyzer validates structure

### Tests for User Story 3

- [X] T071 [US3] Create template validation test in tests/architecture/template_validation_test.go that checks exchange template structure
- [X] T072 [US3] Add test for exchange template skeleton generator (if implemented)

### Implementation for User Story 3

- [X] T073 [P] [US3] Create exchange template directory structure in internal/templates/exchange/
- [X] T074 [P] [US3] Create template for L1 Connection in internal/templates/exchange/infra/ws/client.go.tmpl
- [X] T075 [P] [US3] Create template for L1 REST Connection in internal/templates/exchange/infra/rest/client.go.tmpl
- [X] T076 [P] [US3] Create template for L2 Routing in internal/templates/exchange/routing/ws_router.go.tmpl
- [X] T077 [P] [US3] Create template for L2 REST Routing in internal/templates/exchange/routing/rest_router.go.tmpl
- [X] T078 [P] [US3] Create template for L3 Business in internal/templates/exchange/bridge/session.go.tmpl
- [X] T079 [P] [US3] Create template for L4 Filter/Pipeline integration in internal/templates/exchange/adapter.go.tmpl
- [X] T080 [US3] Add "Adding a New Exchange" detailed guide in quickstart.md with template references
- [X] T081 [US3] Create optional exchange skeleton generator script in scripts/new-exchange.sh that uses templates
- [X] T082 [US3] Test generator by creating a test exchange skeleton and running static analyzer on it

**Checkpoint**: At this point, exchange integration is well-documented with templates; new integrations follow consistent patterns

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Finalization, documentation updates, and cross-cutting improvements

- [X] T083 [P] Add CI integration for static analyzer in .github/workflows/ or CI config
- [X] T084 [P] Update main README.md with four-layer architecture overview and link to quickstart
- [X] T085 [P] Add architecture diagram to docs/ directory (visual representation of four layers)
- [X] T086 [P] Create ARCHITECTURE.md at repo root summarizing layer responsibilities and linking to detailed docs
- [X] T087 Update CLAUDE.md with architectural patterns and layer interface locations
- [X] T088 [P] Add code coverage reporting for architecture tests
- [X] T088b Configure coverage gates in CI with thresholds: 90% for core/, 80% for exchanges/*/routing/, 70% for exchanges/*/infra/, 75% overall (per TS-01)
- [X] T089 Run full test suite (unit + integration + architecture) and verify all passing
- [X] T090 Run static analyzer on full codebase and document remaining violations (expected during migration)
- [X] T091 Update BREAKING_CHANGES_v2.md with layer interface migration notes
- [X] T092 [P] Add examples/ directory with sample layer implementations
- [X] T093 Run quickstart.md validation as smoke test
- [X] T094 Final review: Verify all user stories have acceptance criteria met

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational phase
- **User Story 4 (Phase 4)**: Depends on US1 (needs interfaces defined)
- **User Story 2 (Phase 5)**: Depends on US4 (needs migrated code to demonstrate testing)
- **User Story 3 (Phase 6)**: Depends on US1 and US4 (needs interfaces and reference implementation)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

```
Foundation (Phase 2) - BLOCKS ALL
    ↓
US1 (P1): Layer Identification - Phase 3
    ↓
US4 (P1): Incremental Migration - Phase 4 (depends on US1)
    ↓
US2 (P2): Independent Testing - Phase 5 (benefits from US4 examples)
    ↓
US3 (P3): Consistent Integration - Phase 6 (needs US1 interfaces + US4 reference)
```

### Within Each User Story

- Tests FIRST (ensure they fail)
- Foundation interfaces before implementations
- Adapters before refactoring internals
- Lower layers before higher layers (L1 → L2 → L3 → L4)
- Individual layer migration validated before moving to next
- Story checkpoint verification before proceeding

### Parallel Opportunities

**Phase 1 (Setup)**: All tasks T001-T005 can run in parallel

**Phase 2 (Foundation)**: 
- T006-T019 (interface definitions) all parallel
- T020 (documentation) depends on T006-T019
- T021-T023 (observability) parallel with interface definitions

**Phase 3 (US1)**:
- Tests T024-T027 all parallel
- Analyzer T028-T031 sequential (core logic)
- T032-T036 can run in parallel after analyzer core complete

**Phase 4 (US4)**:
- Sub-phase 4a: T039-T040 parallel
- Sub-phase 4b: T044-T045 parallel
- Sub-phase 4c: T049 standalone
- Sub-phase 4d: T053-T054 parallel

**Phase 5 (US2)**:
- Mocks T059-T061 all parallel
- Examples T062-T064 parallel after mocks
- Documentation T065-T067 parallel

**Phase 6 (US3)**:
- Templates T074-T079 all parallel
- T080-T082 sequential (integration)

**Phase 7 (Polish)**:
- T083-T086 all parallel
- T088-T092 all parallel

---

## Parallel Example: User Story 1 - Layer Identification

```bash
# Launch all interface definitions together (Foundation):
Task: "Define base Connection interface in core/layers/connection.go"
Task: "Define base Routing interface in core/layers/routing.go"
Task: "Define base Business interface in core/layers/business.go"
Task: "Define base Filter interface in core/layers/filter.go"

# Launch all contract tests together (US1):
Task: "Create layer boundary test in tests/architecture/layer_boundaries_test.go"
Task: "Create interface contract test framework in tests/architecture/interface_contracts_test.go"
Task: "Add contract test for Connection interface"
Task: "Add contract test for Routing interface"

# Implement static analyzer sequentially:
Task: "Implement static analyzer main structure in internal/linter/analyzer.go"
Task: "Define layer assignment rules in internal/linter/rules.go"
Task: "Implement import validation logic"
```

---

## Parallel Example: User Story 4 - Incremental Migration

```bash
# L1 Connection adapters in parallel:
Task: "Create connection adapter for Binance WebSocket client in exchanges/binance/infra/ws/adapter.go"
Task: "Create connection adapter for Binance REST client in exchanges/binance/infra/rest/adapter.go"

# L2 Routing adapters in parallel:
Task: "Create routing adapter for Binance WSRouter in exchanges/binance/routing/adapter.go"
Task: "Create routing adapter for Binance RESTRouter in exchanges/binance/routing/rest_adapter.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - defines all interfaces)
3. Complete Phase 3: User Story 1
4. **STOP and VALIDATE**: 
   - Can developers identify layers?
   - Does static analyzer work?
   - Are contract tests passing?
5. Demo/review if ready

### Incremental Delivery

1. Setup + Foundational → Interface contracts defined, static analyzer ready
2. Add US1 → Test independently → Developers can navigate architecture ✅
3. Add US4 → Test independently → Binance fully migrated, migration proven ✅
4. Add US2 → Test independently → Testing patterns documented with examples ✅
5. Add US3 → Test independently → New exchange integration template ready ✅
6. Polish → CI integration, final documentation
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers after Foundation completes:

**Scenario 1: Two developers**
- Developer A: US1 (Layer Identification) - Highest priority
- Developer B: Begin US4 L1 migration prep (review existing code)
- After US1 complete: Developer A moves to US4, Developer B continues US4

**Scenario 2: Three developers**
- Developer A: US1 (static analyzer, tests)
- Developer B: US4 L1+L2 migration (Connection, Routing)
- Developer C: US4 L3+L4 migration (Business, Filter)
- After US1+US4 complete: 
  - Developer A: US2 (testing patterns)
  - Developer B: US3 (templates)
  - Developer C: Polish

---

## Task Count Summary

- **Phase 1 (Setup)**: 5 tasks
- **Phase 2 (Foundational)**: 18 tasks
- **Phase 3 (US1 - Layer Identification)**: 13 tasks
- **Phase 4 (US4 - Incremental Migration)**: 22 tasks
- **Phase 5 (US2 - Independent Testing)**: 12 tasks
- **Phase 6 (US3 - Consistent Integration)**: 12 tasks
- **Phase 7 (Polish)**: 12 tasks

**Total**: 94 tasks

**Parallelizable**: ~45 tasks marked [P] (48%)

**Critical Path**:
1. Setup (5 tasks)
2. Foundational (18 tasks) - BLOCKS ALL
3. US1 Core (13 tasks) - ~8 sequential
4. US4 Migration (22 tasks) - ~12 sequential
5. US2+US3 (24 tasks) - Can overlap with US4 partially
6. Polish (12 tasks)

**Estimated Sequential Critical Path**: ~60 tasks
**With 3 developers in parallel**: ~25-30 task cycles

---

## Notes

- [P] tasks = different files, no dependencies, safe to parallelize
- [Story] label (US1-US4) maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing (TDD approach for contracts)
- Commit after each task or logical group
- Stop at each checkpoint to validate story independently
- Foundation phase is CRITICAL - all interfaces must be defined before any migration
- Migration is bottom-up (L1→L2→L3→L4) to minimize breaking changes
- Static analyzer catches violations at build time - enable in CI when migration >50% complete
- When breaking changes needed for architecture improvement, create migration tasks rather than deferring innovation
