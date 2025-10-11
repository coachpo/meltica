# Data Model: Four-Layer Architecture

**Date**: 2025-01-20  
**Feature**: 008-architecture-requirements-req

## Overview

This document defines the entities, relationships, and contracts for the four-layer architecture system.

---

## Core Entities

### 1. Layer

**Description**: A horizontal slice of the architecture with specific responsibilities and constraints.

**Attributes**:
- `Name` (string): Layer identifier - "Connection", "Routing", "Business", or "Filter"
- `Level` (int): Hierarchical level (1-4, where 1 is lowest)
- `Responsibilities` ([]string): List of layer responsibilities
- `AllowedDependencies` ([]Layer): Layers this layer can depend on

**Relationships**:
- One Layer `depends on` zero or more Layers (must be lower levels only)
- One Layer `contains` multiple Implementations

**Constraints**:
- Exactly 4 layers exist in the system
- Dependency graph must be acyclic
- Each layer can only depend on layers with lower level numbers
- Level numbering: Connection=1, Routing=2, Business=3, Filter=4

**States**: N/A (layers are static architectural concepts)

---

### 2. Layer Interface

**Description**: A contract defining how a layer exposes functionality to layers above it.

**Attributes**:
- `Name` (string): Interface name (e.g., "Connection", "SpotConnection")
- `Layer` (Layer): Which layer this interface represents
- `Methods` ([]MethodSignature): Interface method definitions
- `Category` (ExchangeCategory | nil): Optional category specialization
- `BaseInterface` (LayerInterface | nil): Interface this extends (for composition)

**Relationships**:
- One LayerInterface `belongs to` one Layer
- One LayerInterface `may extend` zero or one LayerInterface (base interface)
- One LayerInterface `is implemented by` multiple Implementations

**Constraints**:
- Interface methods must not reference concrete types from other layers
- Type-specific interfaces (with Category) must extend base interface
- Interface names follow convention: `{Category?}{Layer}` (e.g., "SpotConnection", "FuturesRouting")

**Example Interfaces**:
```go
// Base interfaces
type Connection interface { ... }
type Routing interface { ... }
type Business interface { ... }
type Filter interface { ... }

// Type-specific interfaces
type SpotConnection interface {
    Connection  // Extends base
    // Spot-specific methods
}

type FuturesRouting interface {
    Routing  // Extends base
    // Futures-specific methods
}
```

---

### 3. Layer Implementation

**Description**: Concrete realization of a layer interface for a specific exchange.

**Attributes**:
- `Name` (string): Implementation identifier (e.g., "BinanceWSConnection")
- `Interface` (LayerInterface): Interface this implements
- `Exchange` (string): Exchange name (e.g., "binance", "coinbase")
- `PackagePath` (string): Go package path (e.g., "exchanges/binance/infra/ws")
- `IsLegacy` (bool): Whether this is legacy code or migrated to interface

**Relationships**:
- One Implementation `implements` one or more LayerInterfaces
- One Implementation `belongs to` one Exchange
- One Implementation `may depend on` Implementations from lower layers

**Constraints**:
- Must satisfy all methods of implemented interface(s)
- Package path must match layer assignment in static analysis rules
- Cannot import packages from higher layers

**Lifecycle**:
1. **Legacy**: Existing implementation not yet migrated to interface
2. **Wrapped**: Legacy code wrapped with interface adapter
3. **Migrated**: Full interface implementation
4. **Validated**: Passes contract tests

---

### 4. Exchange Module

**Description**: Vertical slice containing implementations of all four layers for a specific exchange.

**Attributes**:
- `Name` (string): Exchange identifier (e.g., "binance")
- `Categories` ([]ExchangeCategory): Supported market types
- `Implementations` (map[Layer][]Implementation): Layer implementations

**Relationships**:
- One ExchangeModule `contains` multiple Implementations (one or more per layer)
- One ExchangeModule `supports` one or more ExchangeCategories

**Constraints**:
- Must provide at least one implementation for each of the four layers
- All implementations within module must follow four-layer pattern
- Category-specific functionality uses type-specific interfaces

