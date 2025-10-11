<!--
Sync Impact Report
Version: 1.3.0 → 1.4.0
Modified Principles:
- ARCH-01: Now explicitly mandates business-agnostic core libraries live under /lib only; clarified import rules and added naming/types/docs neutrality
- ARCH-02/ARCH-03/ARCH-04: Reworded to refer to core libraries and /lib (removed /frameworks references)
- CQ-02: Clarified API boundaries by tying contracts to core/layers/* and requiring boundary ownership
- GOV-01/GOV-04: Updated architecture boundary references to ARCH-01 through ARCH-06
Added Sections:
- CQ-09: Structured Logging & Metrics Interfaces (MUST)
- TS-10: Exhaustive Boundary Tests (MUST)
- PERF-07: Routing Hot Path Baselines (MUST)
- ARCH-05: Zero Domain Leakage (MUST)
- ARCH-06: Stable Package Names (MUST)
- GOV-08: Spec-Authorized Breaking Changes (MUST)
Removed Sections:
- None
Template Updates:
- ✅ .specify/templates/spec-template.md (compatibility note updated; /lib only; added observability/perf gates)
- ✅ .specify/templates/plan-template.md (constitution check aligned to /lib; perf/observability/stability gates)
- ✅ .specify/templates/tasks-template.md (/lib reference; migration tasks guidance retained)
Follow-up TODOs:
- None
-->

# Code Quality Principles

## 1. Code Quality Principles

### 1.1 Core Design Philosophy

**CQ-01: SDK-First Architecture (MUST)**  
Principle: The core package defines canonical truth; all adapters conform to core interfaces.  
Rationale: Ensures uniform behavior across exchanges and prevents adapter-specific leakage into client code.  
Implementation:  
- Core interfaces are frozen and change only with protocol version bumps  
- Adapters implement, never extend, core contracts  
- No adapter-specific types exposed in public APIs

**CQ-02: Layered Architecture Enforcement (MUST)**  
Principle: Strict separation between Transport (L1), Routing (L2), Exchange (L3), and Pipeline (L4) layers with clear API boundaries.  
Rationale: Prevents circular dependencies and ensures testability at each layer.  
Implementation:  
- L1 (Transport): Only handles HTTP/WebSocket primitives, retries, rate limits  
- L2 (Routing): Maps exchange formats to/from canonical types  
- L3 (Exchange): Exposes unified market APIs and services  
- L4 (Pipeline): Coordinates multi-exchange filtering and aggregation  
Anti-pattern: Routing logic in transport clients; business logic in routers
Boundary Ownership: Contracts for each layer live under `core/layers/*.go` and are the only allowed cross-layer API surface; changes require design review and version alignment.

**CQ-03: Zero Floating-Point Policy (MUST)**  
Principle: All monetary values use *big.Rat; float32/float64 prohibited in public APIs.  
Rationale: Eliminates precision loss in financial calculations.  
Implementation:  
- Use internal/numeric helpers for formatting and parsing  
- JSON marshaling must preserve decimal precision  
- Test fixtures include boundary values (MAX_SAFE_INTEGER, microcents, satoshis)  
Violations: CI fails on any float32/float64 in struct fields, function signatures, or JSON tags

**CQ-04: Canonical Symbol Format (MUST)**  
Principle: All public APIs use BASE-QUOTE uppercase format (e.g., BTC-USDT).  
Rationale: Uniform symbol representation across exchanges.  
Implementation:  
- Translators convert exchange-native symbols at adapter boundaries  
- Regex validation: ^[A-Z0-9]+-[A-Z0-9]+$  
- Register translators in plugin/plugin.go during adapter initialization

**CQ-05: Exhaustive Enum Mapping (MUST)**  
Principle: All enum mappings use explicit switch statements; no silent defaults.  
Rationale: Forces developers to handle all exchange-specific values.  
Implementation:  
- Map all known values explicitly  
- Return error for unknown values  
Anti-pattern: Default case returning a "best guess" status

**CQ-06: Typed Errors (MUST)**  
Principle: All public APIs return *errs.E with canonical error codes.  
Rationale: Enables programmatic error handling and consistent logging.  
Implementation:  
- Use errs.New() with CodeAuth, CodeRateLimited, CodeInvalid, CodeExchange, CodeNetwork  
- Include exchange name, HTTP status, raw error codes/messages  
- Never return raw error from fmt.Errorf in public APIs

**CQ-07: Documentation as Code (MUST)**  
Principle: All exported types and functions have godoc comments.  
Rationale: Self-documenting code reduces onboarding friction.  
Implementation:  
- Document expected behavior, not implementation details  
- Include examples for non-obvious APIs  
- Link to relevant exchange documentation for adapter-specific quirks

**CQ-08: Immutable Interfaces (MUST)**  
Principle: Public interfaces freeze after 1.0 release; changes require protocol version bump.  
Rationale: API stability guarantees for SDK consumers.  
Implementation:  
- core.ProtocolVersion follows SemVer  
- Adapters return matching version via SupportedProtocolVersion()  
- New capabilities added via optional participant interfaces, not interface changes

**CQ-09: Structured Logging & Metrics Interfaces (MUST)**  
Principle: Observability is exposed via narrow, vendor-neutral interfaces housed in `/lib` (e.g., `/lib/log`, `/lib/metrics`).  
Rationale: Enables consistent structured logs and metrics without coupling to a specific provider.  
Implementation:  
- Logging interface supports leveled, structured fields: Debug/Info/Warn/Error(ctx, msg, fields...)  
- Metrics interfaces for counters, gauges, histograms with label support  
- No direct dependencies on logging/metrics vendors in domain or routing code; wiring occurs at composition roots  
Anti-pattern: Importing concrete vendor SDKs (e.g., zap, slog, prometheus) directly in domain or routing packages

## 2. Testing Standards

**TS-01: Minimum Coverage Thresholds (MUST)**  
Principle: Code must maintain ≥80% overall coverage and ≥70% coverage per package.  
Rationale: Ensures comprehensive testing and prevents untested code from reaching production.  
Implementation:  
- Overall repository coverage: ≥80%  
- Per-package minimum: ≥70%  
- Coverage checks block CI pipeline on violations  
- Use `go test -cover` and coverage reports to track compliance  
Enforcement: CI fails if coverage drops below thresholds

**TS-02: Test Design Discipline (MUST)**  
Principle: Enforce table-driven unit tests; prefer small, focused tests over end-to-end where possible.  
Rationale: Table-driven tests improve maintainability, readability, and enable exhaustive scenario coverage with minimal code duplication.  
Implementation:  
- Unit tests use table-driven design with named test cases  
- Each test case specifies: input, expected output, and assertion logic  
- Prefer testing individual functions/methods over full integration paths  
- Reserve end-to-end tests for critical user journeys only  
Test Pyramid Target: 70% unit tests, 20% integration tests, 10% end-to-end tests  
Anti-pattern: Copy-pasted test functions with minor input variations; monolithic test functions covering multiple scenarios

**TS-03: Race Detector Always (MUST)**  
Principle: All CI test runs include -race flag.  
Rationale: Catch concurrency bugs early.

**TS-04: Deterministic Fixtures and Tests (MUST)**  
Principle: Integration tests use recorded HTTP/WebSocket fixtures; require deterministic tests (no sleeps; use context timeouts/fakes).  
Rationale: Reproducible tests without exchange dependencies; eliminates flaky tests caused by timing issues.  
Implementation:  
- Store fixtures in testdata/ directories  
- Use httptest for REST clients; mock WebSocket frames  
- Never commit API keys or secrets to fixtures  
- Replace time.Sleep() with context.WithTimeout() or fake clock implementations  
- Use dependency injection to provide controllable time sources in tests  
Anti-pattern: Tests with arbitrary sleep durations; tests that occasionally fail due to timing

**TS-05: Error Path Coverage (MUST)**  
Principle: Every error return path has a corresponding test case.  
Rationale: Ensures graceful degradation under failure.  
Implementation:  
- Test network errors, timeouts, rate limits, malformed responses  
- Verify error codes and messages match errs package standards  
- Test partial failures in batch operations

**TS-06: Live Test Hygiene (SHOULD)**  
Principle: Optional live tests gated by environment variables; skip cleanly when absent.  
Rationale: CI can run without credentials; developers can opt-in for real testing.  
Implementation:  
- Check for required environment variables  
- Skip test if credentials not available  
- Document required environment variables in README

**TS-07: Performance Benchmarks (SHOULD)**  
Principle: Critical paths have benchmark tests tracking allocations and latency.  
Rationale: Prevents performance regressions.  
Implementation:  
- Benchmark hot paths: decimal parsing, symbol translation, event decoding  
- Target: <500ns per event decode; zero allocations in message routing  
- Use testing.B with -benchmem

**TS-08: Integration Test Isolation (MUST)**  
Principle: Separate integration tests with the build tag 'integration' and exclude them from unit coverage by default.  
Rationale: Keeps unit test runs fast and focused; allows selective execution of slower integration tests.  
Implementation:  
- Tag integration test files with `//go:build integration` at the top  
- Run unit tests with standard `go test ./...`  
- Run integration tests separately with `go test -tags=integration ./...`  
- Exclude integration tests from unit coverage calculations  
- Document integration test execution requirements in test file comments

**TS-09: Test Deduplication and Fixture Sharing (MUST)**  
Principle: Avoid duplicate testing: no repeated assertions for the same behavior across unit/integration layers; share fixtures via internal/testhelpers; forbid duplicate test names; use subtests.  
Rationale: Reduces maintenance burden, prevents test divergence, and improves test suite clarity.  
Implementation:  
- Test each behavior at the most appropriate level (unit vs integration) only once  
- Share common test fixtures and helpers in `internal/testhelpers` package  
- Use `t.Run()` for subtests to organize related test cases and ensure unique names  
- Forbid duplicate test function names across the entire test suite  
- Code review must verify no redundant test assertions for identical behavior  
Anti-pattern: Testing the same parsing logic in both unit tests and integration tests; copy-pasted test fixtures; multiple tests named "TestValidation"

**TS-10: Exhaustive Boundary Tests (MUST)**  
Principle: All public APIs and enumerations at layer boundaries are exhaustively tested.  
Rationale: Guarantees contract correctness and prevents silent regressions across layers.  
Implementation:  
- Every exported function/method in boundary packages (`core/layers`, `/lib/**` public APIs) has at least one positive and one negative test  
- Enumerations and switch-based mappings include a test case per value; unknown values covered with error assertions  
- Table-driven tests enumerate all supported message types in routing and all error paths in adapters  
Enforcement: CI blocks merges if new exported members land without corresponding tests

## 3. User Experience Consistency

**UX-01: Uniform Error Messages (MUST)**  
Principle: All error messages follow a consistent structure: "[exchange] operation failed: reason (code: X)"  
Example: "[binance] ticker fetch failed: rate limit exceeded (code: 429)"  
Rationale: Predictable error parsing for logging/monitoring.

**UX-02: Progressive Disclosure (MUST)**  
Principle: Simple use cases require minimal configuration; advanced options opt-in.  
Implementation:  
- Default constructors (New()) work with environment variables  
- Advanced options via functional option pattern (WithTimeout(), WithRateLimit())  
- Sensible defaults for all settings (HTTP timeout: 30s, reconnect backoff: exponential)

**UX-03: Context Propagation (MUST)**  
Principle: All I/O operations accept context.Context for cancellation and deadlines.  
Rationale: Enables graceful shutdowns and request timeouts.  
Implementation:  
- First parameter of all API methods is ctx context.Context  
- Respect context cancellation in long-running operations (WebSocket subscriptions)  
- Propagate context through transport, routing, and exchange layers

**UX-04: Graceful Degradation (MUST)**  
Principle: Failures in one exchange/symbol don't crash the entire pipeline.  
Implementation:  
- WebSocket disconnects trigger reconnects with backoff, not panics  
- Invalid messages logged and discarded, not propagated as crashes  
- Rate limit errors pause subscriptions, don't terminate connections

**UX-05: Observability Hooks (SHOULD)**  
Principle: Expose metrics, logs, and traces without forcing specific implementations.  
Implementation:  
- Structured logging with log levels (DEBUG, INFO, WARN, ERROR)  
- Optional metrics callbacks (OnMessage, OnError, OnReconnect)  
- OpenTelemetry-compatible trace contexts

**UX-06: Idiomatic Go APIs (MUST)**  
Principle: Follow standard Go conventions for naming, patterns, and idioms.  
Implementation:  
- Interfaces in consumer packages (e.g., io.Reader, not custom Readable)  
- Error handling via explicit returns, not exceptions  
- Channels for asynchronous events (WebSocket messages)  
- Close() methods for resource cleanup

## 4. Performance Requirements

**PERF-01: WebSocket Message Latency (MUST)**  
Target: P99 < 10ms from wire receipt to routed event emission  
Measurement: Instrument message timestamps at entry/exit points  
Rationale: Low-latency trading requires minimal processing overhead

**PERF-02: REST API Response Time (SHOULD)**  
Target: P95 < 500ms for public endpoints (tickers, instruments)  
Target: P95 < 1s for authenticated endpoints (orders, balances)  
Excludes: Network RTT (out of SDK control)

**PERF-03: Memory Efficiency (MUST)**  
Target: Zero allocations in hot paths (event decoding, symbol translation)  
Target: Bounded memory growth in long-running connections (no leaks)  
Implementation:  
- Pool reusable buffers for JSON parsing  
- Pre-allocate slices with known capacities  
- Use sync.Pool for temporary objects

**PERF-04: Concurrent Subscription Scalability (SHOULD)**  
Target: Handle 100+ concurrent symbol subscriptions on single WebSocket connection  
Rationale: Reduce connection overhead; respect exchange limits  
Implementation:  
- Multiplex subscriptions over shared connections  
- Fan-out messages to multiple subscribers via pub/sub pattern

**PERF-05: Rate Limit Compliance (MUST)**  
Principle: Never exceed exchange-documented rate limits; use token bucket algorithms.  
Implementation:  
- Configurable rates per endpoint class (public, private, order placement)  
- Pre-request checks; delay requests if bucket empty  
- Respect Retry-After headers on 429 responses

**PERF-06: Connection Reuse (MUST)**  
Principle: HTTP clients use connection pooling; WebSocket connections persist.  
Implementation:  
- http.Client with MaxIdleConnsPerHost tuned per exchange  
- WebSocket reconnect on errors, not per-subscription connections

**PERF-07: Routing Hot Path Baselines (MUST)**  
Target: Router dispatch in `/lib/ws-routing` performs with zero allocations and median < 500ns, P99 < 2µs per event on reference hardware.  
Measurement: Benchmarks in `lib/ws-routing` cover decode → route → emit; include `-benchmem` and allocation checks.  
Rationale: Routing is a critical latency path; strict baselines prevent regressions.

## 5. Governance Framework

### 5.1 How Principles Guide Technical Decisions

**GOV-01: Decision Authority**  
- Code Quality (CQ-01 through CQ-09): All MUST principles are CI-enforced and non-negotiable  
- Testing Standards (TS-01 through TS-10): Coverage thresholds and test discipline are CI-enforced; SHOULD principles warn but don't block  
- User Experience (UX-01 through UX-06): MUST principles require code review approval; SHOULD principles are advisory  
- Performance (PERF-01 through PERF-07): MUST principles validated via benchmarks; SHOULD principles tracked as metrics  
- Architecture Boundaries (ARCH-01 through ARCH-06): MUST principles owned by architecture council; violations require design review sign-off

**GOV-02: Standard Enforcement**  
- Automated CI checks: Block merge on MUST violations  
- Code review: Verify adherence to all principles; may override SHOULD violations with justification  
- Exception process: Requires written rationale and 2 core maintainer approvals for MUST principle exceptions

**GOV-03: Implementation Choices**  
- When principles conflict: Document the conflict and propose alternatives; escalate to core maintainers  
- When technically infeasible: Provide evidence (benchmark, external constraint); request exception review  
- When principles are silent: Follow idiomatic Go practices; defer to code review judgment

**GOV-04: Quality Gates**  
All code changes must satisfy:  
- Code quality principles (CQ-01 through CQ-09)  
- Testing standards coverage thresholds (TS-01) - **Coverage checks are blocking in CI**  
- Testing discipline requirements (TS-02, TS-04, TS-08, TS-09, TS-10)  
- User experience consistency requirements (UX-01, UX-03, UX-04, UX-06)  
- Performance benchmarks showing no regressions (PERF-03)  
- Architecture boundaries (ARCH-01 through ARCH-06)

**GOV-05: Principle Evolution**  
- Review cycle: Principles reviewed annually or after significant project milestones  
- Update process: Collect feedback from production incidents and developer experience; propose changes via RFC  
- Version control: All changes to principles are versioned and dated in this document

**GOV-06: Innovation Over Compatibility (MUST)**
Principle: Prioritize shipping new capabilities and architectural improvements over preserving backward compatibility.
Rationale: Sustained innovation keeps the platform competitive even when changes require breaking adjustments.
Implementation:
- When trade-offs arise, select the path that unlocks new capabilities even if it requires a breaking change
- Pair breaking changes with clear migration notes and protocol version updates
- Communicate compatibility impacts in release notes and planning artifacts before rollout

**GOV-07: Migration Safety Net (MUST)**
Principle: Refactors that alter import paths must include migration notes and a one-release deprecation shim.
Rationale: Provides teams runway to adopt breaking changes without halting delivery.
Implementation:
- Document migration steps in release notes and specs before merging breaking changes
- Maintain backward-compatible import aliases for at least one release cycle
- Remove shims only after verifying downstream adoption or obtaining explicit approvals

**GOV-08: Spec-Authorized Breaking Changes (MUST)**  
Principle: Backward-compatibility-breaking refactors are allowed when explicitly stated in the feature spec.  
Rationale: Makes intentional breaks visible and reviewable while preserving overall project velocity.  
Implementation:  
- Spec must include an explicit statement: "This change is backward incompatible"  
- Spec must document impact scope, migration path, and import path changes (if any)  
- Plan must include a one-release deprecation shim when import paths change; link to migration notes

## 6. Architecture Boundaries

**ARCH-01: Core Library Residency (/lib) (MUST)**  
Principle: Shared, reusable infrastructure lives exclusively under `/lib` and is strictly business-agnostic.  
Rationale: Isolates capabilities from business rules and enables reuse across domains.  
Implementation:  
- `/lib/**` packages must not import domain modules or rely on domain-specific data models  
- APIs expose configuration via interfaces, not concrete domain types  
- No domain terms appear in `/lib` package names, exported identifiers, or doc comments  
- CI fails if reusable groundwork appears outside `/lib`

**ARCH-02: Domain Package Purity (MUST)**  
Principle: Domain packages (e.g., `/market_data`, `/trading`) contain only business logic and must not ship reusable core libraries.  
Rationale: Protects domain boundaries and prevents cross-contamination of shared infrastructure.  
Implementation:  
- Domain directories expose orchestrations, aggregates, and application services only  
- Reusable helpers discovered in domain code are relocated to `/lib`  
- Code review blocks merges that add core-library-style abstractions under domain paths

**ARCH-03: Dependency Direction (MUST)**  
Principle: Domain code may depend on core libraries; core libraries must never depend on domain packages.  
Rationale: Enforces a clean dependency graph and enables framework reuse across domains.  
Implementation:  
- Static analysis forbids imports from `/market_data`, `/pricing`, or other domain packages within `/lib/**` code  
- Domain layers wrap core-library services behind domain-facing interfaces  
- Violations trigger architectural review before merge

**ARCH-04: Core Library API Lifecycle (MUST)**  
Principle: Core library APIs are documented, versioned, and covered by automated contract tests.  
Rationale: Guarantees predictable upgrades and confidence in cross-domain reuse.  
Implementation:  
- Document `/lib/**` APIs with Godoc plus usage guides (e.g., `/lib/docs/`)  
- Version library modules using semantic versioning with changelogs capturing migration steps  
- Provide unit coverage and contract tests that validate integration expectations for each public API

**ARCH-05: Zero Domain Leakage (MUST)**  
Principle: No domain-specific naming, types, or documentation leaks into `/lib/**`.  
Rationale: Keeps `/lib` reusable across multiple domains and prevents implicit coupling.  
Implementation:  
- Prohibit domain nouns (e.g., "orderbook", "trade") in `/lib` package names and exported identifiers  
- Prohibit domain-specific structs/enums in `/lib`; use generic types and interfaces  
- Prohibit domain examples in `/lib` godoc; use abstract examples

**ARCH-06: Stable Package Names (MUST)**  
Principle: Package import paths are stable; renames require an RFC and spec-flagged breaking change with deprecation shims.  
Rationale: Preserves import stability for downstream users.  
Implementation:  
- No package renames or moves that change import paths unless GOV-08 process is followed  
- Provide `deprecated` wrappers/aliases for one release when import paths change  
- Static analysis in CI flags import changes for review

### Appendix: Severity Levels

- **MUST**: Failing blocks CI; non-negotiable requirement  
- **SHOULD**: Failing warns; may block in strict mode; requires justification to override  
- **INFO**: Advisory; best practice recommendation

**Version**: 1.4.0 | **Ratified**: 2025-10-08 | **Last Amended**: 2025-10-11
