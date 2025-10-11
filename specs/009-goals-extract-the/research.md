# Phase 0 Research: WebSocket Routing Framework Extraction

## Decision Log

### 1. Framework Residency
- **Decision**: Host the reusable WebSocket routing framework under `/lib/ws-routing`.
- **Rationale**: Aligns with ARCH-01 requiring domain-agnostic infrastructure to live in `/lib`; avoids future relocation once additional domains adopt the framework.
- **Alternatives Considered**:
  - `/frameworks/ws-routing`: Consistent with name but increases path churn because existing shared utilities already consolidated in `/lib`.
  - Leave code inside `market_data`: Violates ARCH-02 by keeping reusable code in a domain package and blocks cross-domain adoption.

### 2. Public API Surface
- **Decision**: Expose explicit functions for `Init`, `Start`, `Subscribe`, `Publish`, and `UseMiddleware`, each returning typed errors from the `errs` package.
- **Rationale**: Mirrors current domain entry points so adopters have a predictable migration path while enabling contract tests to cover invocation semantics.
- **Alternatives Considered**:
  - Builder pattern with fluent chaining: Adds complexity without evidence of broader benefit and risks breaking parity with existing call sites.
  - Channel-only interface: Reduces flexibility for middleware injection and hinders typed error propagation.

### 3. Observability Strategy
- **Decision**: Preserve existing structured logging behavior and make instrumentation extension points optional for downstream domains.
- **Rationale**: Satisfies constitution UX-05 guidance without expanding scope; avoids adding hard dependencies to metrics/tracing frameworks while keeping hooks for future enhancements.
- **Alternatives Considered**:
  - Mandate metrics emission: Introduces cross-cutting concerns and potential new dependencies conflicting with current minimal footprint.
  - Provide built-in tracing: Overextends milestone and risks violating "no behavior change" acceptance criteria.

### 4. Testing Coverage Approach
- **Decision**: Maintain regression parity through contract suites comparing old vs. new routers, backed by deterministic fixtures and race-enabled unit tests.
- **Rationale**: Meets TS-01 coverage thresholds, TS-04 deterministic fixtures, and SC-001/SC-002 success criteria while safeguarding concurrency behavior.
- **Alternatives Considered**:
  - Rely solely on existing market data tests: Fails to cover framework-only usage and weakens confidence for future domains.
  - Focus on property-based testing: Valuable long-term but extends timeline without immediate parity gains.

### 5. Architecture Council Approval
- **Decision**: Architecture council session AC-2025-10-11 ratified `/lib/ws-routing` as an approved shared module with a one-release shim requirement for downstream adopters.
- **Rationale**: Ensures governance records reflect the new framework location and documents the mandated deprecation window for `market_data/framework/router` imports.
- **References**: Meeting minutes `AC-2025-10-11-WSR` archived in Confluence, action item to notify domain leads before GA.
