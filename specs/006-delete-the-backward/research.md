# Phase 0: Research - Remove Backward Compatibility Code

**Feature**: Remove Backward Compatibility Code  
**Branch**: `006-delete-the-backward`  
**Date**: 2025-01-10

## Research Questions

1. What is the complete inventory of deprecated code to remove?
2. What are the best practices for safely removing deprecated code?
3. How should version information be updated for breaking changes?
4. What documentation strategy communicates the breaking release effectively?
5. How should error messages guide users away from removed code?

---

## R1: Complete Inventory of Deprecated Code

### Decision

Remove the following code artifacts completely:

**Files to Delete**:
- `market_data/framework/parser/doc.go`
- `market_data/framework/parser/json_pipeline.go`
- `market_data/framework/parser/validation_stage.go`
- `market_data/MIGRATION.md`

**Directory to Delete**:
- `market_data/framework/parser/` (entire package)

**References to Clean**:
- Import statements referencing `market_data/framework/parser`
- Documentation references to deprecated parser
- Test files importing or testing deprecated parser functions
- Any example code using the old parser API

### Rationale

The codebase analysis identified the `market_data/framework/parser` package as the primary backward compatibility layer. All functions in this package are marked with `// Deprecated:` comments directing users to the modern `market_data/processors` and `framework/router` architecture. The MIGRATION.md file serves only to guide users from the old system to the new one.

### Alternatives Considered

**Keep parser package but remove implementation**: Rejected because it creates confusion - users would see the package but get runtime errors. Clean removal is clearer.

**Gradual removal over multiple releases**: Rejected per clarification decisions - immediate removal in single release is the chosen approach.

---

## R2: Safe Deprecation Removal Strategy

### Decision

Follow this removal sequence:

1. **Audit all imports**: Use `go list -deps` and grep to find all packages importing deprecated code
2. **Remove internal usages first**: Clean up any remaining internal code using deprecated APIs
3. **Delete deprecated code**: Remove entire `parser/` directory and `MIGRATION.md`
4. **Update tests**: Remove tests for deprecated code, ensure tests using deprecated APIs are updated or removed
5. **Verify compilation**: Run `go build ./...` to ensure no broken imports remain
6. **Verify tests**: Run `go test ./...` to ensure all tests pass with deprecated code removed

### Rationale

This sequence minimizes the risk of breaking the build. By auditing imports first, we understand the full impact. Removing internal usages before deletion prevents orphaned code that won't compile. The verification steps ensure the codebase remains functional.

### Alternatives Considered

**Delete first, fix compilation errors**: Rejected because it's harder to track what needs updating and risks missing subtle issues.

**Use build tags to conditionally compile**: Rejected because it adds complexity and doesn't fully remove the code.

---

## R3: Version Bump Strategy for Breaking Changes

### Decision

Update version from `1.2.0` to `2.0.0` following Semantic Versioning (SemVer):

**Changes Required**:
- Update `core.ProtocolVersion` constant from `"1.2.0"` to `"2.0.0"` in `core/core.go`
- All exchange adapters automatically inherit this version via `core.ProtocolVersion` constant
- Update `go.mod` version if tagged
- Create git tag `v2.0.0` when releasing

**Major Version Rationale**: Removing an entire public package (`market_data/framework/parser`) is a breaking change requiring major version bump per SemVer rules.

### Rationale

The project uses strict protocol version matching (`core/exchanges/bootstrap/protocol.go` enforces exact match). A major version bump clearly signals breaking changes to users and prevents accidental upgrades without migration.

### Alternatives Considered

**Use version 1.3.0 with "deprecated removal" notice**: Rejected because SemVer requires major bump for breaking changes. Minor version changes should be backward compatible.

**Skip protocol version update**: Rejected because the constitution requires version updates (GOV-06 implementation notes specify "protocol version updates").

---

## R4: Breaking Release Documentation Strategy

### Decision

Create comprehensive breaking changes documentation:

**New Document**: `BREAKING_CHANGES_v2.md` in repository root containing:
- Summary of removed code
- Rationale for removal
- Migration path from any v1.x version to v2.0
- Code examples showing old vs new approach
- Timeline and deprecation history

**Update Existing Docs**:
- `README.md`: Add prominent notice about v2.0 breaking changes
- Release notes: Clear markers indicating major version with no backward compatibility
- `docs/` folder: Remove all references to deprecated parser package

**Migration Documentation Strategy**:
- Transform existing `market_data/MIGRATION.md` content into breaking changes guide
- Add "upgrading from v1.x to v2.0" section
- Include examples for each removed function showing modern equivalent

### Rationale

Clear communication is critical per FR-003 and FR-010. Users need comprehensive documentation to migrate successfully. Preserving migration examples from the existing MIGRATION.md ensures users have actionable guidance.

### Alternatives Considered

**Only release notes, no dedicated doc**: Rejected because breaking changes warrant detailed documentation beyond release notes.

**Remove MIGRATION.md without replacement**: Rejected because users need migration guidance per FR-004.

---

## R5: Error Messages for Removed Code

### Decision

**Strategy**: Since the code is completely removed, compile-time errors will naturally occur for users attempting to import the package.

**Go compiler will provide**:
```
cannot find package "github.com/coachpo/meltica/market_data/framework/parser" in any of...
```

**Enhancement**: Add a clear notice in the removal commit message and documentation directing users to the breaking changes guide.

**No runtime error messages needed**: Because the package is deleted entirely, users cannot compile code that imports it. This is actually better than runtime errors because issues are caught immediately at build time, not in production.

### Rationale

Go's compile-time type system provides automatic error detection. Compile errors are preferable to runtime errors for deprecated API removal. Documentation and release notes will provide the necessary context.

### Alternatives Considered

**Keep package with stub functions that panic with helpful messages**: Rejected because it violates the "single implementation path" requirement from clarifications. Complete removal is clearer.

**Redirect imports via type aliases**: Rejected because deprecated code has no modern equivalent with the same API - the architecture changed fundamentally.

---

## Summary

All research questions resolved with actionable decisions:

1. **Inventory**: `market_data/framework/parser/` package (3 files) and `market_data/MIGRATION.md`
2. **Strategy**: Audit imports → remove internal usage → delete code → verify build/tests
3. **Version**: Bump to 2.0.0 per SemVer for breaking changes
4. **Documentation**: Create `BREAKING_CHANGES_v2.md` with comprehensive migration guide
5. **Error Handling**: Rely on compile-time errors with clear documentation pointers

No additional research needed. Ready for Phase 1 design.
