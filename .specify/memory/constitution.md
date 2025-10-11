<!--
Sync Impact Report
Version: 1.5.0 → 1.6.0
Modified Principles:
- GOV-01: Clarified pre-1.0 phase decision authority; removed architecture council references
- GOV-02: Simplified enforcement for solo/small team; updated exception process
- GOV-02b: NEW - Added automated enforcement inventory and expansion roadmap
- GOV-03: Simplified implementation choices for solo developer workflow
- GOV-05: Removed RFC requirement; replaced with spec or direct commit
- CQ-08: Renamed to "Interface Stability Roadmap"; added pre-1.0 vs post-1.0 phases
- CQ-09: Added /internal/observability as acceptable location for internal helpers
- TS-01: Added coverage exclusions and practical notes for integration-heavy code
- PERF-01: Relaxed from MUST to SHOULD; changed target from <10ms to <100ms
- PERF-02: Removed specific ms targets for pre-1.0 phase
- PERF-07: Relaxed from MUST to SHOULD; changed from strict baselines to trend tracking
Added Sections:
- GOV-02b: Automated Enforcement Inventory
- Version History: Added comprehensive changelog at end of document
Removed Sections:
- None
Template Updates:
- None required
Follow-up TODOs:
- Consider adding float detection linter (Q1 2026 roadmap item in GOV-02b)
- Consider adding import path stability checks (Q1 2026 roadmap item in GOV-02b)
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

**CQ-08: Interface Stability Roadmap (MUST)**  
Principle: Public interfaces are versioned; breaking changes permitted pre-1.0 with documentation.  
Current Phase (Pre-1.0):  
- Core interfaces may change between minor versions  
- Breaking changes documented in specs and BREAKING_CHANGES_v2.md  
- Adapters must stay synchronized with core interface versions  

Post-1.0 Commitment:  
- Public interfaces freeze; changes require protocol version bump  
- New capabilities added via optional interfaces, not interface changes  
- core.ProtocolVersion follows SemVer  
- Adapters return matching version via SupportedProtocolVersion()

**CQ-09: Structured Logging & Metrics Interfaces (MUST)**  
Principle: Observability is exposed via narrow, vendor-neutral interfaces housed in `/lib` (e.g., `/lib/log`, `/lib/metrics`) or `/internal/observability` for internal-only use.  
Rationale: Enables consistent structured logs and metrics without coupling to a specific provider.  
Implementation:  
- Public-facing observability interfaces → `/lib/observability` or `/lib/log`  
- Internal-only observability helpers → `/internal/observability`  
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
- Coverage checks block CI pipeline on violations via `make coverage-check`  
- Use `go test -cover` and coverage reports to track compliance  
- Exclusions: Generated code, main.go entry points, and example packages exempt  
Enforcement: CI fails if coverage drops below thresholds

Practical Notes:  
- If a package falls below 70%, either add tests or document why it's difficult to test (e.g., integration-heavy adapter code)  
- Integration tests (with `//go:build integration` tag) count toward coverage when run

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

**PERF-01: Reasonable WebSocket Latency (SHOULD)**  
Target: Process messages within human-perceptible time (<100ms P99)  
Measurement: Benchmark tests in hot paths; monitor for regressions  
Rationale: Performance matters but ultra-low latency isn't required for this project's use case

**PERF-02: REST API Response Time (SHOULD)**  
Target: Reasonable timeouts; don't add unnecessary overhead  
Implementation: Profile when optimizing; no specific ms targets pre-1.0  
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

**PERF-07: Hot Path Efficiency (SHOULD)**  
Target: Zero allocations in message routing hot paths where practical  
Measurement: Benchmarks with `-benchmem` flag; track trends, not absolutes  
Rationale: Memory efficiency prevents resource leaks in long-running processes

## 5. Governance Framework

### 5.1 How Principles Guide Technical Decisions

**GOV-01: Decision Authority (Pre-1.0 Phase)**  
During pre-1.0 development:
- Code Quality (CQ-01 through CQ-09): CI-enforced MUST principles are non-negotiable
- Testing Standards (TS-01, TS-02, TS-04, TS-08, TS-09, TS-10): CI-enforced and blocking
- Performance (PERF-03, PERF-05, PERF-06): Best-effort; tracked but not blocking
- Architecture Boundaries (ARCH-01 through ARCH-06): CI-enforced via static analysis
- Breaking Changes: Permitted with migration documentation (see GOV-07)

