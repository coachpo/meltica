# Feature Specification: Multi-Stream Message Router with Architecture Migration

**Feature Branch**: `003-we-ve-built`  
**Created**: 2025-01-09  
**Status**: Draft  
**Input**: User description: "We've built a high-performance, lightweight Go framework that efficiently handles high-throughput WebSocket message streams. It currently lives in the market_data folder. Its developing documents live in specs/002-build-a-lightweight New requirements: 1. Mixed-flow support [object Object] routing: Support mixed subscription flows where a single stream pushes multiple message types. The framework must route each message type to the appropriate parser, which converts the payload into its corresponding instance/model. 2. Architecture integration [object Object] replacement: After requirement 1 is complete, this framework should replace the current exchanges/binance implementation and conform to the four-level architecture defined in @exchanges/sys_arch_overview.md."

## Clarifications

### Session 2025-01-09

- Q: What should happen when message processing cannot keep up with arrival rate? → A: Apply backpressure to slow down the upstream source (blocks incoming messages)
- Q: What should happen when a processor fails to initialize at startup? → A: Start system but mark that message type as unavailable and route to default handler
- Q: How should the new and old implementations coexist during migration? → A: Single cutover event (no gradual rollout)
- Q: When multiple detection strategies are configured, how should type determination work? → A: First strategy to return a match wins (race condition acceptable)
- Q: How should the framework handle messages that span multiple frames? → A: Connection layer handles frame reassembly transparently (framework sees complete messages)

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Route Mixed Message Types from Single Stream (Priority: P1)

Service developers must receive multiple message types (trades, order books, account updates) on a single connection and have the framework automatically route each message to its appropriate processor so they can handle different event types without manual type discrimination.

**Why this priority**: This is the foundation for efficient stream processing - without automatic routing, teams must manually parse and route every message, defeating the framework's value proposition. This enables the core mixed-flow capability.

**Independent Test**: Connect to a test stream that emits trades, order books, and account updates in random order. Verify each message type is correctly routed to its designated parser and output channels remain segregated by type.

**Acceptance Scenarios**:

1. **Given** a single stream configured to receive trades and order books, **When** the stream emits 100 trade messages and 50 order book snapshots in random order, **Then** all 100 trades arrive at the trade handler and all 50 snapshots arrive at the order book handler without cross-contamination.
2. **Given** a stream emitting mixed message types, **When** an unrecognized message type arrives, **Then** the framework routes it to a default handler or logs a routing error without disrupting processing of recognized types.
3. **Given** multiple concurrent streams each with different message type configurations, **When** messages arrive on all streams simultaneously, **Then** each stream's messages are routed to the correct parser instances without cross-stream interference.

---

### User Story 2 - Convert Payloads to Typed Models (Priority: P1)

Data consumers need messages automatically converted from raw data into strongly-typed domain models for trades, order books, and account updates so they can work with validated, type-safe data structures rather than unstructured formats.

**Why this priority**: Type safety is critical for correctness. Without automatic conversion to typed models, every consumer must implement parsing and validation, leading to duplication, inconsistencies, and bugs.

**Independent Test**: Send raw messages representing different event types and verify the framework outputs strongly-typed instances with all fields correctly populated according to their schema.

**Acceptance Scenarios**:

1. **Given** a trade message in raw format, **When** the framework processes it, **Then** it outputs a trade data instance with price, quantity, side, and exchange trade identifier fields correctly populated as typed values.
2. **Given** an order book update with bids and asks arrays, **When** the framework parses it, **Then** it outputs an order book data instance with typed price level lists where each price and quantity preserves arbitrary-precision decimal accuracy.
3. **Given** an account balance update, **When** the framework converts it, **Then** it outputs an account data instance with balance records containing asset identifiers and total/available amounts as arbitrary-precision decimals.
4. **Given** a malformed payload that fails schema validation, **When** the framework attempts conversion, **Then** it emits a parsing error with diagnostic information while continuing to process subsequent valid messages.

---

### User Story 3 - Adopt Four-Level Architecture (Priority: P2)

Platform architects need the new framework to replace the existing exchange adapter and conform to the four-level architecture (Connection, Routing, Business, Filter) so the system achieves consistent separation of concerns and maintains architectural integrity across all exchange integrations.

