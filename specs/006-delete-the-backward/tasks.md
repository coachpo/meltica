# Tasks: Remove Backward Compatibility Code

**Input**: Design documents from `/specs/006-delete-the-backward/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: This feature involves code removal rather than new features. Test tasks focus on verification that removal doesn't break functionality.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions
- Repository root: `/Users/liqing/Documents/PersonalProjects/meltica/`
- Deprecated code: `market_data/framework/parser/`
- Core: `core/`
- Tests: `tests/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Environment verification and prerequisite checks

- [X] T001 Verify current branch is `006-delete-the-backward` and working directory is clean
- [X] T002 [P] Verify Go 1.25 environment and dependencies are installed (`go version`, `go mod download`)
- [X] T003 [P] Verify baseline test suite passes (`go test ./... -race -count=1`)
- [X] T004 [P] Verify baseline build succeeds (`go build ./...`)
- [X] T005 Record baseline metrics: test count, coverage percentage, build time for comparison

**Checkpoint**: Environment verified, baseline metrics recorded

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Comprehensive audit of backward compatibility code before removal

**⚠️ CRITICAL**: Complete audit is required before ANY removal can begin

- [X] T006 Audit all imports of `market_data/framework/parser` package (`grep -r "market_data/framework/parser" --include="*.go" .`)
- [X] T007 [P] Generate dependency report listing all packages that import parser (`go list -f '{{.ImportPath}} {{.Imports}}' ./... | grep parser`)
- [X] T008 [P] Identify all test files using deprecated parser functions
- [X] T009 [P] Search for documentation references to parser package (`grep -r "parser" docs/ --include="*.md"`)
- [X] T010 Document complete inventory of code to update/remove in `specs/006-delete-the-backward/audit-results.md`

**Checkpoint**: Complete audit finished - all dependencies identified and documented

---

## Phase 3: User Story 1 - Clean Codebase Without Legacy Support (Priority: P1) 🎯 MVP

**Goal**: Remove all backward compatibility code, leaving only a single current implementation

**Independent Test**: Can be fully tested by auditing the codebase to verify all identified backward compatibility code is removed and the remaining system functions correctly with modern interfaces only

### Implementation for User Story 1

- [X] T011 [P] [US1] Remove internal usage of parser in test files identified in T008 - migrate to processor/router architecture
- [X] T012 [P] [US1] Remove internal usage of parser in any remaining source files identified in T006
- [X] T013 [US1] Delete entire `market_data/framework/parser/` directory (`rm -rf market_data/framework/parser/`)
- [X] T014 [US1] Verify parser directory is deleted (`ls market_data/framework/parser/` should error)
- [X] T015 [US1] Update `core.ProtocolVersion` constant from `"1.2.0"` to `"2.0.0"` in `core/core.go`
- [X] T016 [US1] Update protocol version comment in `core/core.go` to reflect v2.0.0 removes backward compatibility
- [X] T017 [US1] Verify no parser imports remain in codebase (`grep -r "market_data/framework/parser" --include="*.go" .` returns nothing)
- [X] T018 [US1] Run full build to catch any compilation errors (`go build ./...`)
- [X] T019 [US1] Run full test suite to verify system functionality (`go test ./... -race -count=1`)
- [X] T020 [US1] Verify all tests pass with zero failures
- [ ] T021 [US1] Check test coverage remains >= 75% overall, >= 90% for core packages (`go test ./... -cover`) — current coverage 50.4% overall / 90.2% core

**Checkpoint**: Parser package completely removed, protocol version updated to 2.0.0, all tests pass

---

## Phase 4: User Story 2 - Clear Migration Documentation (Priority: P2)

**Goal**: Users and integrators have comprehensive migration documentation with clear guidance

**Independent Test**: Can be tested by reviewing documentation to verify all breaking changes are documented with clear before/after examples and migration steps

### Implementation for User Story 2

