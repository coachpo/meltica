# Implementation Plan: Remove Backward Compatibility Code

**Branch**: `006-delete-the-backward` | **Date**: 2025-01-10 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/006-delete-the-backward/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Remove ALL backward compatibility code across all previous versions in a single release to create a codebase with only one current implementation. This includes eliminating the deprecated `market_data/framework/parser` package, removing migration documentation, and ensuring the system runs exclusively on the modern processor/router architecture introduced in protocol version 1.2.0. The removal enables maximum architectural simplification and fastest development velocity for new features.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.25  
**Primary Dependencies**: github.com/goccy/go-json (JSON), github.com/gorilla/websocket (WebSocket), github.com/stretchr/testify (testing)  
**Storage**: N/A (SDK/library project with no persistent storage)  
**Testing**: Go's built-in testing with `go test ./... -race -count=1`  
**Target Platform**: Cross-platform Go library (Linux, macOS, Windows)
**Project Type**: Single SDK/library project  
**Performance Goals**: Maintain current P99 < 10ms WebSocket message latency, zero allocations in hot paths  
**Constraints**: Must maintain 75%+ overall test coverage, 90%+ for core packages per TS-01  
**Scale/Scope**: SDK library with ~50k LOC, supporting multiple exchange adapters

**Backward Compatibility Code Identified**:
- `market_data/framework/parser/` - Entire deprecated package (3 files: doc.go, json_pipeline.go, validation_stage.go)
- `market_data/MIGRATION.md` - Legacy migration guide
- All references to deprecated parser in existing code and documentation

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### GOV-06: Innovation Over Compatibility ✅ ALIGNED

**Analysis**: This feature directly implements GOV-06 by removing backward compatibility code to enable cleaner architecture and faster development. The constitution explicitly states: "Prioritize shipping new capabilities and architectural improvements over preserving backward compatibility" and "When trade-offs arise, select the path that unlocks new capabilities even if it requires a breaking change."

**Breaking Changes**:
- Removal of entire `market_data/framework/parser` package
- Users still relying on deprecated parser APIs must migrate to processor/router architecture
- Major version bump required (1.2.0 → 2.0.0)

**Migration Documentation**:
- Current `market_data/MIGRATION.md` provides migration path from parser to processor
- Will be replaced with comprehensive breaking changes documentation
- Release notes will clearly communicate no backward compatibility

**Justification**: Removing deprecated code after migration path was provided aligns with innovation-first principle. The modern processor/router architecture is already in place and fully functional.

✅ **GATE PASSED** - Work directly implements constitutional principle GOV-06

### Re-check After Phase 1 Design

**Status**: ✅ STILL ALIGNED

The completed design maintains full alignment with GOV-06:
- Research confirms clean removal strategy
- Data model identifies all backward compatibility code
- Quickstart provides systematic removal process
- Breaking changes documentation ensures user communication
- No new complexity introduced
- All quality gates satisfied

✅ **FINAL GATE PASSED** - Design and implementation approach fully supports constitutional principle

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

```
# Go SDK/Library Structure
core/                          # Core interfaces and types (frozen per CQ-08)
├── exchanges/
│   ├── bootstrap/            # Protocol validation (strict version matching)
│   └── capabilities/         # Exchange capability declarations

market_data/
├── framework/
│   ├── parser/              # ⚠️ DEPRECATED - TO BE REMOVED
│   ├── router/              # Current routing implementation
│   └── handler/             # Handler registry
├── processors/              # Modern processor implementations
└── MIGRATION.md             # ⚠️ TO BE REMOVED/REPLACED

exchanges/                    # Exchange-specific adapters
├── binance/
├── shared/
│   ├── plugin/
│   ├── mock/
│   └── infra/

pipeline/                     # Multi-exchange coordination
internal/                     # Internal utilities
errs/                        # Typed error handling
cmd/                         # Command-line tools
tests/                       # Integration and E2E tests
```

**Structure Decision**: Go SDK with layered architecture (Transport L1, Routing L2, Exchange L3, Pipeline L4). The backward compatibility removal focuses on eliminating the deprecated `market_data/framework/parser/` package and updating all references to use the current `market_data/processors/` and `market_data/framework/router/` architecture.

## Complexity Tracking

*No violations - Constitution Check passed. This section is not applicable.*

---

## Planning Phase Completion Summary

**Status**: ✅ COMPLETE (Phase 0 & Phase 1)

**Generated Artifacts**:
- ✅ `plan.md` - This file (Technical Context, Constitution Check, Project Structure)
- ✅ `research.md` - Complete inventory and removal strategies
- ✅ `data-model.md` - Affected code artifacts and documentation
- ✅ `contracts/README.md` - Removal verification contracts
- ✅ `quickstart.md` - Step-by-step implementation guide
- ✅ `CLAUDE.md` - Updated agent context

**Key Decisions**:
1. **Scope**: Remove `market_data/framework/parser/` package (3 files) and `MIGRATION.md`
2. **Strategy**: Audit → cleanup → delete → verify (safe removal sequence)
3. **Version**: Bump to 2.0.0 per SemVer for breaking changes
4. **Documentation**: Create `BREAKING_CHANGES_v2.md` with comprehensive migration guide
5. **Error Handling**: Rely on compile-time errors with clear documentation

**Constitution Compliance**: ✅ Fully aligned with GOV-06 (Innovation Over Compatibility)

**Next Command**: `/speckit.tasks` to generate detailed implementation tasks

**Estimated Implementation Effort**: 5-7 hours total

## Implementation Notes (Post-Execution)

- Removed the entire `market_data/framework/parser` package and introduced inline decoding with pooled JSON decoders inside `session_runtime`.
- Added `invalidTracker` for threshold handling, wiring `DialOptions.InvalidThreshold` into runtime validation and heartbeat flow.
- Migrated documentation by creating `BREAKING_CHANGES_v2.md`, deleting the legacy `market_data/MIGRATION.md`, and updating README and troubleshooting guides.
- Simplified test suite by dropping parser-specific cases, recording a reduction from 271 to 269 verbose test runs and trimming cumulative runtime by ~3.8s.
