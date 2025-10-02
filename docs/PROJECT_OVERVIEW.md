# Meltica — Project Overview

## What it is

Meltica is a **provider-agnostic Go SDK** that normalizes REST and WebSocket market/trading flows across major crypto exchanges (e.g., Binance, Coinbase, Kraken, OKX). It exposes stable **core abstractions**, plus **adapters** that translate each exchange’s quirks into that contract. A **schema-driven protocol**, **conformance harness**, and **custom linter** keep all providers honest and interoperable. 

---

## Architecture at a glance

* **core/** — Canonical domain models (orders, trades, tickers, balances), provider interfaces, capability bitset helpers, symbol/topic utilities, and normalized WS event types. This is the contract everything else targets. 
* **providers/<exchange>/** — Concrete adapters per venue: REST & WS surfaces, request signing, error/status mappings, and tests. 
* **transport/** — Shared HTTP client with signing hooks, retries, and token-bucket rate limiting. 
* **protocol/** — JSON Schemas, sample vectors, and a protocol version constant that lock down the wire format. 
* **conformance/** — Harness that compiles schemas, validates vectors, and verifies that declared capabilities match reality. 
* **internal/meltilint/** — Static analysis enforcing protocol standards, provider compliance, frozen interfaces, doc presence, and “no-float” rules; ships a CLI. 
* **internal/test/** — Golden/vector tests, fixture capture helpers, and provider-specific checks (live or recorded). 
* **cmd/** — CLI tools:

  * `barista` (scaffolds a new provider),
  * `market-stream` (subscribe/print WS streams),
  * `validate-schemas` (schema/vector validator). 
* **docs/** — Governance, expectations, migration steps, how-tos, and validation rules. 
* **out/** — Built binaries produced by `make build` for distribution/testing. 

---

## Typical developer workflow

1. **Target the contract:** Implement or extend features against `core` interfaces/models. 
2. **Scaffold a provider:** Use `cmd/barista` to create `providers/<exchange>/` with stubs for REST/WS, signing, and status mappings. 
3. **Implement & test:** Add REST/WS logic; write unit and golden tests under `internal/test`; capture fixtures as needed. 
4. **Validate the protocol:** Update/confirm schemas & vectors in `protocol/`; run the `conformance` harness. 
5. **Lint for compliance:** Run `internal/meltilint` to enforce standards (capabilities, frozen interfaces, docs, no floats, etc.). 
6. **Ship binaries:** Build CLI tools with `make build`; artifacts land in `out/`. Use `cmd/market-stream` for manual integration checks. 

---

## Key modules & roles (quick reference)

* **core/**: canonical models & interfaces; capability helpers; symbols/topics; WS event payloads. 
* **errs/**: unified error envelope (`errs.E`) and standardized capability errors. 
* **transport/**: HTTP wrapper with retries, signing hooks, and rate limiting. 
* **protocol/**: JSON Schemas and golden vectors; protocol version. 
* **conformance/**: loads schemas/vectors and asserts provider capability declarations. 
* **internal/meltilint/**: CLI linter ensuring protocol/code/documentation standards. 
* **internal/test/**: goldens, capture harnesses, provider conformance checks. 
* **providers/**: Binance, Coinbase, Kraken, OKX adapters (REST/WS, signing, errors/status, tests). 
* **cmd/**: `barista`, `market-stream`, `validate-schemas` CLIs. 
* **docs/**: onboarding, expectations, migration, and validation rulebook. 

---

## CLI tools

* **barista** — Generator that scaffolds new provider packages from templates. 
* **market-stream** — Instantiates a provider, subscribes to WS topics, and prints normalized events (e.g., trades, tickers, depth). 
* **validate-schemas** — Invokes the conformance validator over schemas/vectors. 

*Example usage* (choose appropriate flags for your venue/topic), then run tests/lints as usual. 

---

## Testing & conformance

* **Unit + golden tests:** Validate parsing and normalization using recorded fixtures and expected JSON snapshots. 
* **Conformance harness:** Compiles schemas, validates vectors, and checks provider capability declarations against implementations. 
* **meltilint:** Enforces protocol invariants, capability/version declarations, doc completeness, and precision rules. Integrates cleanly with CI. 

---

## Top-level files

* **README.md** — intro, capability matrix, usage, build/test guidance.
* **LICENSE** — MIT terms.
* **Makefile** — build/test/lint targets; binary recipes.
* **go.mod / go.sum** — module/dependency metadata.
* **market-stream** — prebuilt CLI binary for quick runs. 

---

## Release & distribution

* Protocol changes must co-move with updated schemas/vectors and pass `conformance` + `meltilint`. 
* `make build` compiles CLIs; artifacts are placed under `out/`. Use these for distribution and manual testing. 

---

## Onboarding checklist

1. Read **docs/START-HERE.md** to grok expectations and workflow.
2. Review **docs/expectations/** and **docs/how-to/** for adapter standards and the migration path.
3. Build & run **cmd/market-stream** against a test venue; inspect normalized events.
4. Add or update a provider, then run: unit tests → conformance → `meltilint` → build. 

---

**In one line:** Develop against `core`, prove it with `protocol` + `conformance`, enforce it with `meltilint`, and exercise it with the CLIs—repeat per provider. 
