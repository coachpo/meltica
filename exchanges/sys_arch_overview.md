# System Architecture Overview

The system is structured into four logical layers — **Level 1**, **Level 2**, **Level 3**, and **Level 4** — each responsible for a specific aspect of communication and business processing.
This layered structure supports both WebSocket and HTTP REST interfaces while maintaining consistent message flow and error handling.

---

## **Level 1 – Connection Layer**

**Purpose:**
Provides the fundamental infrastructure for network communication.

**Responsibilities:**
- Manages physical connections for both **WebSocket** and **REST** clients.
- Handles connection lifecycle operations: **connect**, **reconnect**, and **ping/pong** (for WebSocket).
- Acts as the entry and exit point for all inbound and outbound data.
- Handles request signing for authenticated API calls.
- Reports connection-related errors and exchange errors to Level 2 for parsing and routing.

---

## **Level 2 – Routing Layer**

**Purpose:**
Handles message routing, translation, and request/response formatting between the connection layer and business layer.

**Responsibilities:**
- **WebSocket:**
  - Manages **subscriptions** and **unsubscriptions** through separate Public and Private dispatchers.
  - Routes incoming messages to the correct business handler using a stream registry.
  - Converts raw WebSocket messages into a standardized internal format for business processing.
  - Manages listen key lifecycle and keepalive for private streams.
- **REST:**
  - Builds and formats **HTTP requests** (URL, path, headers, payload).
  - Converts raw **HTTP responses** into a standardized internal format for business processing.

**Error Handling:**
- Receives error notifications from Level 1.
- Parses and categorizes errors (e.g., connection issues, protocol violations, invalid responses, exchange API errors).
- Forwards structured error information to Level 3 for appropriate handling.

---

## **Level 3 – Business Layer**

**Purpose:**
Implements the domain and business logic.

**Responsibilities:**
- Generates business-level requests and passes them to **Level 2**.
- Processes normalized responses and messages received from **Level 2**.
- Distributes results to client interfaces, services, or other consumers.
- Handles error messages from Level 2, performing recovery actions, user notifications, or logging based on business rules.

---

## **Level 4 – Filter Layer**

**Purpose:**
Creates exchange-agnostic market-data pipelines that orchestrate Level 3 services, apply policy filters, and expose normalized streams to downstream clients.

**Responsibilities:**
- Coordinates feed sourcing across exchanges via adapters that expose declared capabilities.
- Normalizes events into canonical envelopes (symbol, timestamp, payload) regardless of exchange-specific quirks.
- Applies filter policies such as throttling, deduplication, enrichment, aggregation, and selective fan-out.
- Manages lifecycle concerns for feed pipelines: context propagation, retries, error fan-in, and graceful teardown.
- Provides a single facade for interactive CLIs and services to consume filtered market data without touching Level 3 primitives directly.

**Stage Catalog:**
- `Source` – multiplexes exchange-native feed channels (books, trades, tickers) into canonical event envelopes.
- `Normalize` – enforces canonical symbol casing, guarantees timestamps, and prepares events for downstream policy stages.
- `Throttle` – enforces minimum emit intervals per symbol/kind to prevent overload from bursty feeds.
- `Aggregate` – trims order-book depth, updates snapshot caches, and prepares derived data for observers.
- `VWAP` – maintains running volume-weighted average price analytics emitted alongside raw trade flow.
- `Reliability` – wraps upstream errors with pipeline context and preserves ordering guarantees.
- `Observer` – emits structured callbacks for metrics/logging hooks without blocking the data path.
- `Sampling` – performs time-based down-sampling when requested (useful for UI dashboards or logs).
- `Dispatch` – final guard that ensures non-nil output channels even when earlier stages short-circuit.

### Registering Filter Adapters & Stages

1. Implement the `marketdata/filter.Adapter` interface inside the exchange plugin. Declare supported feeds via `Capabilities()` and surface channel-based sources for each feed type you expose (`BookSources`, `TradeSources`, `TickerSources`). The adapter should translate Level 3 services (for example, Binance `OrderBookService.Subscribe`) into canonical channels of `corestreams` events.
2. Wire the adapter into consumers (such as `cmd/market_data`) by instantiating `filter.NewCoordinator(adapter)` and issuing a `FilterRequest`. The coordinator will validate capabilities before building the stage pipeline.
3. To extend filtering behaviour, create a new `filter.Stage` (via `filter.NewStageFunc`) that transforms, enriches, or routes `EventEnvelope` streams. Update the coordinator’s stage builder to insert the new stage where appropriate or conditionally add it based on `FilterRequest` flags.
4. Add integration tests that exercise the adapter with recorded fixtures to ensure stage orchestration remains stable, and unit tests for any new stages to verify ordering, error propagation, and cancellation semantics.