**Why this priority**: Architectural conformance ensures maintainability and extensibility. Once mixed-flow routing is proven (P1), migrating to the standard architecture prevents technical debt and enables consistent patterns across exchanges.

**Independent Test**: Deploy the new framework as the exchange adapter and verify all existing functionality (public feeds, private streams, API calls) continues to work while observing that components map cleanly to the four architectural levels.

**Acceptance Scenarios**:

1. **Given** the current exchange adapter handles 10 concurrent public data streams, **When** the new framework replaces it, **Then** all 10 streams maintain connectivity and message throughput without degradation.
2. **Given** the four-level architecture defines Level 1 (Connection), Level 2 (Routing), Level 3 (Business), and Level 4 (Filter), **When** architectural review examines the new implementation, **Then** each component is unambiguously assigned to one level with clear boundaries and no cross-level violations.
3. **Given** existing exchange adapter validation tests covering subscription, unsubscription, authentication, and error handling, **When** tests run against the new framework, **Then** 100% of tests pass without modification to test expectations.
4. **Given** the migration is complete, **When** operators query runtime metrics, **Then** telemetry reports separation by architectural level (connection health at L1, routing latency at L2, business processing at L3, filter throughput at L4).

---

### User Story 4 - Preserve Existing Exchange Functionality (Priority: P2)

Operations teams must have confidence that the framework replacement maintains all current capabilities (session management, authenticated request handling, reconnection logic, subscription lifecycle) so existing services experience zero downtime and no behavioral changes during the migration.

**Why this priority**: Production stability is non-negotiable. Migration cannot occur until all existing functionality is preserved. This runs parallel to P2 architectural adoption since both are required for safe deployment.

**Independent Test**: Run the new framework through the full suite of exchange integration tests and production smoke tests. Verify session keepalive, authenticated requests, and reconnection scenarios all behave identically to the current implementation.

**Acceptance Scenarios**:

1. **Given** the current implementation manages session lifecycle with periodic keepalive, **When** the new framework takes over, **Then** sessions remain valid and private streams stay connected without authentication errors.
2. **Given** authenticated API requests require signature generation, **When** the new framework handles requests, **Then** all signatures validate successfully and requests return expected responses.
3. **Given** connection drops trigger automatic reconnection with exponential backoff, **When** the new framework detects disconnection, **Then** it reconnects using the same backoff policy and resumes subscriptions without data loss.
4. **Given** subscription and unsubscription flows require proper sequencing, **When** the new framework processes these requests, **Then** the exchange acknowledges subscriptions and unsubscriptions in the expected order with correct confirmation messages.

---

### Edge Cases

