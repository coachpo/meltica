# Data Model: WebSocket Routing Framework Extraction

## Entities

### FrameworkSession
- **Description**: Represents a reusable WebSocket routing session owned by the framework and shared by domain adapters.
- **Attributes**:
  - `ID (string)`: Unique session identifier provided by the domain adapter.
  - `State (enum)`: `initialized | starting | streaming | stopping | stopped`.
  - `Subscriptions ([]SubscriptionSpec)`: Ordered list of active stream subscriptions.
  - `MiddlewareChain ([]MiddlewareFn)`: Ordered middleware callbacks applied to inbound events.
  - `BackoffPolicy (BackoffConfig)`: Configuration for reconnect intervals and jitter.
  - `Logger (LoggerInterface)`: Structured logger implementing existing logging contract.
- **Relationships**:
  - Owns many `SubscriptionSpec` records.
  - References zero or more `MiddlewareFn` supplied by domains.
- **Validation Rules**:
  - `ID` must be non-empty and unique per process.
  - `BackoffPolicy` must define finite max attempts or time budget matching existing defaults.
  - Middleware functions must accept/return framework-defined context and message types.
- **State Transitions**:
  - `initialized → starting`: Triggered by `Start` call.
  - `starting → streaming`: Upon successful connection handshake.
  - `streaming → stopping`: Initiated via graceful shutdown or fatal error.
  - `stopping → stopped`: After cleanup completes with all goroutines exited.
  - Any unexpected error causes `streaming → stopping` with typed error propagation.

### SubscriptionSpec
- **Description**: Defines a single stream subscription managed by the framework.
- **Attributes**:
  - `Exchange (string)`: Exchange identifier (e.g., `binance`).
  - `Channel (string)`: WebSocket channel/topic.
  - `Symbols ([]string)`: Canonical symbols in BASE-QUOTE format.
  - `QoS (enum)`: `realtime | snapshot` following existing domain semantics.
- **Validation Rules**:
  - `Exchange` must map to a registered adapter in `registry`.
  - `Symbols` entries must match regex `^[A-Z0-9]+-[A-Z0-9]+$`.
  - `QoS` defaults to `realtime` when omitted.

### MiddlewareFn
- **Description**: Adapter-supplied hook invoked before events reach domain handlers.
- **Signature**: `func(ctx context.Context, msg *routing.Message) (*routing.Message, error)`.
- **Constraints**:
  - Must return typed errors (`*errs.E`).
  - Must not mutate shared state without synchronization; framework guarantees sequential invocation order.

### DomainAdapter
- **Description**: Market data specific wrapper that binds framework sessions to domain handlers.
- **Attributes**:
  - `Name (string)`: Adapter identifier (e.g., `market_data_ws`).
  - `Framework (FrameworkSession)`: Embedded session reference.
  - `Handlers (map[string]DomainHandler)`: Mapping of message types to business logic.
- **Relationships**:
  - Wraps a single `FrameworkSession` instance.
  - Provides domain handlers consumed by the framework via publish callbacks.
- **Validation Rules**:
  - All handlers must maintain legacy message schemas (`market_data/events` package).
  - Must not import internal framework packages (enforced by architecture lint).

### BackoffConfig
- **Description**: Parameters controlling reconnection strategy.
- **Attributes**:
  - `InitialDelay (time.Duration)`
  - `MaxDelay (time.Duration)`
  - `Multiplier (float64)` formatted as rational to comply with zero-float policy.
  - `JitterPercent (int)` representing integer percentage.
- **Validation Rules**:
  - `InitialDelay` > 0 and <= existing default (e.g., 1s).
  - `MaxDelay` >= `InitialDelay`.
  - `Multiplier` expressed using `big.Rat` to satisfy CQ-03.
  - `JitterPercent` between 0 and 100.

## Derived Relationships
- Each `DomainAdapter` publishes events back to existing market data pipelines via `Publish`, maintaining compatibility with filter layer contracts.
- Contract tests assert that `FrameworkSession.Publish` output equals prior `market_data` router output for identical fixtures.
