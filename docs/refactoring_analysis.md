# Golang Project Refactoring Analysis

## Part 1: Overall Architecture Evaluation
- The `core` module defines protocol constants, shared data transfer objects, and the key extension points for adapters (`Exchange`, `SpotAPI`, `FuturesAPI`, `WS`, symbol registry, transport contracts). These interfaces are mostly segregation-friendly and avoid referencing concrete exchange code, preserving a clean dependency flow from business logic into concrete adapters.【F:core/core.go†L189-L255】【F:core/symbols.go†L9-L57】【F:core/transport/transport_contracts.go†L9-L71】
- The `core/registry` package offers runtime exchange discovery by binding exchange factories and symbol mappers, keeping adapter construction outside of the business layer.【F:core/registry/registry.go†L9-L70】【F:core/registry/binance/binance.go†L9-L34】
- The `exchanges/binance` adapter conforms to the Level 1/2/3 layering described in `exchanges/sys_arch_overview.md`, but the business layer (`Exchange`, `spotAPI`, `linearAPI`, `inverseAPI`, `wsService`) still couples tightly to concrete infra clients (`rest.Client`, `ws.Client`) and local routing packages.【F:exchanges/sys_arch_overview.md†L1-L54】【F:exchanges/binance/binance.go†L21-L82】【F:exchanges/binance/spot.go†L3-L54】
- Interfaces in `core` generally respect dependency inversion (adapters depend on abstractions), yet there are overlaps between the symbol-mapping helpers exposed on the REST/WebSocket interfaces and the global `core.SymbolMapper`, which creates redundant responsibilities for adapters.【F:core/core.go†L229-L255】【F:core/symbols.go†L12-L53】

## Part 2: Issue List
| Priority | Module | Issue Summary | Recommendation |
|----------|--------|---------------|----------------|
| High | `exchanges/binance` (`linearAPI`, `inverseAPI`) | Futures `Ticker` implementations discard REST payloads, returning empty bid/ask data and not honoring the `core.FuturesAPI` contract. | Parse the REST response into `core.Ticker` (bid/ask/last update) similarly to the spot implementation and add tests covering futures tickers.【F:exchanges/binance/linear_futures.go†L26-L37】【F:exchanges/binance/inverse_futures.go†L26-L37】 |
| High | `exchanges/binance` symbol loaders | Symbol conversion helpers call `ensureMarketSymbols` with `context.Background()`, bypassing caller cancellation/timeouts and forcing global symbol loads even for scoped requests. | Thread the caller `ctx` through `SpotNativeSymbol`/`FutureNativeSymbol`/`CanonicalSymbol` to reuse request-scoped deadlines, and consider loading only the needed market instead of all markets.【F:exchanges/binance/spot.go†L185-L206】【F:exchanges/binance/linear_futures.go†L114-L136】【F:exchanges/binance/inverse_futures.go†L114-L136】【F:exchanges/binance/binance.go†L186-L209】 |
| Medium | `core` vs. adapters | REST/WebSocket interfaces expose symbol mapping methods despite the existence of the global `core.SymbolMapper`, pushing duplication and mixed responsibilities into adapters. | Consolidate symbol translation behind `core.SymbolMapper` (or a dedicated adapter) and remove conversion methods from `SpotAPI`/`FuturesAPI`/`WS`, so business code depends on one mapping abstraction.【F:core/core.go†L229-L255】【F:core/symbols.go†L12-L53】【F:core/registry/binance/binance.go†L17-L34】 |
| Medium | `exchanges/binance` | Business layer structs directly instantiate concrete Level 1 clients, making unit testing and alternative transports difficult. | Accept interfaces or configuration structs for REST/WS clients and use dependency injection so tests can supply mocks/fakes, improving adherence to dependency inversion.【F:exchanges/binance/binance.go†L21-L120】【F:exchanges/binance/ws_service.go†L9-L55】 |
| Low | `exchanges/binance` futures & spot APIs | Significant duplication exists between spot/linear/inverse order placement, instrument lookup, and symbol conversion logic. | Extract shared helpers (e.g., generic order builder, market-specific symbol lookup) to reduce maintenance costs and keep SRP-compliant modules.【F:exchanges/binance/spot.go†L108-L144】【F:exchanges/binance/linear_futures.go†L39-L68】【F:exchanges/binance/inverse_futures.go†L39-L68】 |