- When a single stream emits message types faster than processors can consume them, the framework applies backpressure to slow down the upstream source, blocking incoming messages until processing capacity becomes available.
- Messages that span multiple protocol frames are reassembled transparently by the Connection layer (Level 1) before reaching the Routing layer; the framework always operates on complete messages.
- When a processor for a message type fails to initialize at system startup, the system starts successfully but marks that message type as unavailable and routes all matching messages to the default handler until manual intervention.
- **Session renewal during migration**: The framework replacement occurs during a maintenance window when no active listen key renewals are in progress. If deployment must occur mid-renewal cycle, the new framework inherits the existing listen key state and continues the renewal schedule without interruption. The deployment runbook (T052) will include a pre-deployment check to verify time-to-next-renewal > deployment duration.
- **Schema conflicts across stream types**: Message type IDs must be unique across all stream types (public/private). If the same conceptual event (e.g., "trade") has different schemas on public vs private streams, use stream-qualified type IDs: `public_trade` and `private_trade`. The MessageTypeDescriptor.ID field enforces this by including stream context in the identifier when schemas differ. Detection rules use both the message type field AND stream context to disambiguate.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The framework MUST accept configuration specifying which message types (by identifier or schema pattern) are expected on each stream.
- **FR-002**: The framework MUST examine each incoming message to determine its type using a configurable detection strategy (field-based, schema-based, or metadata-based). When multiple strategies are configured for a stream, the first strategy to return a match determines the message type.
- **FR-003**: The framework MUST route each message to the processor registered for its type, maintaining routing tables that map message types to processor instances.
- **FR-004**: The framework MUST support registration of multiple processors, each responsible for converting one message type's raw payload into a corresponding typed model instance. If a processor fails to initialize at system startup, the system MUST start successfully, mark that message type as unavailable, and route all matching messages to the default handler.
- **FR-005**: The framework MUST convert raw payloads into strongly-typed domain models for trades, order books, account updates, and funding rates, preserving all field types including arbitrary-precision decimal values.
- **FR-006**: The framework MUST handle unrecognized message types by routing to a configurable default handler without disrupting processing of other types, and MUST emit routing error events for observability.
- **FR-007**: The framework MUST separate connection lifecycle (connect, reconnect, ping/pong, frame reassembly) as Level 1, message routing and type detection as Level 2, business logic processing as Level 3, and policy filters (throttle, aggregate) as Level 4. Level 1 MUST handle protocol frame reassembly transparently so higher levels always receive complete messages.
- **FR-008**: The framework MUST replace the existing exchange adapter while preserving all current capabilities including session management, authenticated request handling, subscription lifecycle, and reconnection logic.
- **FR-009**: The framework MUST maintain backward compatibility with existing client code consuming the exchange adapter's public interfaces for subscriptions, unsubscriptions, and stream access.
- **FR-010**: The framework MUST emit routing metrics including messages routed per type, routing errors, processor invocation counts, and conversion durations.
- **FR-011**: The framework MUST support concurrent routing where multiple streams emit mixed messages simultaneously without contention on routing tables.
- **FR-012**: The framework MUST apply backpressure to upstream sources when message processing cannot keep up with arrival rate, blocking incoming messages until processing capacity becomes available.
- **FR-013**: The framework replacement MUST occur as a single cutover deployment event with comprehensive validation before and after, rather than gradual rollout with coexistence.

### Key Entities

- **Message Type Descriptor**: Identifies a message category (trade, orderbook, account) with schema metadata and routing configuration.
- **Processor Registration**: Associates a message type descriptor with a processor that converts raw data to typed models.
- **Routing Table**: Maps message type identifiers to registered processors, supporting concurrent lookups across multiple streams.
- **Typed Model Instance**: Strongly-typed data structure representing trades, order books, or account updates produced by processors.
- **Architectural Level Boundary**: Defines the interface contract between Connection (L1), Routing (L2), Business (L3), and Filter (L4) layers.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A single stream emitting 1,000 messages comprising 600 trades, 300 order book updates, and 100 account events routes 100% of messages to their correct processors with zero cross-contamination.
- **SC-002**: Message processing duration from receipt to typed output availability remains under 5 milliseconds at the 95th percentile under sustained load of 50,000 mixed messages per second.
- **SC-003**: The migrated exchange adapter passes 100% of existing validation tests without changes to expected behaviors.
- **SC-004**: Architectural review confirms all components map to exactly one level (L1, L2, L3, or L4) with no cross-level coupling beyond defined interface contracts.
- **SC-005**: Production rollout completes with zero unplanned downtime and no increase in error rates, reconnection frequency, or message loss compared to baseline measurements from the previous week.
- **SC-006**: Developers report that adding a new message type requires only registering a processor and updating routing configuration without modifying core framework capabilities.

## Assumptions

- The existing framework already handles connection management, pooling, and basic message dispatch; this feature extends it with type-aware routing.
- Exchange message types can be distinguished using a field in the message payload (e.g., "event", "type", "stream") or by stream-level configuration.
- Processors for core message types (Trade, OrderBook, Account, Funding) are already available.
- The existing exchange adapter follows the four-level architecture conceptually but requires refactoring to make boundaries explicit and remove cross-level dependencies.
- Migration will occur as a single cutover event after comprehensive validation in non-production environments.

## Glossary

- **Mixed-flow stream**: A single connection carrying multiple message types that require different processing and handling logic.
- **Message type routing**: The process of examining incoming messages to determine their type and dispatching them to the appropriate processor.
- **Typed model**: A strongly-typed data structure that represents a parsed and validated message payload.
- **Four-level architecture**: The system design pattern separating Connection (L1), Routing (L2), Business (L3), and Filter (L4) concerns.
- **Processor registration**: The configuration step that associates message type identifiers with processing handlers.
