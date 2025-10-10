# Phase 1: Implementation Quickstart - Remove Backward Compatibility Code

**Feature**: Remove Backward Compatibility Code  
**Branch**: `006-delete-the-backward`  
**Date**: 2025-01-10

## Prerequisites

- Ensure you're on branch `006-delete-the-backward`
- Current implementation is on protocol version 1.2.0
- Modern processor/router architecture is fully functional
- Existing test suite is passing

## Implementation Sequence

This quickstart provides a step-by-step guide to safely remove all backward compatibility code.

---

## Step 1: Audit Import Usage

**Objective**: Identify all code that imports the deprecated parser package

**Commands**:
```bash
# Find all imports of deprecated parser
grep -r "market_data/framework/parser" --include="*.go" .

# List all packages with dependencies on parser
go list -f '{{.ImportPath}} {{.Imports}}' ./... | grep parser
```

**Expected Findings**:
- Potentially some test files
- Possibly some documentation examples
- Ideally, no production code (should already be migrated)

**Action**: Document all files that need updates before deletion

---

## Step 2: Remove Internal Usage

**Objective**: Clean up any remaining internal code using deprecated APIs

**Tasks**:
1. For each file identified in Step 1:
   - If it's a test, update to use processor/router architecture
   - If it's an example, rewrite using modern API
   - If it's production code, migrate to processor pattern

**Example Migration**:

**Before (Deprecated Parser)**:
```go
import "github.com/coachpo/meltica/market_data/framework/parser"

pipeline := parser.NewJSONPipeline()
validator := parser.NewValidationStage()
// ... old validation logic
```

**After (Modern Processor)**:
```go
import (
    "github.com/coachpo/meltica/market_data/processors"
    "github.com/coachpo/meltica/market_data/framework/router"
)

processor := &processors.TradeProcessor{}
dispatcher := router.NewDispatcher()
dispatcher.RegisterProcessor("trade", processor)
// ... routing logic
```

**Verification**:
```bash
go test ./... -race -count=1
```

---

## Step 3: Delete Deprecated Parser Package

**Objective**: Remove the entire deprecated parser directory

**Commands**:
```bash
# Delete parser package
rm -rf market_data/framework/parser/

# Verify deletion
ls market_data/framework/parser/  # Should error: no such directory
```

**Verify no broken imports**:
```bash
go build ./...
```

**Expected**: Build should fail if any remaining imports exist. If build succeeds, parser is successfully removed with no dependencies.

---

## Step 4: Update Protocol Version

**Objective**: Bump version from 1.2.0 to 2.0.0 for breaking release

**File**: `core/core.go`

**Change**:
```diff
- const ProtocolVersion = "1.2.0"
+ const ProtocolVersion = "2.0.0"
```

**Update Comment**:
```diff
- // ProtocolVersion declares the canonical protocol version supported by this repository.
- // Version 1.2.0 introduces WebSocket code organization refactoring and Channel Mapper architecture.
+ // ProtocolVersion declares the canonical protocol version supported by this repository.
+ // Version 2.0.0 removes all backward compatibility code; parser package deleted.
```

**Verification**:
```bash
grep "ProtocolVersion = " core/core.go
# Expected: const ProtocolVersion = "2.0.0"
```

---

## Step 5: Create Breaking Changes Documentation

**Objective**: Provide comprehensive migration guide for users

**File**: Create `BREAKING_CHANGES_v2.md` in repository root

**Content Template**:
```markdown
# Breaking Changes in v2.0.0

## Overview

Version 2.0.0 removes ALL backward compatibility code across all previous versions. The codebase now contains only a single current implementation using the processor/router architecture introduced in v1.2.0.

## What Was Removed

### Deprecated Parser Package

**Removed**: `market_data/framework/parser`

The entire parser package has been deleted, including:
- `NewJSONPipeline()` - Legacy JSON parsing
- `NewValidationStage()` - Legacy validation framework
- `ValidatorFunc` - Legacy validator type

### Migration Documentation

**Removed**: `market_data/MIGRATION.md`

The migration guide is no longer needed as the old parser is completely removed.

## Migration Guide

### Before (v1.x with deprecated parser)

```go
import "github.com/coachpo/meltica/market_data/framework/parser"

pipeline := parser.NewJSONPipeline()
validator := parser.NewValidationStage()
```

### After (v2.0 with processor/router)

```go
import (
    "github.com/coachpo/meltica/market_data/processors"
    "github.com/coachpo/meltica/market_data/framework/router"
)