---

## **Data Flow Summary**

| Communication Type | Level 1 | Level 2 | Level 3 | Level 4 | Description |
|--------------------|----------|----------|----------|----------|--------------|
| **WebSocket** | Connection management (connect, reconnect, ping/pong) | Message routing via dispatchers, subscription/unsubscription, message conversion via stream registry | Business logic, request generation | Pipeline orchestration, normalization, filtering, fan-out | Continuous, bidirectional stream |
| **REST (HTTP)** | Connection handling, request signing | Request building & response normalization | Business logic, request initiation | Snapshot hydration, enrichment, aggregation before distribution | Stateless, point-to-point request/response |

---

## **Breaking Changes in Latest Refactor**

### **1. TransportConfig Consolidation**
**Previous:** Factory functions accepted exchange-specific config types (`rest.Config`, `ws.Config`)
**Current:** Factory functions now accept unified `bootstrap.TransportConfig` struct

**Migration:**
```go
// Before
params.Transports = bootstrap.TransportFactories{
    NewRESTClient: func(cfg interface{}) coretransport.RESTClient {
        return rest.NewClient(cfg.(rest.Config))
    },
}

// After
params.Transports = bootstrap.TransportFactories{
    NewRESTClient: func(cfg bootstrap.TransportConfig) coretransport.RESTClient {
        return rest.NewClient(rest.Config{
            APIKey:         cfg.APIKey,
            Secret:         cfg.Secret,
            SpotBaseURL:    cfg.SpotBaseURL,
            // ... map fields from TransportConfig
        })
    },
}
```

### **2. BuildTransportBundle Signature**
**Previous:** `BuildTransportBundle(transports, routers, restCfg, wsCfg)`
**Current:** `BuildTransportBundle(transports, routers, transportCfg)`

**Migration:**
```go
// Before
bundle := bootstrap.BuildTransportBundle(transports, routers, restCfg, wsCfg)

// After
transportCfg := bootstrap.TransportConfig{
    APIKey:         credentials.APIKey,
    Secret:         credentials.Secret,
    SpotBaseURL:    restURLs.Spot,
    PublicURL:      wsURLs.Public,
    PrivateURL:     wsURLs.Private,
    // ... all transport config in one place
}
bundle := bootstrap.BuildTransportBundle(transports, routers, transportCfg)
```

### **3. WebSocket Router Architecture**
**Previous:** Monolithic `WSRouter` with mixed public/private logic
**Current:** Separate `PublicDispatcher` and `PrivateDispatcher` with stream registry

**Changes:**
- WSRouter delegates to specialized dispatchers for public and private streams
- Stream parsing uses a registry pattern instead of string matching
- Listen key keepalive is now encapsulated in PrivateDispatcher
- WSRouter.Close() no longer closes the infrastructure client (managed by TransportBundle)

**Impact:**
- Tests expecting WSRouter.Close() to close infrastructure should be updated
- Custom router implementations should follow the new dispatcher pattern
- Stream handlers are now registered in StreamRegistry for extensibility

### **4. Option Hooks**
**Previous:** `WithRESTClientFactory(func(rest.Config) ...)` and `WithWSClientFactory(func(ws.Config) ...)`
**Current:** `WithRESTClientFactory(func(bootstrap.TransportConfig) ...)` and `WithWSClientFactory(func(bootstrap.TransportConfig) ...)`

**Migration:**
All option hooks now use the unified TransportConfig type.

### **5. Removed Files**
The following files have been removed and their functionality consolidated:
- `exchanges/binance/routing/parse_public.go` → functionality moved to `dispatchers.go`, `parse_helpers.go`, `stream_registry.go`
- `exchanges/binance/routing/parse_private.go` → functionality moved to `dispatchers.go`, `parse_helpers.go`, `stream_registry.go`

### **Migration Path**
1. Update custom transport/router factories to accept `bootstrap.TransportConfig`
2. Update calls to `BuildTransportBundle` to use unified config
3. Update any custom option hooks to use new signatures
4. If implementing custom routers, follow the new dispatcher pattern
5. Update tests that assume WSRouter.Close() closes infrastructure client

**Benefits:**
- Cleaner separation of concerns with Public/Private dispatchers
- More maintainable message parsing via stream registry
- Unified configuration reduces coupling between REST and WebSocket setup
- Easier to extend with new stream types