**Example Structure**:
```
exchanges/binance/
├── infra/      # L1: Connection implementations
├── routing/    # L2: Routing implementations
├── bridge/     # L3: Business implementations
└── filter/     # L4: Filter implementations
```

---

### 5. Exchange Category

**Description**: Classification of exchanges by market type.

**Attributes**:
- `Type` (enum): "Spot", "Futures", "Options"
- `InterfaceExtensions` ([]LayerInterface): Category-specific interfaces

**Values**:
- **Spot**: Traditional spot market trading
- **Futures**: Futures/perpetual contracts
- **Options**: Options contracts

**Relationships**:
- One ExchangeCategory `defines` multiple type-specific LayerInterfaces
- One ExchangeModule `supports` one or more ExchangeCategories

**Constraints**:
- Each category may extend base layer interfaces with category-specific methods
- Cannot violate base layer interface contracts

---

### 6. Boundary

**Description**: Separation point between layers enforced through interface contracts.

**Attributes**:
- `UpperLayer` (Layer): Layer above the boundary
- `LowerLayer` (Layer): Layer below the boundary
- `Contract` (LayerInterface): Interface that defines the boundary
- `EnforcementRules` ([]Rule): Static analysis rules

**Relationships**:
- One Boundary `separates` exactly two Layers (adjacent levels)
- One Boundary `is enforced by` one LayerInterface

**Constraints**:
- Must have exactly 3 boundaries (between 4 layers)
- Upper layer can only access lower layer through interface contract
- Static analysis enforces no direct imports across boundaries

**Enforcement Mechanism**:
- Static analysis checks import statements
- Reports violations at build time
- Provides fix suggestions

---

### 7. Static Analysis Rule

**Description**: Rule defining allowed dependencies and enforced by static analyzer.

**Attributes**:
- `PackagePattern` (string): Glob pattern matching packages
- `AssignedLayer` (Layer): Which layer this package belongs to
- `AllowedImports` ([]string): Allowed import patterns

**Relationships**:
- Multiple Rules `define` one Layer's boundaries
- One Rule `is checked by` StaticAnalyzer

**Constraints**:
- Patterns must be unambiguous (no overlapping patterns)
- Must cover all packages in codebase
- Core interface packages exempt from restrictions

**Example Rules**:
```go
{
    PackagePattern: "*/infra/*",
    AssignedLayer: Connection,
    AllowedImports: ["core/layers/*", "core/transport/*"]
}
{
    PackagePattern: "*/routing/*",
    AssignedLayer: Routing,
    AllowedImports: ["core/layers/*", "*/infra/*"]
}
```

---

### 8. Cross-Cutting Concern

**Description**: Functionality needed across all layers (logging, metrics, error handling).

**Attributes**:
- `Name` (string): Concern name (e.g., "logging", "metrics")
- `GlobalAccessor` (string): Package path to accessor function
- `Interface` (interface): Contract for the concern

**Relationships**:
- One CrossCuttingConcern `is accessible by` all Layers
- One CrossCuttingConcern `does not create` Layer dependencies

**Constraints**:
- Must be accessed via global singleton or static accessor
- Cannot require explicit dependency injection through layer interfaces
- Should be mockable for testing

**Examples**:
- **Logging**: `observability.Log()` → Logger interface
- **Metrics**: `observability.Metrics()` → Metrics interface  
- **Error Handling**: `errs` package with typed errors

---

## Entity Relationships Diagram

```
┌─────────────────┐
│     Layer       │
│  (4 instances)  │
└────────┬────────┘
         │ contains
         ▼
┌─────────────────┐      extends      ┌──────────────────┐
│ Layer Interface │◄─ ─ ─ ─ ─ ─ ─ ─ ─│Type-Specific     │
│    (base)       │                   │Interface         │
└────────┬────────┘                   │(e.g., SpotConn)  │
         │                            └──────────────────┘
         │ implemented by
         ▼
┌─────────────────┐      belongs to   ┌──────────────────┐
│Implementation   │─────────────────►│Exchange Module   │
│                 │                   │                  │
└────────┬────────┘                   └──────────────────┘
         │                                     │
         │ depends on (lower layers)           │ supports
         ▼                                     ▼
┌─────────────────┐                   ┌──────────────────┐
│Implementation   │                   │Exchange Category │
│ (lower layer)   │                   │(Spot/Futures/    │
└─────────────────┘                   │ Options)         │
                                      └──────────────────┘

┌─────────────────┐      separates    ┌──────────────────┐
│    Boundary     │◄──────────────────│   Layer Pair     │
│                 │                   │(adjacent levels) │
└────────┬────────┘                   └──────────────────┘
         │
         │ enforced by
         ▼
┌─────────────────┐      checks       ┌──────────────────┐
│Static Analysis  │◄──────────────────│   Build Process  │
│     Rule        │                   │                  │
└─────────────────┘                   └──────────────────┘
```