processor := &processors.TradeProcessor{}
dispatcher := router.NewDispatcher()
dispatcher.RegisterProcessor("trade", processor)
```

### Step-by-Step Migration

1. **Identify Processor Type**: Determine which processor matches your use case (TradeProcessor, TickerProcessor, etc.)
2. **Instantiate Processor**: Create processor instance
3. **Register with Router**: Add processor to router dispatcher
4. **Forward Raw Messages**: Send raw WebSocket frames to dispatcher instead of pipeline
5. **Handle Typed Output**: Receive typed models from processor

## Version Requirements

- **Go**: 1.25 (unchanged)
- **Protocol**: 2.0.0 (breaking change)

## External Dependencies

External systems integrating with this library must update to v2.0 APIs immediately. No compatibility layer is provided.

## Support

For migration assistance, consult:
- Processor examples: `market_data/processors/`
- Router integration tests: `market_data/framework/router/*_test.go`
- Architecture docs: `docs/`

## Timeline

- **v1.2.0** (October 2024): Processor architecture introduced, parser deprecated
- **v2.0.0** (January 2025): Parser removed completely, no backward compatibility
```

---

## Step 6: Remove Old Migration Documentation

**Objective**: Delete the legacy migration guide as it's no longer relevant

**Command**:
```bash
rm market_data/MIGRATION.md
```

**Note**: The important migration information has been incorporated into `BREAKING_CHANGES_v2.md`

---

## Step 7: Update Documentation References

**Objective**: Remove all references to deprecated parser from documentation

**Files to Update**:

1. **README.md**: Add breaking changes notice
   ```markdown
   ## ⚠️ Breaking Changes in v2.0

   Version 2.0.0 removes all backward compatibility code. See [BREAKING_CHANGES_v2.md](./BREAKING_CHANGES_v2.md) for migration guide.
   ```

2. **Documentation in `docs/`**: Remove parser references
   ```bash
   grep -r "parser" docs/ --include="*.md"
   # Manually review and remove or update references
   ```

---

## Step 8: Verify and Test

**Objective**: Ensure system works correctly with all backward compatibility removed

**Verification Steps**:

1. **Build Check**:
   ```bash
   go build ./...
   # Expected: Clean build, no errors
   ```

2. **Test Execution**:
   ```bash
   go test ./... -race -count=1
   # Expected: All tests pass
   ```

3. **Coverage Check**:
   ```bash
   go test ./... -cover -coverprofile=coverage.out
   go tool cover -func=coverage.out | grep total
   # Expected: Total coverage >= 75%
   ```

4. **Import Audit**:
   ```bash
   grep -r "market_data/framework/parser" --include="*.go" .
   # Expected: No matches
   ```

5. **Version Verification**:
   ```bash
   grep "ProtocolVersion = " core/core.go
   # Expected: const ProtocolVersion = "2.0.0"
   ```

---

## Step 9: Final Quality Gates (Constitution Compliance)

**Verify Constitution Requirements**:

1. **CQ-01 through CQ-08**: No changes to core SDK interfaces ✅
2. **TS-01**: Coverage thresholds maintained ✅
3. **UX-01 through UX-06**: Error messages clear, APIs unchanged ✅
4. **PERF-01 through PERF-06**: Performance maintained (no regressions) ✅
5. **GOV-06**: Innovation over compatibility implemented ✅

**Run Standards Check**:
```bash
make standards
# Expected: vet passes, tests pass
```

---

## Step 10: Commit and Document

**Objective**: Commit the changes with clear message

**Git Commands**:
```bash
# Stage all changes
git add -A

# Review changes
git status
git diff --staged

# Commit with descriptive message
git commit -m "feat: remove backward compatibility code for v2.0.0

- Remove market_data/framework/parser package (deprecated)
- Remove market_data/MIGRATION.md (superseded)
- Update core.ProtocolVersion to 2.0.0
- Add BREAKING_CHANGES_v2.md with migration guide
- Update documentation to remove parser references

BREAKING CHANGE: Parser package removed. Users must migrate to processor/router architecture.

Co-authored-by: factory-droid[bot] <138933559+factory-droid[bot]@users.noreply.github.com>"

# Verify commit
git log --oneline -1
```

---

## Rollback Plan

If issues are discovered after removal:

1. **Revert the commit**:
   ```bash
   git revert HEAD
   ```

2. **Investigate failures**: Determine if there's undiscovered legacy code usage

3. **Fix and retry**: Update remaining legacy code and attempt removal again

---

## Success Criteria Checklist

- [ ] All parser package files deleted
- [ ] Protocol version updated to 2.0.0
- [ ] `BREAKING_CHANGES_v2.md` created with comprehensive migration guide
- [ ] `market_data/MIGRATION.md` removed
- [ ] Documentation updated to remove parser references
- [ ] All tests pass (`go test ./...`)
- [ ] Build succeeds (`go build ./...`)
- [ ] Coverage >= 75% overall, >= 90% for core packages
- [ ] No grep matches for parser imports in source code
- [ ] Constitution quality gates satisfied
- [ ] Changes committed with clear breaking change message

---

## Next Steps After Implementation

1. **Tag Release**: `git tag v2.0.0` after merging to main
2. **Publish Release Notes**: Include breaking changes prominently
3. **Communicate to Users**: Notify all integrators of breaking changes
4. **Monitor for Issues**: Watch for user reports needing migration assistance

---

## Estimated Effort

- **Step 1-2** (Audit & cleanup): 1-2 hours
- **Step 3-4** (Delete & version bump): 30 minutes
- **Step 5-7** (Documentation): 2-3 hours
- **Step 8-9** (Testing & verification): 1 hour
- **Step 10** (Commit): 15 minutes

**Total**: 5-7 hours

---

## Summary

This quickstart provides a safe, systematic approach to removing backward compatibility code:

1. Audit current usage
2. Clean up internal references
3. Delete deprecated code
4. Update version to 2.0.0
5. Create comprehensive migration documentation
6. Verify system integrity
7. Commit with clear breaking change message

Following these steps ensures a clean removal while maintaining system functionality and providing users with the guidance they need to migrate.
