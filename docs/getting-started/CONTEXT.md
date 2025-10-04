# CONTEXT.md – Meltica Monorepo

This file provides a high‑level, always‑up‑to‑date orientation for the **meltica** repository: a provider‑agnostic cryptocurrency exchange SDK in Go with pluggable adapters and tooling.

> **Purpose:** Deliver a unified trading/data SDK that normalizes REST & WebSocket flows across major exchanges, with schema‑driven validation and conformance tooling.
>
> **Primary language:** Go (1.22+; tested on 1.25+)

---

## Docs quick links

- Full docs index: [README.md](./README.md)
- Start here: [START-HERE.md](./START-HERE.md)
- Project overview: [PROJECT_OVERVIEW.md](./PROJECT_OVERVIEW.md)
- Expectations & standards: [expectations/](./expectations/)
- How-to guides: [how-to/](./how-to/)
- Protocol validation rules: [validation/protocol-validation-rules.md](./validation/protocol-validation-rules.md)

## Repository Overview

* **Unified API** surface across exchanges (spot, linear futures, inverse futures).
* **Adapters** under `exchanges/` translate venue‑specific APIs to canonical `core` interfaces.
* **Examples/CLIs** help stream market data.

### Supported Providers (current targets)

* Binance

---

## Essential Commands

### Getting / Building

```bash
# Install module
go get github.com/coachpo/meltica

# Build the CLI
go build -o meltica ./cmd/main

# Run directly
go run ./cmd/main
```

### Testing & Validation

```bash
# Unit tests (race recommended during dev)
go test ./... -count=1

go test ./... -race -count=1
```

### CLIs for Everyday Work

```bash
# Stream market data
go run ./cmd/main
```

---

## High‑Level Architecture

### Core Modules

* **`core/`** – Canonical domain models & interfaces (`Exchange`, markets, symbols, topics, WS events); capability bitsets; decimal utilities.
* **`exchanges/`** – Venue adapters (REST + WS + signing + error/status mapping) per exchange.
* **`transport/`** – HTTP client with retries, signing hooks; token‑bucket rate limiting; WS helpers.
* **`errs/`** – Unified error envelope and standardized capability errors.
* **`config/`** – Configuration management for exchange adapters.
* **`cmd/`** – CLI entrypoint: `main` for market data streaming.

### Provider Interface (Sketch)

```go
interface Exchange {
  Name() string
  Capabilities() ExchangeCapabilities
  SupportedProtocolVersion() string
  Spot(ctx) SpotAPI
  LinearFutures(ctx) FuturesAPI
  InverseFutures(ctx) FuturesAPI
  WS() WS
  Close() error
}
```

---

## Development Workspace Structure

```
.
├── cmd/                      # CLI tool (main)
├── core/                     # Canonical types, interfaces, symbols, topics, WS events
├── docs/                     # Onboarding, expectations, how‑to, validation rules
├── errs/                     # Unified error definitions
├── exchanges/                # Exchange adapters (binance)
├── transport/                # HTTP/WS transport + rate limiting
├── config/                   # Configuration management
├── Makefile                  # build/test/lint targets
├── go.mod / go.sum           # module + deps
└── README.md                 # quickstart & feature table
```

---

## Core SDK Architecture

### Surfaces

1. **Spot REST / Trading** – instruments, tickers, order placement, balances.
2. **Linear Futures** – positions, orders, PnL; USDT/USDC‑margined.
3. **Inverse Futures** – coin‑margined positions & orders.
4. **WebSocket** – public feeds (trades/ticker/depth) + private feeds (orders/balances).

### Normalization Guarantees

* Canonical symbol formats & topic builders.
* Unified order/status enums and error envelopes.
* Decimal‑first numeric handling (avoid float surprises).

---

## Testing Patterns

* **Unit Tests:** Standard `go test ./...` covering core utilities and adapter logic.
* **Integration Tests:** End-to-end testing with exchange adapters.

---

## Development Infrastructure

### Build & Release

* `make build` compiles the CLI binary.

### Credentials & Env

* Public data requires no credentials.
* Private endpoints (trading, balances) read keys from environment variables per provider.

---

## Exchange Development Workflow (Happy Path)

1. **Implement** REST/WS surfaces, request signing, status & error mappings.
2. **Wire** capability declarations and ensure protocol version is correct.
3. **Test**: unit + integration; use fixtures where applicable.
4. **Build** CLI; smoke‑test via `cmd/main`.

---

## Performance & Reliability Notes

* Token‑bucket rate limiting in `transport/` to protect against venue throttling.
* Decimal math helpers respect instrument scales; avoid binary float for price/size.
* Event decoding aims for zero‑allocation hot paths where possible.

---

## Governance

Project rules that keep Meltica stable and shippable. These are non‑negotiable and CI‑enforced.

### Principles

* **SDK‑first:** `core` defines the truth; adapters conform to them.
* **Frozen surfaces:** Public interfaces, event types, and error envelopes change only with a protocol bump.
* **Normalization:** Canonical symbols (`BASE-QUOTE`), enums (exhaustive), and decimals (`*big.Rat` + `core.FormatDecimal`) across all APIs and events.

### Quality gates (Definition of Done)

1. `go build ./...` and `go test ./...` green (prefer `-race`).
2. Symbols canonical; decimals via `*big.Rat` + `core.FormatDecimal`.
3. Exchange minimal file set present; adapter README lists env vars for optional live tests.

### Versioning & release

