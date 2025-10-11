# Research: Four-Layer Architecture Implementation

**Date**: 2025-01-20  
**Feature**: 008-architecture-requirements-req

## Overview

This document consolidates research findings for implementing formal layer interfaces, static analysis enforcement, and type-specific interface support for the existing four-layer architecture.

---

## Decision 1: Static Analysis Tool Selection

**Question**: Which Go static analysis framework should be used to enforce layer boundary violations?

**Decision**: Use `golang.org/x/tools/go/analysis` framework with custom analyzer

**Rationale**:
- Standard Go toolchain integration - works with `go vet`, `golangci-lint`
- Well-documented API for building custom analyzers
- Supports SSA (Static Single Assignment) for deep analysis if needed
- Can analyze import graphs and detect cross-layer violations
- Already a project dependency

**Alternatives Considered**:
- **go/ast directly**: Lower-level, requires more boilerplate, harder to integrate with existing tools
- **Third-party tools (e.g., go-arch-lint)**: Less flexible, may not support our specific layer rules
- **golangci-lint plugin**: Requires analyzer framework anyway; better to build analyzer first

**Implementation Approach**:
1. Create analyzer in `internal/linter/analyzer.go`
2. Define layer rules mapping packages to layers
3. Check import statements against allowed dependencies
4. Report violations with clear error messages including fix suggestions
5. Integrate with Makefile via `go vet -vettool=...`

**References**:
- https://pkg.go.dev/golang.org/x/tools/go/analysis
- https://github.com/golang/tools/blob/master/go/analysis/doc.go

---

## Decision 2: Interface Definition Pattern

**Question**: How should layer interfaces be structured to support both common functionality and type-specific (spot/futures/options) requirements?

**Decision**: Composition pattern with base interfaces + type-specific extensions

**Rationale**:
- Go's interface composition allows building type-specific interfaces from common ones
- Avoids duplication while maintaining type safety
- Implementations can satisfy multiple interfaces
- Clear naming convention: `Connection`, `SpotConnection`, `FuturesConnection`

**Pattern**:
```go
// Base layer interfaces (common to all market types)
type Connection interface {
    Connect(ctx context.Context) error
    Close() error
    // ... common connection methods
}

type Routing interface {
    Subscribe(ctx context.Context, req SubscriptionRequest) error
    Unsubscribe(ctx context.Context, req SubscriptionRequest) error
    // ... common routing methods
}

// Type-specific extensions
type SpotConnection interface {
    Connection  // Embed base interface
    GetSpotEndpoint() string
    // ... spot-specific methods
}

type FuturesConnection interface {
    Connection
    GetFuturesEndpoint() string
    GetContractDetails() ContractInfo
    // ... futures-specific methods
}
```

**Alternatives Considered**:
- **Single large interface**: Violates Interface Segregation Principle, forces implementations to stub unused methods
- **Separate hierarchies**: More duplication, harder to maintain common behavior
- **Generic interfaces**: Go 1.25 generics available but add complexity; composition is more idiomatic

---

## Decision 3: Cross-Cutting Concerns Implementation

**Question**: How should global accessors for logging, metrics, and error handling be implemented?

**Decision**: Package-level variables with initialization functions and optional interface injection

**Rationale**:
- Matches clarification decision (global singletons/static accessors)
- Simple to use from any layer without explicit dependencies
- Supports testing via injection of mock implementations
- Common Go pattern (e.g., `log`, `http.DefaultClient`)

**Pattern**:
```go
// In internal/observability/logger.go
package observability

var defaultLogger Logger = &noopLogger{}

type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
}

func SetLogger(l Logger) {
    defaultLogger = l
}

func Log() Logger {
    return defaultLogger
}

// Usage in any layer:
// observability.Log().Info("message", Field("key", value))
```

**Alternatives Considered**:
- **Context.Context propagation**: More complex, requires passing context everywhere, still need globals for initialization
- **Dependency injection**: Violates clarification decision, creates explicit layer dependencies
- **Framework-specific (e.g., zap.L())**: Locks into specific framework, less testable

---

## Decision 4: Layer Boundary Rule Encoding

**Question**: How should layer dependency rules be encoded for static analysis?

**Decision**: Declarative rule map in analyzer with package path prefixes

**Rationale**:
- Easy to understand and maintain
- Supports package hierarchies (e.g., `exchanges/*/infra/*` → Layer 1)
- Can be loaded from config file if needed in future
- Clear error messages from violated rules

**Implementation**:
```go
// Layer definitions
const (
    LayerConnection = "connection"  // L1
    LayerRouting    = "routing"     // L2
    LayerBusiness   = "business"    // L3
    LayerFilter     = "filter"      // L4
)

// Package to layer mapping (glob patterns)
var layerRules = map[string]string{
    "*/infra/*":     LayerConnection,
    "*/routing/*":   LayerRouting,
    "*/bridge/*":    LayerBusiness,
    "*/filter/*":    LayerBusiness, // Some business logic here
    "pipeline/*":    LayerFilter,
    "core/layers/*": "interface",   // Interfaces can be used by anyone
}

// Allowed dependencies (from → to)
var allowedDependencies = map[string][]string{
    LayerFilter:     {LayerBusiness, LayerRouting, LayerConnection, "interface"},
    LayerBusiness:   {LayerRouting, LayerConnection, "interface"},
    LayerRouting:    {LayerConnection, "interface"},
    LayerConnection: {"interface"},
}
```