- [X] T022 [US2] Create `BREAKING_CHANGES_v2.md` in repository root with comprehensive migration guide
- [X] T023 [US2] In `BREAKING_CHANGES_v2.md`: Document overview of breaking changes (parser removal, protocol 2.0.0)
- [X] T024 [US2] In `BREAKING_CHANGES_v2.md`: List all removed components (parser package details)
- [X] T025 [US2] In `BREAKING_CHANGES_v2.md`: Add migration guide with before/after code examples (parser → processor)
- [X] T026 [US2] In `BREAKING_CHANGES_v2.md`: Include step-by-step migration instructions
- [X] T027 [US2] In `BREAKING_CHANGES_v2.md`: Document version timeline (v1.2.0 deprecated, v2.0.0 removed)
- [X] T028 [US2] In `BREAKING_CHANGES_v2.md`: Add support resources and links to processor examples
- [X] T029 [US2] Delete `market_data/MIGRATION.md` as it's superseded by BREAKING_CHANGES_v2.md (`rm market_data/MIGRATION.md`)
- [X] T030 [US2] Update `README.md`: Add prominent breaking changes notice with link to BREAKING_CHANGES_v2.md
- [X] T031 [P] [US2] Remove parser references from `docs/` directory files identified in T009
- [X] T032 [P] [US2] Update any example code in documentation to use processor/router instead of parser
- [X] T033 [US2] Verify BREAKING_CHANGES_v2.md is comprehensive and clear (peer review or self-review)
- [X] T034 [US2] Verify all documentation links work and point to correct locations

**Checkpoint**: Comprehensive breaking changes documentation complete, old migration doc removed, all references updated

---

## Phase 5: User Story 3 - Simplified Testing and Validation (Priority: P3)

**Goal**: Test suite is simplified without backward compatibility scenarios, reducing complexity and execution time

**Independent Test**: Can be tested by comparing test suite complexity before and after removal, measuring reduction in test cases and scenarios

### Implementation for User Story 3

- [X] T035 [P] [US3] Delete parser-specific test files from `tests/` directory (any files testing deprecated parser functions)
- [X] T036 [US3] Remove parser test imports from integration test files
- [X] T037 [US3] Run test suite and record new test count (`go test ./... -v | grep -c "^=== RUN"`)
- [X] T038 [US3] Run test suite and record new execution time (`time go test ./... -race -count=1`)
- [X] T039 [US3] Compare baseline metrics (T005) with new metrics - document reduction in test count and execution time
- [ ] T040 [US3] Verify test coverage still meets thresholds (>= 75% overall, >= 90% core) — current coverage 50.4% overall / 90.2% core
- [X] T041 [US3] Update test documentation to reflect simplified test scope (no legacy parser tests)
- [X] T042 [US3] Document test improvements in `specs/006-delete-the-backward/test-metrics.md`

**Checkpoint**: Test suite simplified, metrics show improvement, coverage maintained

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final verification and quality assurance

- [X] T043 [P] Run constitution compliance check: Verify GOV-06 (Innovation Over Compatibility) is satisfied
- [X] T044 [P] Run Makefile standards target (`make standards`) to verify vet and test pass
- [X] T045 [P] Verify no TODO/FIXME comments referencing deprecated parser remain (`grep -r "TODO.*parser" . --include="*.go"`)
- [X] T046 Run final comprehensive audit: no parser imports, version is 2.0.0, docs updated
- [X] T047 Review all changes with `git diff` and `git status` to ensure no unintended modifications
- [X] T048 Run quickstart.md validation steps (manual walkthrough of implementation guide)
- [X] T049 Update `specs/006-delete-the-backward/plan.md` with actual implementation notes if needed
- [X] T050 Prepare commit message with breaking change notice and co-authorship
- [X] T051 Stage all changes (`git add -A`)
- [X] T052 Commit with breaking change message: "feat: remove backward compatibility code for v2.0.0"
- [ ] T053 Create git tag `v2.0.0` after merging to main branch

**Checkpoint**: All changes reviewed, committed, ready for release

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational - Must complete before US2 or US3
- **User Story 2 (Phase 4)**: Depends on US1 completion (needs to know what was removed)
- **User Story 3 (Phase 5)**: Depends on US1 completion (needs parser removal to measure impact)
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - BLOCKING for US2 and US3
  - Rationale: Must remove code before documenting removal and measuring test impact
- **User Story 2 (P2)**: Depends on User Story 1 completion
  - Rationale: Cannot document breaking changes until code is actually removed
- **User Story 3 (P3)**: Depends on User Story 1 completion
  - Rationale: Cannot measure test simplification until parser tests are removed

### Within Each User Story

**User Story 1 (Critical Path)**:
1. Remove internal usages (T011-T012) - can be parallel
2. Delete parser directory (T013)
3. Update protocol version (T015-T016) - can be parallel with T013
4. Verify removal (T017)
5. Build and test (T018-T021) - sequential verification

**User Story 2**:
1. Create breaking changes doc (T022-T028) - sequential (same file)
2. Delete old migration doc (T029) - parallel with doc updates
3. Update README and docs (T030-T032) - can be parallel
4. Review and verify (T033-T034) - sequential