## Part 3: Detailed Refactoring Suggestions
### 1. Fix Futures `Ticker` Contract (High)
- **Issue:** Both futures implementations dispatch the REST request but ignore the payload (`&struct{}{}`), so callers receive a `Ticker` with zero bids/asks, violating the normalization expected by the interface.【F:exchanges/binance/linear_futures.go†L26-L37】【F:exchanges/binance/inverse_futures.go†L26-L37】 
- **Refactor:** Deserialize the `bidPrice`/`askPrice` (and optionally last update timestamp) exactly as the spot ticker does, using the shared numeric parser. Add table-driven tests for linear & inverse futures tickers.
- **Impact:** Aligns adapters with `core.FuturesAPI` expectations; no interface changes in `core` required.

### 2. Respect Request Context in Symbol Lookups (High)
- **Issue:** Symbol conversion helpers invoke `ensureMarketSymbols`/`ensureAllSymbols` with `context.Background()`, causing potentially long REST calls to ignore upstream cancellations and to load all markets eagerly even when only spot or a single futures market is needed.【F:exchanges/binance/spot.go†L185-L206】【F:exchanges/binance/linear_futures.go†L114-L136】【F:exchanges/binance/inverse_futures.go†L114-L136】【F:exchanges/binance/binance.go†L186-L209】 
- **Refactor:** Pass the incoming request context (or a derived context with timeout) through symbol lookup utilities. Split `ensureAllSymbols` into per-market loaders to avoid unnecessary API calls.
- **Impact:** Improves responsiveness under cancellation and reduces load on exchange APIs; no breaking change to `core` but adapters gain better SRP compliance.

### 3. Unify Symbol Mapping Responsibilities (Medium)
- **Issue:** `SpotAPI`/`FuturesAPI`/`WS` expose conversion methods while a separate `core.SymbolMapper` registry also exists. Adapters must keep both in sync (e.g., `core/registry/binance` registers a mapper that proxies to the exchange), doubling maintenance effort.【F:core/core.go†L229-L255】【F:core/symbols.go†L12-L53】【F:core/registry/binance/binance.go†L17-L34】 
- **Refactor:** Choose a single abstraction—either rely on `core.SymbolMapper` for symbol translation and remove converter methods from the REST/WS interfaces, or define smaller interfaces that adapters implement and register. Update business code accordingly.
- **Impact:** Simplifies adapter responsibilities and clarifies SRP boundaries between transport and symbol normalization. Would require coordinated changes in any consumers using the existing methods.

### 4. Introduce Dependency Injection for Transport Clients (Medium)
- **Issue:** `exchanges/binance.Exchange` constructs `rest.Client`, routers, and websocket clients internally, preventing tests from providing mocks and tightly coupling the business layer to Level 1 implementations.【F:exchanges/binance/binance.go†L55-L120】【F:exchanges/binance/ws_service.go†L9-L55】 
- **Refactor:** Accept interfaces (e.g., `routingrest.RESTDispatcher`, websocket router) or factory functions via constructor options. Provide defaults for production but allow tests to inject fakes.
- **Impact:** Enhances testability and adherence to dependency inversion without altering `core` interfaces.

### 5. Deduplicate Order & Instrument Logic (Low)
- **Issue:** Spot, linear, and inverse APIs repeat order-building, instrument lookup, and numeric formatting logic, raising maintenance risk when Binance changes contract formats.【F:exchanges/binance/spot.go†L108-L144】【F:exchanges/binance/linear_futures.go†L39-L68】【F:exchanges/binance/inverse_futures.go†L39-L68】 
- **Refactor:** Extract shared helpers (e.g., `buildOrderPayload`, `lookupInstrument(ctx, market, symbol)`) or embed a reusable struct per market.
- **Impact:** Reduces duplication; minimal risk if helpers remain internal to the adapter package.

## Part 4: Refactoring Task List
1. **Parse futures ticker payloads and add coverage** (High) – Update `linearAPI` and `inverseAPI` to populate `Ticker` fields and add unit tests.
2. **Propagate contexts through symbol lookup flows** (High) – Change symbol conversion helpers to honor caller contexts and avoid all-market loading.
3. **Decide on a single symbol mapping abstraction** (Medium) – Refactor either the `core` interfaces or the mapper registry to eliminate duplication.
4. **Allow dependency injection for REST/WS transports** (Medium) – Update `Exchange` constructors to accept interfaces or options for mocks.
5. **Extract shared order/instrument helpers** (Low) – Consolidate repeated payload formatting code across spot and futures APIs.