**References**:
- Go package path matching: https://pkg.go.dev/path/filepath#Match

---

## Decision 5: Migration Strategy for Existing Code

**Question**: What is the specific sequence for migrating existing code to use layer interfaces?

**Decision**: Bottom-up migration starting with Connection layer, using interface wrappers initially

**Rationale**:
- Connection layer (L1) has fewest dependencies
- Each layer can be tested independently after migration
- Wrapper pattern allows gradual transition without breaking existing code
- Matches clarification: "new interfaces replace legacy components"

**Migration Sequence**:
1. **Phase 1**: Define all layer interfaces in `core/layers/`
2. **Phase 2**: Create wrapper implementations in each exchange (implement interface, delegate to existing code)
3. **Phase 3**: Migrate Connection layer (L1) - `exchanges/*/infra/*`
   - Implement `Connection` interface
   - Replace direct struct usage with interface types in L2
   - Update tests to use mock interfaces
4. **Phase 4**: Migrate Routing layer (L2) - `exchanges/*/routing/*`
   - Implement `Routing` interface
   - Update L3 to use interface types
5. **Phase 5**: Migrate Business layer (L3) - `exchanges/*/bridge/*`
   - Implement `Business` interface  
   - Update L4 (pipeline) to use interface types
6. **Phase 6**: Migrate Filter layer (L4) - `pipeline/*`
   - Implement `Filter` interface
   - Update consumers to use interface types
7. **Phase 7**: Enable static analysis tool in CI

**Incremental Testing**:
- Each phase includes passing existing tests with wrappers
- New interface-based tests added alongside
- No phase proceeds until previous phase tests pass

---

## Decision 6: Interface Contract Testing

**Question**: How to ensure all implementations satisfy interface contracts beyond type checking?

**Decision**: Behavioral contract tests using table-driven approach

**Rationale**:
- Go type system ensures method signatures match
- Behavioral tests ensure semantic contracts (e.g., "Connect must be idempotent")
- Table-driven tests allow testing multiple implementations against same contract
- Documents expected behavior for implementers

**Pattern**:
```go
// In tests/architecture/interface_contracts_test.go
type ConnectionContractTest struct {
    name  string
    setup func() layers.Connection
}

func TestConnectionContract(t *testing.T) {
    tests := []ConnectionContractTest{
        {"Binance WS", func() layers.Connection { return binanceWS.New(...) }},
        {"Binance REST", func() layers.Connection { return binanceREST.New(...) }},
        // Add new implementations here
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            conn := tt.setup()
            
            // Test idempotency
            err1 := conn.Connect(ctx)
            err2 := conn.Connect(ctx) // Second call should succeed or be no-op
            assert.NoError(t, err2)
            
            // Test cleanup
            err := conn.Close()
            assert.NoError(t, err)
            
            // ... more contract tests
        })
    }
}
```

---

## Decision 7: Documentation Strategy

**Question**: How to document layer responsibilities and interface contracts?

**Decision**: Multi-tier documentation approach

**Rationale**:
- Different audiences need different detail levels
- Code comments for developers working with interfaces
- Architecture docs for onboarding and high-level understanding
- Enforces documentation culture

**Documentation Tiers**:

1. **Interface godoc comments** (in code):
   ```go
   // Connection manages the lifecycle of network connections to exchanges.
   // Implementations must be safe for concurrent use.
   // 
   // Contract guarantees:
   // - Connect is idempotent (multiple calls safe)
   // - Close releases all resources
   // - Close can be called multiple times safely
   type Connection interface { ... }
   ```

2. **Architecture guide** (specs/008-architecture-requirements-req/quickstart.md):
   - Layer responsibilities
   - Dependency rules
   - Migration guide for new developers

3. **Contract test documentation** (inline in contract tests):
   - Explains WHY each behavioral test exists
   - Links to relevant edge cases from spec

**References**:
- Effective Go: https://go.dev/doc/effective_go#commentary
- Go Code Review Comments: https://github.com/golang/go/wiki/CodeReviewComments#doc-comments

---

## Summary of Key Decisions

| Decision | Choice | Impact |
|----------|--------|--------|
| Static Analysis | golang.org/x/tools/go/analysis | CI-integrated, extensible |
| Interface Pattern | Composition with type-specific extensions | Flexible, type-safe |
| Cross-Cutting | Package-level singletons | Simple, testable |
| Layer Rules | Declarative map with glob patterns | Clear, maintainable |
| Migration Strategy | Bottom-up with wrappers | Incremental, low-risk |
| Contract Testing | Table-driven behavioral tests | Comprehensive, reusable |
| Documentation | Multi-tier (code/architecture/tests) | Accessible to all audiences |

---

## Next Steps

With research complete, proceed to Phase 1:
1. Define concrete layer interfaces in `core/layers/`
2. Document data model for layer interactions
3. Create API contracts (Go interfaces are the contracts)
4. Generate quickstart guide for developers
