# API Contracts

**Feature**: Remove Backward Compatibility Code  
**Branch**: `006-delete-the-backward`

## Overview

This feature involves code removal and does not introduce new APIs or contracts. The "contracts" for this feature are the verification steps that ensure backward compatibility code has been completely removed.

## Removal Contracts

### Contract 1: No Parser Package Imports

**Verification**:
```bash
# Must return no results in source files
grep -r "market_data/framework/parser" --include="*.go" --exclude-dir=".git" .
```

**Expected**: Exit code 0, no matches

---

### Contract 2: Protocol Version Updated

**Verification**:
```bash
# Must show version 2.0.0
grep "ProtocolVersion = " core/core.go
```

**Expected Output**:
```go
const ProtocolVersion = "2.0.0"
```

---

### Contract 3: Parser Package Deleted

**Verification**:
```bash
# Must not exist
ls market_data/framework/parser/
```

**Expected**: Directory not found error

---

### Contract 4: All Tests Pass

**Verification**:
```bash
go test ./... -race -count=1
```

**Expected**: All tests pass, coverage >= 75% overall, >= 90% for core packages

---

### Contract 5: Build Succeeds

**Verification**:
```bash
go build ./...
```

**Expected**: Clean build with no compilation errors

---

### Contract 6: Breaking Changes Documentation Exists

**Verification**:
```bash
ls -la BREAKING_CHANGES_v2.md
```

**Expected**: File exists with comprehensive migration documentation

---

## Summary

Rather than API contracts, this feature defines "removal contracts" - verification steps that prove backward compatibility code has been completely eliminated while maintaining system functionality.
