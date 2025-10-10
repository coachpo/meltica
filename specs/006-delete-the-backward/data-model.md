# Phase 1: Data Model - Remove Backward Compatibility Code

**Feature**: Remove Backward Compatibility Code  
**Branch**: `006-delete-the-backward`  
**Date**: 2025-01-10

## Overview

This feature involves code removal and documentation updates rather than data modeling. However, we document the key "entities" (code artifacts and documentation) that are affected by this change.

## Affected Code Artifacts

### 1. Deprecated Package: `market_data/framework/parser`

**Status**: TO BE DELETED

**Components**:
- `doc.go` - Package documentation declaring deprecation
- `json_pipeline.go` - Legacy JSON parsing pipeline (deprecated)
- `validation_stage.go` - Legacy validation framework (deprecated)

**Current Purpose**: Backward compatibility layer for users who haven't migrated to the processor/router architecture

**Relationships**:
- **Superseded by**: `market_data/processors/` package (modern implementation)
- **Related to**: `market_data/framework/router/` dispatcher

**Impact**: Any code importing this package will fail to compile after removal

---

### 2. Migration Document: `market_data/MIGRATION.md`

**Status**: TO BE REPLACED

**Current Content**:
- Migration guide from parser pipeline to router processors
- Checklist for migration steps
- Validation commands

**Transformation**:
- Content will be incorporated into `BREAKING_CHANGES_v2.md`
- Document will be deleted after content migration

---

### 3. Protocol Version: `core.ProtocolVersion`

**Status**: TO BE UPDATED

**Current Value**: `"1.2.0"`  
**New Value**: `"2.0.0"`

**Location**: `core/core.go:14`

**Impact**:
- All exchange adapters reference this constant
- Bootstrap validation enforces strict version matching
- Breaking change signals to all integrators

**Related Code**:
```go
// core/core.go
const ProtocolVersion = "1.2.0"  // Will become "2.0.0"

// core/exchanges/bootstrap/protocol.go
func ValidateProtocol(exchange core.Exchange) error {
    version := exchange.SupportedProtocolVersion()
    if version != core.ProtocolVersion {
        return fmt.Errorf("protocol mismatch: %s != %s", version, core.ProtocolVersion)
    }
    return nil
}
```

---

## Documentation Artifacts

### 4. Breaking Changes Documentation

**Status**: TO BE CREATED

**File**: `BREAKING_CHANGES_v2.md` (repository root)

**Content Structure**:
```markdown
# Breaking Changes in v2.0.0

## Overview
- Complete removal of backward compatibility code
- Parser package removed
- Migration required for all users

## Removed Components
- market_data/framework/parser package
- All deprecated parser functions

## Migration Guide
- From parser to processor
- Code examples (old vs new)
- Step-by-step instructions

## Version History
- v1.2.0: Processor architecture introduced, parser deprecated
- v2.0.0: Parser removed completely
```

**Source Content**: Derived from existing `market_data/MIGRATION.md`

---

### 5. Release Documentation

**Status**: TO BE UPDATED

**Affected Files**:
- `README.md` - Add prominent v2.0 breaking changes notice
- Release notes (to be created) - Document all breaking changes
- `docs/` folder - Remove deprecated parser references

**Key Messages**:
- No backward compatibility in v2.0
- Immediate migration required
- Link to `BREAKING_CHANGES_v2.md`

---

## Import Dependency Graph

```
User Code
    │
    ├─> market_data/framework/parser  [TO BE REMOVED]
    │       ├─> NewJSONPipeline()
    │       ├─> NewValidationStage()
    │       └─> ValidatorFunc
    │
    └─> market_data/processors         [CURRENT, KEEP]
            ├─> Processor interface
            └─> framework/router
                    └─> Dispatcher
```

**Migration Path**: Users must update imports from `parser` package to `processors` package and refactor code to use the Processor interface.

---

## Version Evolution Timeline

| Version | Status | Parser Package | Migration Doc | Protocol |
|---------|--------|----------------|---------------|----------|
| 1.0.0 - 1.1.x | Historical | Active | N/A | < 1.2.0 |
| 1.2.0 | Current | Deprecated | Added | 1.2.0 |
| 2.0.0 | Target | Removed | Transformed to Breaking Changes | 2.0.0 |

---

## Validation Rules

After removal, the following must be true:

1. **No parser imports**: `grep -r "market_data/framework/parser" .` returns no results in source code
2. **Version updated**: `core.ProtocolVersion == "2.0.0"`
3. **Tests pass**: `go test ./... -race -count=1` returns 0 failures
4. **Build succeeds**: `go build ./...` completes without errors
5. **Coverage maintained**: Overall coverage remains >= 75%, core packages >= 90%
6. **Documentation complete**: `BREAKING_CHANGES_v2.md` exists with comprehensive migration guide

---

## Summary

This data model documents the artifacts affected by backward compatibility removal:

- **Deleted**: `market_data/framework/parser/` package (3 files)
- **Deleted**: `market_data/MIGRATION.md`
- **Updated**: `core.ProtocolVersion` constant (1.2.0 → 2.0.0)
- **Created**: `BREAKING_CHANGES_v2.md` with migration guidance
- **Updated**: Various documentation files

The removal creates a clean break, leaving only the single current implementation (processor/router architecture) in the codebase.
