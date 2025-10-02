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
* **Adapters** under `providers/` translate venue‑specific APIs to canonical `core` interfaces.
* **Protocol** (JSON Schemas + golden vectors) locks wire formats and event shapes.
* **Tooling** (CLI binaries + linter) ensures capability declarations and protocol compliance.
* **Examples/CLIs** help stream market data, validate schemas, and scaffold new providers.

### Supported Providers (current targets)

* Binance • OKX • Coinbase • Kraken

---

## Essential Commands

### Getting / Building

```bash
# Install module
go get github.com/coachpo/meltica

# Build all tools to ./out
make build

# Build individual tools
go build -o market-stream ./cmd/market-stream
go build -o validate-schemas ./cmd/validate-schemas
go build -o barista ./cmd/barista
```

### Testing & Validation

```bash
# Unit tests (race recommended during dev)
go test ./... -count=1

go test ./... -race -count=1

# Conformance (requires API keys for some checks)
export MELTICA_CONFORMANCE=1
export BINANCE_KEY=...; export BINANCE_SECRET=...
go test ./internal/test -count=1

# Schema + vector validation
go run ./cmd/validate-schemas

# Protocol linter
go build ./internal/meltilint/cmd/meltilint
./meltilint ./providers/...
```

### CLIs for Everyday Work

```bash
# Stream market data
go run ./cmd/market-stream --exchange binance --symbol BTC-USDT --channel trades

# Scaffolding a new provider
go run ./cmd/barista -name bybit

# Validate protocol schemas
go run ./cmd/validate-schemas
```

---

## High‑Level Architecture

### Core Modules

* **`core/`** – Canonical domain models & interfaces (`Provider`, markets, symbols, topics, WS events); capability bitsets; decimal utilities.
* **`providers/`** – Venue adapters (REST + WS + signing + error/status mapping) per exchange.
* **`protocol/`** – JSON Schemas for provider descriptors & WS events; sample vectors; protocol semantic version.
* **`transport/`** – HTTP client with retries, signing hooks; token‑bucket rate limiting; WS helpers.
* **`errs/`** – Unified error envelope and standardized capability errors.
* **`conformance/`** – Harness to compile schemas, validate vectors, and verify capability declarations.
* **`internal/meltilint/`** – Static analysis enforcing protocol and project standards (CLI included).
* **`cmd/`** – CLI entrypoints: `market-stream`, `barista`, `validate-schemas`.

### Data & Event Contract (Protocol)

* JSON Schema‑backed event types: trades, ticker, depth, balances, orders.
* Golden vectors assert stable, normalized payloads across providers.
* Versioned via `protocol/version.go`.

### Provider Interface (Sketch)