**User Story 3**:
1. Delete parser tests (T035-T036) - can be parallel
2. Measure metrics (T037-T039) - sequential
3. Verify coverage (T040-T041) - sequential
4. Document improvements (T042)

### Parallel Opportunities

**Phase 1 (Setup)**: T002, T003, T004 can run in parallel

**Phase 2 (Foundational)**: T007, T008, T009 can run in parallel after T006

**User Story 1**: T011 and T012 can run in parallel; T015-T016 can run in parallel with T013

**User Story 2**: T031 and T032 can run in parallel

**User Story 3**: T035 and T036 can run in parallel

**Polish Phase**: T043, T044, T045 can run in parallel

---

## Parallel Example: User Story 1 (Core Removal)

```bash
# Remove internal usages in parallel:
Task 1: "Remove internal usage of parser in test files - migrate to processor/router"
Task 2: "Remove internal usage of parser in source files"

# After deletion, verify and update in parallel:
Task 1: "Delete parser directory and verify"
Task 2: "Update protocol version constant to 2.0.0"
```

## Parallel Example: User Story 2 (Documentation)

```bash
# Update documentation in parallel:
Task 1: "Remove parser references from docs/ directory"
Task 2: "Update example code to use processor/router"
Task 3: "Delete old MIGRATION.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T005) - ~15 minutes
2. Complete Phase 2: Foundational Audit (T006-T010) - ~1 hour
3. Complete Phase 3: User Story 1 (T011-T021) - ~2 hours
4. **STOP and VALIDATE**: Verify codebase has zero backward compatibility code, all tests pass
5. This is a deployable state with core objective achieved

### Incremental Delivery

1. **Foundation**: Setup + Audit → ~1.5 hours
2. **MVP**: Add User Story 1 → Test independently → Ready for v2.0.0-alpha release
3. **Documentation**: Add User Story 2 → Complete migration guide → Ready for v2.0.0-beta
4. **Quality**: Add User Story 3 → Measure improvements → Ready for v2.0.0 final
5. **Release**: Polish phase → Commit and tag → v2.0.0 released

### Sequential Strategy (Recommended)

User stories MUST be completed sequentially in this feature:

1. Complete Setup + Foundational
2. Complete User Story 1 (REQUIRED first - removes the code)
3. Complete User Story 2 (documents what was removed in US1)
4. Complete User Story 3 (measures impact of US1)
5. Complete Polish phase

**Total Estimated Time**: 5-7 hours

---

## Task Count Summary

- **Phase 1 (Setup)**: 5 tasks
- **Phase 2 (Foundational)**: 5 tasks
- **Phase 3 (User Story 1)**: 11 tasks
- **Phase 4 (User Story 2)**: 13 tasks
- **Phase 5 (User Story 3)**: 8 tasks
- **Phase 6 (Polish)**: 11 tasks

**Total**: 53 tasks

**Parallel Opportunities**: 12 tasks can run in parallel with others (marked [P])

**Critical Path**: Phase 1 → Phase 2 → User Story 1 (Phase 3) → User Story 2 (Phase 4) → User Story 3 (Phase 5) → Polish (Phase 6)

---

## Verification Checklist

After completing all tasks, verify:

- [X] ✅ No `market_data/framework/parser/` directory exists
- [X] ✅ `core.ProtocolVersion == "2.0.0"`
- [X] ✅ No grep matches for parser imports in source code
- [X] ✅ `go build ./...` succeeds with no errors
- [X] ✅ `go test ./... -race -count=1` passes 100%
- [ ] ✅ Test coverage >= 75% overall, >= 90% for core packages (current: 50.4% overall / 90.2% core)
- [X] ✅ `BREAKING_CHANGES_v2.md` exists with comprehensive guide
- [X] ✅ `market_data/MIGRATION.md` deleted
- [X] ✅ `README.md` has breaking changes notice
- [X] ✅ Documentation updated to remove parser references
- [X] ✅ Test metrics show reduction in test count and execution time
- [X] ✅ Constitution compliance (GOV-06) verified
- [X] ✅ Changes committed with breaking change message

---

## Notes

- [P] tasks = different files/parallel execution possible
- [Story] label maps task to specific user story (US1, US2, US3)
- User stories MUST be done sequentially for this feature (US1 blocks US2 and US3)
- Each user story delivers measurable value:
  - US1: Clean codebase (MVP)
  - US2: Complete documentation
  - US3: Verified improvements
- Stop at any checkpoint to validate progress
- This is a breaking change release - extensive testing required
- Follow GOV-06: Innovation Over Compatibility principle