---

## Data Flow

### 1. Request Flow (Top to Bottom)

```
Filter Layer (L4)
    ↓ calls via Business interface
Business Layer (L3)  
    ↓ calls via Routing interface
Routing Layer (L2)
    ↓ calls via Connection interface
Connection Layer (L1)
    ↓ network I/O
Exchange API
```

### 2. Response Flow (Bottom to Top)

```
Exchange API
    ↓ raw data
Connection Layer (L1) - receive & basic parsing
    ↓ via callback/channel
Routing Layer (L2) - normalize & route
    ↓ via typed events
Business Layer (L3) - apply business logic
    ↓ via domain events
Filter Layer (L4) - filter & aggregate
    ↓ via public API
Consumer Application
```

### 3. Cross-Cutting Access (All Layers)

```
Any Layer
    ↓ calls
observability.Log() / Metrics() / etc.
    ↓
Global Singleton
    ↓
Concrete Implementation (injected)
```

---

## Validation Rules

### Interface Contract Validation

**Rule**: All implementations must satisfy behavioral contracts, not just type signatures

**Validation Method**: Table-driven contract tests

**Example**:
```go
func TestConnectionContract(t *testing.T) {
    implementations := []layers.Connection{
        binanceWS.New(...),
        binanceREST.New(...),
    }
    
    for _, impl := range implementations {
        // Test idempotency
        impl.Connect(ctx)
        err := impl.Connect(ctx) // Must succeed or no-op
        assert.NoError(t, err)
        
        // Test cleanup
        impl.Close()
        impl.Close() // Must be safe to call multiple times
    }
}
```

### Layer Boundary Validation

**Rule**: Packages must only import from allowed layers

**Validation Method**: Static analysis at build time

**Example Violation**:
```go
// In exchanges/binance/infra/ws/client.go (L1: Connection)
import "exchanges/binance/routing" // ❌ VIOLATION: L1 cannot import L2

// Analyzer reports:
// client.go:5: layer boundary violation: 
//   package "exchanges/binance/infra/ws" (Connection layer) 
//   cannot import "exchanges/binance/routing" (Routing layer)
//   Allowed imports: core/layers/*, core/transport/*
```

### Type-Specific Interface Validation

**Rule**: Type-specific interfaces must extend base interface

**Validation Method**: Compile-time type checking + documentation checks

**Example**:
```go
type SpotConnection interface {
    Connection  // ✅ Must embed base interface
    GetSpotEndpoint() string
}
```

---

## Migration State Tracking

**Purpose**: Track progress of migrating legacy code to layer interfaces

**Attributes per Implementation**:
- `MigrationStatus`: "Legacy" | "Wrapped" | "Migrated" | "Validated"
- `ContractTestsPassing`: bool
- `StaticAnalysisClean`: bool

**Progression**:
1. **Legacy** → Existing code, no interface
2. **Wrapped** → Interface adapter created, delegates to legacy code
3. **Migrated** → Full refactor to implement interface directly
4. **Validated** → Contract tests pass, static analysis clean

**Completion Criteria**:
- All implementations reach "Validated" status
- Static analysis enabled in CI with no violations
- Contract tests pass for all implementations
- Documentation updated

---

## Summary

This data model defines:
- **4 Layer entities** with clear hierarchy and dependency rules
- **Layer Interfaces** as contracts with composition support
- **Implementations** mapping code to interfaces
- **Exchange Modules** as vertical slices
- **Boundaries** enforced by static analysis
- **Migration path** from legacy to validated implementations

Next: Generate concrete interface contracts in `contracts/` directory.
