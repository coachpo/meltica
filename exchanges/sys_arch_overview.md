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
Creates exchange-agnostic market-data pipelines that orchestrate Level 3 services, apply policy filters, and expose normalized streams to downstream clients. Acts as a channel-agnostic façade that orchestrates all exchange interactions (public market data, private websocket topics, and REST request/response flows).

**Responsibilities:**
- Coordinates multi-channel sourcing across exchanges via adapters that expose declared capabilities (public feeds, private streams, REST endpoints).
- Normalizes events into canonical envelopes (symbol, timestamp, payload, channel context) regardless of exchange-specific quirks.
- Applies filter policies such as throttling, deduplication, enrichment, aggregation, and selective fan-out.
- Manages lifecycle concerns for feed pipelines: context propagation, retries, error fan-in, and graceful teardown.
- Provides a single facade for interactive CLIs and services to consume filtered market data without touching Level 3 primitives directly.
- Orchestrates authentication context for private streams and correlation IDs for REST request/response tracking.

**Channel Types:**
- `public_ws` – Public WebSocket streams (order books, trades, tickers)
- `private_ws` – Private WebSocket streams (account updates, order events)
- `rest` – REST API request/response flows
- `hybrid` – Mixed channel workflows

**Stage Catalog:**
- `MultiSource` – multiplexes mixed channel sources (public feeds, private streams, REST requests) into typed client events with channel context.
- `Normalize` – enforces canonical symbol casing, guarantees timestamps, and prepares events for downstream policy stages.
- `Throttle` – enforces minimum emit intervals per symbol/kind to prevent overload from bursty feeds.
- `Aggregate` – trims order-book depth, updates snapshot caches, and prepares derived data for observers.
- `VWAP` – maintains running volume-weighted average price analytics emitted alongside raw trade flow.
- `Reliability` – wraps upstream errors with pipeline context and preserves ordering guarantees.
- `Observer` – emits structured callbacks for metrics/logging hooks without blocking the data path.
- `Sampling` – performs time-based down-sampling when requested (useful for UI dashboards or logs).
- `Dispatch` – final guard that ensures non-nil output channels even when earlier stages short-circuit.

**Pipeline Event Contract:**
- Level 3 adapters emit `pipeline.Event` values that pair a `TransportType` (public, private, REST, hybrid) with a strongly typed payload (`BookPayload`, `TradePayload`, `AccountPayload`, `RestResponsePayload`, etc.).
- Level 4 stages operate exclusively on these payloads and ultimately surface `ClientEvent` to consumers; downstream code must switch on payload type rather than on string-based `EventKind`.
- When introducing new payloads, add them to `pipeline/payloads.go`, implement `isPayload`, and update any stages that need awareness of the new type.

### Registering Pipeline Adapters & Stages

1. Implement the `pipeline.Adapter` interface inside the exchange plugin. Declare supported capabilities via `Capabilities()` including public feeds (`Books`, `Trades`, `Tickers`), private streams (`PrivateStreams`), and REST endpoints (`RESTEndpoints`).
2. Surface channel-based sources for each feed type (`BookSources`, `TradeSources`, `TickerSources`, `PrivateSources`) and implement REST execution (`ExecuteREST`). Public feeds still stream `corestreams` events, while private/REST flows now emit `pipeline.Event` payloads that carry transport metadata for Level 4 assembly into `ClientEvent`s.
3. Wire the adapter into consumers (such as `cmd/market_data`) by instantiating `pipeline.NewInteractionFacade(adapter, auth)` and using high-level methods (`SubscribePublic`, `SubscribePrivate`, `FetchREST`). For advanced use cases, use `pipeline.NewCoordinator(adapter, auth)` directly.
4. To extend filtering behaviour, create a new `pipeline.Stage` (via `pipeline.NewStageFunc`) that transforms, enriches, or routes `ClientEvent` streams. Update the coordinator's stage builder to insert the new stage where appropriate or conditionally add it based on `PipelineRequest` flags.
5. Add integration tests that exercise the adapter with recorded fixtures to ensure stage orchestration remains stable, and unit tests for any new stages to verify ordering, error propagation, and cancellation semantics.

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

---

## **Multi-Channel Level-4 Architecture (Latest)**

### **Overview**
The Level-4 Filter Layer has been elevated to a channel-agnostic façade that orchestrates all exchange interactions:
- **Public WebSocket** streams (order books, trades, tickers)
- **Private WebSocket** streams (account updates, order events)
- **REST API** request/response flows
- **Mixed channel** workflows combining multiple interaction types

### **Key Components**

**InteractionFacade**
- High-level client API: `SubscribePublic()`, `SubscribePrivate()`, `FetchREST()`
- Manages authentication context and correlation IDs
- Provides backward compatibility with existing public feed usage

**Multi-Source Stage**
- Orchestrates mixed channel sources concurrently
- Handles different lifecycle patterns (streaming vs request/response)
- Preserves channel context throughout the pipeline

**Enhanced Adapter Interface**
- `Capabilities()` declares support for PrivateStreams and RESTEndpoints
- `PrivateSources()` provides private stream event channels
- `ExecuteREST()` handles REST request/response flows
- `InitPrivateSession()` manages authentication for private streams

**Event Envelope Extensions**
- `Channel` field identifies source (public_ws, private_ws, rest)
- `CorrelationID` tracks request/response pairs
- Introduced typed `ClientEvent` payloads (Account, Order, RestResponse, Analytics)
- New payload types (AccountEvent, OrderEvent, RestResponse)

### **Usage Examples**

```go
// Public market data only (existing usage)
facade := filter.NewInteractionFacade(adapter, nil)
stream, err := facade.SubscribePublic(ctx, []string{"BTC-USDT"},
    filter.WithBooks(), filter.WithTrades())

// Private account streams
facade := filter.NewInteractionFacade(adapter, auth)
stream, err := facade.SubscribePrivate(ctx)

// REST API calls
requests := []filter.InteractionRequest{
    filter.GetAccountInfo("req-1"),
    filter.GetOpenOrders("BTC-USDT", "req-2"),
}
stream, err := facade.FetchREST(ctx, requests)

// Mixed workflow
coordinator := filter.NewCoordinator(adapter, auth)
req := filter.FilterRequest{
    Symbols: []string{"BTC-USDT"},
    Feeds: filter.FeedSelection{Books: true, Trades: true},
    EnablePrivate: true,
    RESTRequests: []filter.InteractionRequest{
        filter.GetAccountInfo("mixed-req-1"),
    },
}
stream, err := coordinator.Stream(ctx, req)
```

### **Benefits**
- **Unified Interface**: Clients interact only with Level-4, eliminating direct REST calls or private WS usage
- **Channel Context**: Events carry source channel information for proper handling
- **Request Tracking**: Correlation IDs enable request/response pairing
- **Flexible Orchestration**: Mixed workflows combine streaming and request/response patterns
- **Backward Compatible**: Existing public feed usage continues unchanged
