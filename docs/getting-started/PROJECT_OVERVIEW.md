# Meltica — Project Overview

## What it is

Meltica is a **provider-agnostic Go SDK** that normalizes REST and WebSocket market/trading flows across major crypto exchanges (e.g., Binance, Coinbase, Kraken, OKX). It exposes stable **core abstractions**, plus **adapters** that translate each exchange’s quirks into that contract. A **schema-driven protocol**, **conformance harness**, and **custom linter** keep all providers honest and interoperable. 

---

## Docs quick links

- Start here: [START-HERE.md](./START-HERE.md)
- Context map: [CONTEXT.md](./CONTEXT.md)
- Expectations & standards: [expectations/](./expectations/)
- How-to guides: [how-to/](./how-to/)
- Protocol validation rules: [validation/protocol-validation-rules.md](./validation/protocol-validation-rules.md)
- Venue support report: [Venue_Support_Report.md](./Venue_Support_Report.md)
- Full docs index: [README.md](./README.md)

## Architecture at a glance

* **core/** — Canonical domain models (orders, trades, tickers, balances), provider interfaces, capability bitset helpers, symbol/topic utilities, and normalized WS event types. This is the contract everything else targets. 
* **exchanges/<exchange>/** — Concrete adapters per venue: REST & WS surfaces, request signing, error/status mappings, and tests. 
* **transport/** — Shared HTTP client with signing hooks, retries, and token-bucket rate limiting. 
* **config/** — Configuration management for exchange adapters.
* **errs/** — Unified error envelope and standardized capability errors.
* **cmd/** — CLI tools:

  * `main` (market data streaming and order book monitoring)
* **docs/** — Governance, expectations, migration steps, how-tos, and validation rules. 

---

## Typical developer workflow

1. **Target the contract:** Implement or extend features against `core` interfaces/models. 
2. **Implement an exchange adapter:** Create `exchanges/<exchange>/` with REST/WS, signing, and status mappings. 
3. **Implement & test:** Add REST/WS logic; write unit tests; capture fixtures as needed. 
4. **Validate:** Run unit tests and integration tests.
5. **Use CLI:** Build and run `cmd/main` for manual integration checks. 

---

## Key modules & roles (quick reference)

* **core/**: canonical models & interfaces; capability helpers; symbols/topics; WS event payloads. 
* **errs/**: unified error envelope and standardized capability errors. 
* **transport/**: HTTP wrapper with retries, signing hooks, and rate limiting. 
* **config/**: configuration management for exchange adapters.
* **exchanges/**: Binance adapter (REST/WS, signing, errors/status, tests). 
* **cmd/**: `main` CLI for market data streaming.
* **docs/**: onboarding, expectations, migration, and validation rulebook. 

---

## CLI tools

* **main** — Market data streaming tool that instantiates an exchange, subscribes to WS topics, and prints normalized events (e.g., trades, tickers, depth). 

*Example usage* (choose appropriate flags for your venue/topic), then run tests as usual. 

---

## Testing & conformance

* **Unit tests:** Validate parsing and normalization using recorded fixtures and expected JSON snapshots. 
* **Integration tests:** End-to-end testing with exchange adapters.

---

## Top-level files

* **README.md** — intro, capability matrix, usage, build/test guidance.
* **LICENSE** — MIT terms.
* **Makefile** — build/test/lint targets; binary recipes.
* **go.mod / go.sum** — module/dependency metadata.

---

## Release & distribution

* `make build` compiles the CLI binary.

---

## Onboarding checklist

1. Open **docs/README.md** for the full documentation index.
2. Read **docs/START-HERE.md** to grok expectations and workflow.
3. Review **docs/expectations/** and **docs/how-to/** for adapter standards and the migration path.
4. Build & run **cmd/main** against a test venue; inspect normalized events.
5. Add or update an exchange adapter, then run: unit tests → build. 

---

**In one line:** Develop against `core`, implement exchange adapters, and exercise it with the CLI—repeat per exchange. 
