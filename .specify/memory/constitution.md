<!--
Sync Impact Report
Version: 1.2.0 → 1.3.0
Modified Principles:
- Updated TS-01: Coverage thresholds now ≥80% overall and ≥70% per package (previously variable thresholds)
- Expanded TS-02: Added explicit table-driven test enforcement
- Added TS-08: Integration Test Isolation (build tags)
- Added TS-09: Test Deduplication and Fixture Sharing
- Expanded TS-04: Enhanced to include deterministic test requirements (no sleeps, context timeouts)
- Updated GOV-04: Coverage checks are now blocking in CI
Added Sections:
- None
Removed Sections:
- None
Template Updates:
- ✅ .specify/templates/plan-template.md (no updates needed - testing principles independent)
- ✅ .specify/templates/spec-template.md (no updates needed - testing principles independent)
- ✅ .specify/templates/tasks-template.md (no updates needed - testing principles independent)
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
Principle: Strict separation between Transport (L1), Routing (L2), Exchange (L3), and Pipeline (L4) layers.  
Rationale: Prevents circular dependencies and ensures testability at each layer.  
Implementation:  
- L1 (Transport): Only handles HTTP/WebSocket primitives, retries, rate limits  
- L2 (Routing): Maps exchange formats to/from canonical types  
- L3 (Exchange): Exposes unified market APIs and services  
- L4 (Pipeline): Coordinates multi-exchange filtering and aggregation  
Anti-pattern: Routing logic in transport clients; business logic in routers

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

## 5. Governance Framework

### 5.1 How Principles Guide Technical Decisions

**GOV-01: Decision Authority**  
- Code Quality (CQ-01 through CQ-08): All MUST principles are CI-enforced and non-negotiable  
- Testing Standards (TS-01 through TS-09): Coverage thresholds and test discipline are CI-enforced; SHOULD principles warn but don't block  
- User Experience (UX-01 through UX-06): MUST principles require code review approval; SHOULD principles are advisory  
- Performance (PERF-01 through PERF-06): MUST principles validated via benchmarks; SHOULD principles tracked as metrics  
- Architecture Boundaries (ARCH-01 through ARCH-04): MUST principles owned by architecture council; violations require design review sign-off

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
- Code quality principles (CQ-01 through CQ-08)  
- Testing standards coverage thresholds (TS-01) - **Coverage checks are blocking in CI**  
- Testing discipline requirements (TS-02, TS-04, TS-08, TS-09)  
- User experience consistency requirements (UX-01, UX-03, UX-04, UX-06)  
- Performance benchmarks showing no regressions (PERF-03)  
- Architecture boundaries (ARCH-01 through ARCH-04)

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

## 6. Architecture Boundaries

**ARCH-01: Framework Residency (MUST)**  
Principle: Shared infrastructure lives exclusively under `/frameworks` or `/lib` and stays domain-agnostic.  
Rationale: Keeps reusable capabilities isolated from business rules and simplifies reuse.  
Implementation:  
- Framework packages must not import domain modules or rely on domain-specific data models  
- Infrastructure helpers expose configuration via interfaces, not concrete domain types  
- CI fails if reusable groundwork appears outside approved directories

**ARCH-02: Domain Package Purity (MUST)**  
Principle: Domain packages (e.g., `/market_data`, `/trading`) contain only business logic and must not ship reusable frameworks.  
Rationale: Protects domain boundaries and prevents cross-contamination of shared infrastructure.  
Implementation:  
- Domain directories expose orchestrations, aggregates, and application services only  
- Reusable helpers discovered in domain code are relocated to `/frameworks` or `/lib`  
- Code review blocks merges that add framework-style abstractions under domain paths

**ARCH-03: Dependency Direction (MUST)**  
Principle: Domain code may depend on frameworks; frameworks must never depend on domain packages.  
Rationale: Enforces a clean dependency graph and enables framework reuse across domains.  
Implementation:  
- Static analysis forbids imports from `/market_data`, `/pricing`, or other domain packages within framework code  
- Domain layers wrap framework services behind domain-facing interfaces  
- Violations trigger architectural review before merge

**ARCH-04: Framework API Lifecycle (MUST)**  
Principle: Framework APIs are documented, versioned, and covered by automated contract tests.  
Rationale: Guarantees predictable upgrades and confidence in cross-domain reuse.  
Implementation:  
- Document framework APIs with Godoc plus usage guides in `/frameworks/docs/`  
- Version framework modules using semantic versioning with changelogs capturing migration steps  
- Provide unit coverage and contract tests that validate integration expectations for each public API

### Appendix: Severity Levels

- **MUST**: Failing blocks CI; non-negotiable requirement  
- **SHOULD**: Failing warns; may block in strict mode; requires justification to override  
- **INFO**: Advisory; best practice recommendation

**Version**: 1.3.0 | **Ratified**: 2025-10-08 | **Last Amended**: 2025-10-11