Post-1.0: This section will be updated to include formal review processes.

**GOV-02: Standard Enforcement (Solo/Small Team Mode)**  
- Automated CI checks: Block merge on MUST violations where automated
- Self-review checklist: Verify adherence to principles without automated checks
- Exception process (Pre-1.0): Document rationale in commit message; log in BREAKING_CHANGES_v2.md if user-facing
- Exception process (Post-1.0): Will require documented review and migration notes

**GOV-02b: Automated Enforcement Inventory**  
The following principles have CI automation:
- TS-01: Coverage thresholds (≥80% overall, ≥70% per package) via `make coverage-check`
- TS-03: Race detector via `go test -race`
- ARCH-03: Dependency direction via custom linter in `/internal/linter/`

The following are enforced via code review:
- CQ-03: Zero floating-point policy
- CQ-04: Canonical symbol format
- CQ-05: Exhaustive enum mapping
- All UX principles

Expansion roadmap: Add float detection and import path stability checks in Q1 2026.

**GOV-03: Implementation Choices**  
- When principles conflict: Document the conflict and choose the path that best serves the project goals
- When technically infeasible: Provide evidence (benchmark, external constraint); document rationale
- When principles are silent: Follow idiomatic Go practices and established patterns in the codebase

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
- Update process: Collect feedback from production incidents and developer experience; propose changes via spec or direct commit with rationale  
- Version control: All changes to principles are versioned and dated in this document

**GOV-06: Innovation Over Compatibility (MUST)**
Principle: Prioritize shipping new capabilities and architectural improvements over preserving backward compatibility.
Rationale: Sustained innovation keeps the platform competitive even when changes require breaking adjustments.
Implementation:
- When trade-offs arise, select the path that unlocks new capabilities even if it requires a breaking change
- Pair breaking changes with clear migration notes and protocol version updates
- Communicate compatibility impacts in release notes and planning artifacts before rollout

**GOV-07: Breaking Change Documentation (MUST)**
Principle: Refactors that alter import paths or public APIs must include clear migration notes.
Rationale: Enables teams to understand and adopt breaking changes efficiently without backward compatibility constraints.
Implementation:
- Document migration steps in release notes and specs before merging breaking changes
- List all affected import paths, function signatures, and types
- No backward-compatible shims or deprecation cycles required

**GOV-08: Spec-Authorized Breaking Changes (MUST)**  
Principle: Backward-compatibility-breaking refactors are allowed when explicitly stated in the feature spec.  
Rationale: Makes intentional breaks visible and reviewable while preserving overall project velocity.  
Implementation:  
- Spec must include an explicit statement: "This change is backward incompatible"  
- Spec must document impact scope, migration path, and import path changes (if any)  
- Plan must link to migration notes detailing all breaking changes

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

**Version**: 1.6.0 | **Ratified**: 2025-10-08 | **Last Amended**: 2025-10-11

---

## Version History

### v1.6.0 (2025-10-11) - Pre-1.0 Tailoring
**Modified Principles:**
- GOV-01: Clarified pre-1.0 phase decision authority; removed architecture council references
- GOV-02: Simplified enforcement for solo/small team; updated exception process
- GOV-02b: NEW - Added automated enforcement inventory and expansion roadmap
- GOV-03: Simplified implementation choices for solo developer workflow
- GOV-05: Removed RFC requirement; replaced with spec or direct commit
- CQ-08: Renamed to "Interface Stability Roadmap"; added pre-1.0 vs post-1.0 phases
- CQ-09: Added `/internal/observability` as acceptable location for internal helpers
- TS-01: Added coverage exclusions and practical notes for integration-heavy code
- PERF-01: Relaxed from MUST to SHOULD; changed target from <10ms to <100ms
- PERF-02: Removed specific ms targets for pre-1.0 phase
- PERF-07: Relaxed from MUST to SHOULD; changed from strict baselines to trend tracking

**Rationale:** Tailored constitution for pre-1.0, solo/small team environment where breaking changes are expected, ultra-low-latency performance isn't required, and formal governance structures aren't yet established.