* Protocol: SemVer via `core.ProtocolVersion`; adapters **must** return matching version.
* Releases are tagged after all gates pass and docs/readmes are updated.

### Security, observability, reliability

* Secrets redacted; HMAC signing correct; SAST/dep/license scans clean.
* Structured logs/metrics/traces; dashboards for WS stability and order‑flow latency.
* HTTP retries with backoff; token‑bucket rate limiting; WS heartbeats/reconnect; sequence tracking & gap detection.

---

## Contributions

How to contribute and the exact path for adding a new exchange adapter.

### Contributor map

* **Read first:** `docs/START-HERE.md` → overview & expectations.
* **Choose a path:**

  * **Implementing a new exchange** → follow migration guidelines.

### Implementing an Exchange Adapter — Step‑by‑Step

#### A — New Exchange Scaffold & Capabilities

**Goal:** Create the adapter skeleton and wire metadata/capabilities.

* **Do:**

  * Create: `exchanges/<name>/<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go` (if needed), `README.md`.
  * Implement `Exchange` in `<name>.go`; `Name()` stable id; `SupportedProtocolVersion() == core.ProtocolVersion`.
  * Declare `Capabilities()` bitset conservatively; document env vars in adapter README.
* **Validate:**

  ```bash
  go build ./... && go test ./... -count=1
  grep -R "SupportedProtocolVersion" -n exchanges/<name>
  ```

#### B — REST Surfaces: Spot & Futures

**Goal:** Canonical REST APIs for Spot and (optionally) Futures.

* **Do:**

  * Wire HTTP clients with timeouts, retry/backoff, and rate‑limiting hooks.
  * Implement Spot (server time, instruments, ticker, order lifecycle) and Futures (instruments, positions, orders) returning **core models**.
  * Canonicalize symbols to `BASE-QUOTE`; use `*big.Rat` for all numerics; marshal via `core.FormatDecimal`.
  * Map enums/statuses exhaustively with `switch`; default must error.
* **Validate:**

  ```bash
  go build ./... && go test ./exchanges/<name> -count=1
  ```

#### C — WebSocket Public Streams

**Goal:** Implement public WS channels (trades, ticker, depth) with robust reconnect and typed event decoding.

* **Do:**

  * In `exchanges/<name>/ws.go`, implement connect logic with:
    - Reconnect + jittered backoff
    - Heartbeats/pings per provider spec
    - Multiplexed subscriptions (topics like `trades:BTC-USDT`)
  * Preserve subscription list across reconnect.
  * Implement `decodeTradeToEvent`, `decodeTickerToEvent`, `decodeDepthToEvent` returning **concrete** `core.*Event` types.
  * Always convert symbols to canonical `BASE-QUOTE` **before** emitting the event.
  * Attach raw JSON to the event/message `Raw` field if the model supports it.
* **Validate:**

  ```bash
  go test ./exchanges/<name> -run TestPublicWS -timeout 30m
  ```

#### D — WebSocket Private Streams

**Goal:** Implement authenticated/private WS (orders, balances, positions) with sequencing and recovery.

* **Do:**

  * In `ws_private.go`, implement session creation per provider (listen key endpoints, signed subscribe frames, or cookies).
  * Keep‑alive/refresh tokens or listen keys on schedule.
  * Implement decoder functions returning **concrete** `core/ws.OrderEvent` and `core/ws.BalanceEvent` only.
  * Canonicalize symbols before emitting.
  * Track last sequence/ts; on reconnect, fetch deltas via REST if the provider supports it (orders/positions since `lastSeq`).
* **Validate:**

  ```bash
  go test ./exchanges/<name> -run TestPrivateWS -timeout 30m
  ```

#### E — Error/Status Mapping, Symbols, Decimals, Enums

**Goal:** Produce canonical errors and enums; enforce symbol and numeric policies across all code paths.

* **Do:**

  * In `errors.go`, wrap to `*errs.E` with canonical codes; include raw exchange details.
  * In `status.go` (and friends), implement exhaustive `mapOrderStatus/Type/Side/TIF`.
  * Replace any floats with `*big.Rat`; ensure (Un)MarshalJSON uses `core.FormatDecimal`; always call `core.ToCanonicalSymbol`.
* **Validate:**

  ```bash
  go test ./exchanges/<name> -run TestErrorAndStatusGolden -count=1
  ```

#### F — Integration & Release

**Goal:** Make the adapter provably compliant and wired into CI so regressions cannot merge.

* **Do:**

  * Ensure all unit tests pass.
  * Add integration tests for end-to-end flows.
  * CI gates:

    ```bash
    go build ./...
    go test ./... -race -count=1
    ```
  * Protocol guard: fail CI if protocol changed without version bump; adapters must match.
  * Optional live tests gated by documented env vars.
  * Tag release per SemVer; update root README exchange matrix.
* **Validate:** All above commands green locally and in CI; docs updated.

### Pull request checklist (copy/paste)

* [ ] Unit tests (`-race`) green.
* [ ] Symbols canonical; decimals via `*big.Rat` + `core.FormatDecimal`.
* [ ] If protocol touched: docs updated **and** `core.ProtocolVersion` bumped; adapters match.
* [ ] Adapter README includes env vars and base URLs; minimal file set present.

## License

MIT, see `LICENSE` at repo root.

---

## Quick Reference (Copy‑Paste)

```bash
# Stream market data
go run ./cmd/main

# Full test suite
make test   # or: go test ./... -race -count=1
```