```go
interface Provider {
  Name() string
  Capabilities() ProviderCapabilities
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
├── cmd/                      # CLI tools (barista, market-stream, validate-schemas)
├── conformance/              # Schema & vector validation + capability verification
├── core/                     # Canonical types, interfaces, symbols, topics, WS events
├── docs/                     # Onboarding, expectations, how‑to, validation rules
├── errs/                     # Unified error definitions
├── internal/
│   ├── meltilint/            # Protocol/project linter + CLI
│   └── test/                 # Golden/conformance tests & fixtures
├── protocol/                 # JSON Schemas, vectors, protocol version
├── providers/                # Exchange adapters (binance, coinbase, kraken, okx, ...)
├── transport/                # HTTP/WS transport + rate limiting
├── out/                      # Built binaries (git‑ignored)
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
* **Golden Vectors:** Fixture comparison for provider payloads (e.g., ticker, depth).
* **Conformance Harness:** Ensures declared capabilities match implemented surfaces; validates schemas/vectors.
* **Race & CI:** Prefer `-race`; wire into CI to run unit + conformance + meltilint.

---

## Development Infrastructure

### Build & Release

* `make build` compiles all binaries into `./out/`.
* Prebuilt convenience binaries may live under `market-stream/`.

### Lint & Quality

* **meltilint** enforces: protocol version alignment, doc presence, float detection, error invariants, core interface immutability.
* Go fmt/vet + (optional) golangci‑lint can be layered on top.

### Credentials & Env

* Public data requires no credentials.
* Private endpoints (trading, balances) read keys from environment variables per provider.

---

## Asset & Content Management

### Protocol Assets

* **Schemas:** `protocol/schemas/*.json` for provider descriptors & WS events.
* **Vectors:** `protocol/vectors/*.json` sample payloads that back golden tests.
* **Rules:** Validation rulebook under `docs/validation/` and expectation docs under `docs/expectations/`.

---

## Provider Development Workflow (Happy Path)

1. **Scaffold** a provider with `barista` → `providers/<exchange>/` package skeleton.
2. **Implement** REST/WS surfaces, request signing, status & error mappings.
3. **Wire** capability declarations and ensure protocol version is correct.
4. **Add** schemas/vectors and **run** `validate-schemas`.
5. **Test**: unit + golden + conformance; use `internal/test` fixtures where applicable.
6. **Lint** with `meltilint`; fix violations.
7. **Build** CLIs; smoke‑test via `market-stream`.

---

## Performance & Reliability Notes

* Token‑bucket rate limiting in `transport/` to protect against venue throttling.
* Decimal math helpers respect instrument scales; avoid binary float for price/size.
* Event decoding aims for zero‑allocation hot paths where possible.

---

## Governance

Project rules that keep Meltica stable and shippable. These are non‑negotiable and CI‑enforced.

### Principles

* **SDK‑first:** `core` and protocol define the truth; adapters conform to them.
* **Frozen surfaces:** Public interfaces, event types, and error envelopes change only with a protocol bump.
* **Normalization:** Canonical symbols (`BASE-QUOTE`), enums (exhaustive), and decimals (`*big.Rat` + `core.FormatDecimal`) across all APIs and events.
* **Provable compliance:** Machine‑verifiable via conformance harness, schema checks, and lints.

### Quality gates (Definition of Done)

1. `go build ./...` and `go test ./...` green (prefer `-race`).
2. `./meltilint ./core ./providers/...` green (no floats in public surfaces, exhaustive enums/status mapping, typed WS events, protocol alignment).
3. `go run ./cmd/validate-schemas` green (schemas + golden vectors validate).
4. Offline conformance suite green: `go test ./conformance -run TestOffline`.
5. **Protocol version guard:** Any protocol change requires bump in `protocol/version.go` and matching `SupportedProtocolVersion()` in each adapter.
6. Provider minimal file set present; adapter README lists env vars for optional live tests.

### Versioning & release

* Protocol: SemVer via `protocol/version.go`; adapters **must** return matching version.
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

  * **Implementing a new provider** → follow Steps **A→F** below.
  * **Migrating a provider** → see migration guidelines, then re‑run Steps **A→F**.

### Implementing a Provider Adapter — Step‑by‑Step (Teams A → F)

#### A — 01 New Exchange Scaffold & Capabilities

**Goal:** Create the adapter skeleton and wire metadata/capabilities.

* **Do:**

  * `go run ./cmd/barista --name <name>` → creates: `<name>.go`, `sign.go`, `errors.go`, `status.go`, `ws.go`, `ws_private.go` (if needed), `conformance_test.go`, `golden_test.go`, `README.md`.
  * Implement `Provider` in `<name>.go`; `Name()` stable id; `SupportedProtocolVersion() == protocol.ProtocolVersion`.
  * Declare `Capabilities()` bitset conservatively; document env vars in adapter README.
* **Validate:**

  ```bash
  go build ./... && go test ./... -count=1
  go build ./internal/meltilint/cmd/meltilint && ./meltilint ./core ./providers/<name>
  grep -R "SupportedProtocolVersion" -n providers/<name>
  ```

#### B — 02 REST Surfaces: Spot & Futures

**Goal:** Canonical REST APIs for Spot and (optionally) Futures.

* **Do:**

  * Wire HTTP clients with timeouts, retry/backoff, and rate‑limiting hooks.
  * Implement Spot (server time, instruments, ticker, order lifecycle) and Futures (instruments, positions, orders) returning **core models**.
  * Canonicalize symbols to `BASE-QUOTE`; use `*big.Rat` for all numerics; marshal via `core.FormatDecimal`.
  * Map enums/statuses exhaustively with `switch`; default must error.
* **Validate:**

  ```bash
  ./meltilint ./providers/<name>
  go build ./... && go test ./providers/<name> -count=1
  go test ./conformance -run TestOffline
  go run ./cmd/validate-schemas
  ```

#### C — 03 WebSocket Public Streams

**Goal:** Trades, ticker, depth with robust reconnect and typed decoding.

* **Do:**

  * Implement `ws.go` client with jittered backoff, heartbeats, multiplexed subscriptions, and resubscribe on reconnect.
  * Decode to concrete `core/ws.TradeEvent`, `core/ws.TickerEvent`, `core/ws.DepthEvent`; canonicalize symbols **before** emitting; attach raw JSON where supported.
  * Track sequence numbers and log/report gaps.
* **Validate:**

  ```bash
  ./meltilint ./providers/<name>
  go test ./conformance -run TestOffline
  # optional soak
  go test ./providers/<name> -run TestPublicWS -timeout 30m
  ```

#### D — 04 WebSocket Private Streams

**Goal:** Authenticated streams (orders, balances, positions) with sequencing & recovery.

* **Do:**

  * Implement `ws_private.go` session/auth (listen keys or signed frames); keep‑alive/refresh.
  * Decode to **concrete** `core/ws.OrderEvent` / `core/ws.BalanceEvent`; canonicalize symbols.
  * Track last seq/timestamp; on reconnect, fetch deltas via REST where supported; dedupe.
* **Validate:**

  ```bash
  ./meltilint ./providers/<name>
  go test ./conformance -run TestOffline
  # optional live (gated)
  export MELTICA_CONFORMANCE=1 && go test ./internal/test -run <Provider>Conformance -v
  ```

#### E — 05 Error/Status Mapping, Symbols, Decimals, Enums

**Goal:** Canonical errors/enums; enforce symbols & numerics policy everywhere.

* **Do:**

  * In `errors.go`, wrap to `*errs.E` with canonical codes; include raw exchange details.
  * In `status.go` (and friends), implement exhaustive `mapOrderStatus/Type/Side/TIF`.
  * Replace any floats with `*big.Rat`; ensure (Un)MarshalJSON uses `core.FormatDecimal`; always call `core.ToCanonicalSymbol`.
* **Validate:**

  ```bash
  ./meltilint ./providers/<name>
  go test ./providers/<name> -run TestErrorAndStatusGolden -count=1
  go run ./cmd/validate-schemas
  ```

#### F — 06 Conformance, Schemas, CI & Release

**Goal:** Make the adapter provably compliant and wired into CI.

* **Do:**

  * In `providers/<name>/conformance_test.go`:

    ```go
    conformance.RunAll(t, factory, conformance.Options{})
    ```
  * Ensure schemas exist for emitted models/events; validate vectors:

    ```bash
    go run ./cmd/validate-schemas
    ```
  * CI gates:

    ```bash
    go build ./...
    go test ./... -race -count=1
    go build ./internal/meltilint/cmd/meltilint && ./meltilint ./core ./providers/...
    go test ./conformance -run TestOffline
    ```
  * Protocol guard: fail CI if protocol changed without version bump; adapters must match.
  * Optional live tests gated by `MELTICA_CONFORMANCE=1` and documented env vars.
  * Tag release per SemVer; update root README provider matrix.
* **Validate:** All above commands green locally and in CI; docs updated.

### Pull request checklist (copy/paste)

* [ ] Unit tests (`-race`) green.
* [ ] `meltilint` green; no floats; enums exhaustive; typed events.
* [ ] Schemas & vectors validated; offline conformance green.
* [ ] Symbols canonical; decimals via `*big.Rat` + `core.FormatDecimal`.
* [ ] If protocol touched: docs + schemas + vectors updated **and** `protocol/version.go` bumped; adapters match.
* [ ] Adapter README includes env vars and base URLs; minimal file set present.

> **Handy commands**
>
> ```bash
> ./meltilint ./core ./providers/...
> go run ./cmd/validate-schemas
> go test ./conformance -run TestOffline
> ```

## License

MIT, see `LICENSE` at repo root.

---

## Quick Reference (Copy‑Paste)

```bash
# Stream quotes
go run ./cmd/market-stream --exchange okx --symbol ETH-USDT --channel ticker

# Private WS (orders/balances) typically requires API keys via env
export OKX_KEY=...; export OKX_SECRET=...; export OKX_PASSPHRASE=...

# Add a provider scaffold
go run ./cmd/barista -name bybit

# Validate protocol assets
go run ./cmd/validate-schemas

# Full test suite
make test   # or: go test ./... -race -count=1
```
