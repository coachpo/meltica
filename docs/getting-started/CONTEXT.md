# CONTEXT.md – Meltica Monorepo

This file provides a high‑level, always‑up‑to‑date orientation for the **meltica** repository: a provider‑agnostic cryptocurrency exchange SDK in Go with pluggable adapters and tooling.

> **Purpose:** Deliver a unified trading/data SDK that normalizes REST & WebSocket flows across major exchanges.
>
> **Primary language:** Go (1.22+; tested on 1.25+)

---

## Docs quick links

- Full docs index: [README.md](../README.md)
- Start here: [START-HERE.md](./START-HERE.md)
- Project overview: [PROJECT_OVERVIEW.md](./PROJECT_OVERVIEW.md)
- Expectations & standards: [expectations/](../standards/expectations/)
- How-to guides: [guides/](../guides/)
- Protocol validation rules: [validation/protocol-validation-rules.md](../validation/protocol-validation-rules.md)

## Repository Overview

* **Unified API** surface across exchanges (spot, linear futures, inverse futures).
* **Adapters** under `exchanges/` translate venue‑specific APIs to canonical `core` interfaces.
* **Examples/CLIs** help stream market data.

### Supported Providers (current targets)

* Binance

---

### Essential Commands

### Getting / Building

```bash
# Install module
go get github.com/coachpo/meltica

# Build the project
make build

# Run tests
make test
```

### Testing & Validation

```bash
# Unit tests (race recommended during dev)
go test ./... -count=1

go test ./... -race -count=1
```

### CLIs for Everyday Work

```bash
# Build and run the main CLI
make build
go run ./cmd/main
```

---

## High‑Level Architecture

### Core Modules

* **`core/`** – Canonical domain models & interfaces (`Exchange`, markets, symbols, topics, WS events); capability bitsets.
* **`exchanges/shared/`** – Shared adapters infrastructure (numeric helpers, transport clients, rate limiters, topic mappers).
* **`exchanges/`** – Venue adapters (REST + WS + signing + error/status mapping) per exchange.
* **`errs/`** – Unified error envelope and standardized capability errors.
* **`config/`** – Configuration management for exchange adapters.
* **`cmd/`** – CLI entrypoint: `main` for market data streaming.

### Exchange Interface

```go
interface Exchange {
  Name() string
  Capabilities() ExchangeCapabilities
  SupportedProtocolVersion() string
  Close() error
}

// Market-specific interfaces are implemented as separate participants
interface SpotParticipant {
  Spot(ctx context.Context) SpotAPI
}

interface LinearFuturesParticipant {
  LinearFutures(ctx context.Context) FuturesAPI
}

interface InverseFuturesParticipant {
  InverseFutures(ctx context.Context) FuturesAPI
}

interface WebsocketParticipant {
  WS() WS
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
├── exchanges/                # Exchange adapters (binance, shared)
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

* Rate limiting to protect against venue throttling.
* Decimal math helpers respect instrument scales; avoid binary float for price/size.
* Event decoding aims for zero‑allocation hot paths where possible.

---

## Governance

Project rules that keep Meltica stable and shippable. These are non‑negotiable and CI‑enforced.

### Principles

* **SDK‑first:** `core` defines the truth; adapters conform to them.
* **Frozen surfaces:** Public interfaces, event types, and error envelopes change only with a protocol bump.
* **Normalization:** Canonical symbols (`BASE-QUOTE`), enums (exhaustive), and decimals (`*big.Rat`) across all APIs and events.

### Quality gates (Definition of Done)

1. `go build ./...` and `go test ./...` green (prefer `-race`).
2. Symbols canonical; decimals via `*big.Rat`.
3. Exchange minimal file set present; adapter README lists env vars for optional live tests.

### Versioning & release

* Protocol: SemVer via `core.ProtocolVersion`; adapters **must** return matching version.
* Releases are tagged after all gates pass and docs/readmes are updated.

### Security, observability, reliability

* Secrets redacted; HMAC signing correct; dependency/license scans clean.
* Structured logs/metrics/traces; dashboards for WS stability and order‑flow latency.
* HTTP retries with backoff; rate limiting; WS heartbeats/reconnect; sequence tracking & gap detection.

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

  * Create: `exchanges/<name>/<name>.go`, `exchange/provider.go`, `infra/rest/client.go`, `infra/ws/client.go`, `README.md`.
  * Implement `Exchange` in `<name>.go`; `Name()`