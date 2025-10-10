## Binance Routing Audit (T033)

### Current Architecture Overview

- **dispatchers.go**
  - `PublicDispatcher` manages websocket subscriptions for all public topics.
  - Performs stream derivation (`buildStreams`) translating canonical topics to Binance-native stream names and subscribes via `infra.Subscribe`.
  - Runs `pumpPublic` goroutine that:
    - Receives raw websocket payloads.
    - Unwraps combined stream envelopes (`unwrapCombinedPayload`).
    - Parses the payload directly (`json.Unmarshal`) to determine event type.
    - Delegates to `StreamRegistry.Dispatch` for type-specific decoding.
  - Implements backpressure by dropping the oldest buffered message when channel full (custom logic outside framework flow control).

- **PrivateDispatcher** mirrors public dispatcher but:
  - Manages listen key lifecycle (create, keepalive, close) at dispatcher level.
  - Directly parses payload metadata (`event` field) before dispatching to registry.

- **StreamRegistry**
  - Defines routing table via hardcoded handlers keyed by `core.Topic` enums.
  - Performs JSON decoding and transformation into core domain events (`parseTradeEvent`, `parseTickerEvent`, etc.).
  - Couples Binance-specific parsing with routing decisions (violates planned L2/L3 separation).

- **ws_router.go**
  - Exposes `WSRouter` facade combining public/private dispatchers.
  - Returns `corestreams.RoutedMessage` channels directly to consumers.

### Boundary Violations

| Layer | Expected Responsibility | Current Violation |
|-------|------------------------|-------------------|
| L1 (Connection) | Transport subscriptions only | Dispatchers drop messages & manage backpressure manually. |
| L2 (Routing) | Detect + dispatch message types | StreamRegistry mixes detection with parsing/transform. |
| L3 (Processors) | Typed model conversion | Handlers in StreamRegistry perform conversion inline. |
| L4 (Policy) | Business logic filters | Filter package references routing helpers for parsing decisions. |

Specific findings:

1. `PublicDispatcher.parsePublicMessage` and `PrivateDispatcher.parsePrivateMessage` decode payload metadata using ad-hoc JSON parsing instead of reusable detection rules.
2. Backpressure management in `pumpPublic` bypasses framework flow controller, leading to silent drops.
3. `StreamRegistry.Dispatch` depends on `WSDependencies` (symbol translation), coupling messaging and domain adaptation.
4. Filters under `exchanges/binance/filter` import routing helpers (`Parse`, `Trade`, etc.), suggesting L4 ↔ L2 coupling.

### Refactoring Plan

1. **Message Type Descriptors (T034)**
   - Extract detection rules into new `message_types.go` describing Binance events for framework registration.
   - Normalize detection to rely on envelope fields (`stream`, `data.e`).

2. **Processor Adapters (T035)**
   - Wrap existing handlers (`parseTradeEvent`, `parseOrderbookEvent`, etc.) behind Processor implementations.
   - Inject required dependencies (symbol translation, converters) via struct fields rather than global registry.

3. **Routing Integration (T036)**
   - Replace `StreamRegistry` dispatch with framework `RoutingTable` + `RouterDispatcher`.
   - Public/Private dispatchers become shims that subscribe over transport and forward raw payloads to framework dispatcher channels.
   - Utilize framework backpressure metrics instead of drop-on-full logic.

4. **Boundary Realignment (T037)**
   - Move any residual transformation logic out of filters; ensure filters receive typed events only.

5. **Plugin Wiring (T038)**
   - Instantiate routing table inside Binance plugin, register descriptors/processors, and expose typed inboxes to dispatchers.

### Work Breakdown Dependencies

- Complete extraction of detection rules and processor adapters before touching dispatchers.
- Incrementally replace `StreamRegistry` with framework components while maintaining existing tests (`order_integration_test.go`).
- Update filters and plugin wiring only after routing table integration compiles.

### Testing Strategy

- Preserve existing `order_integration_test.go` and `ws_router_test.go` as regression coverage.
- Introduce new framework integration tests (T039) verifying descriptor-driven detection for representative messages (trade, orderbook, balance, order update).
- Run full exchange integration suite (T040) once routing replacement complete.

This audit satisfies T033 by documenting current routing behavior, identifying layer boundary violations, and outlining a concrete refactoring plan aligned with the four-level architecture.
