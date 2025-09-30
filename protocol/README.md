# Protocol Specification

This directory contains machine- and human-readable assets that describe the universal provider protocol. Files here are versioned alongside the core abstractions in `core/` and must be updated whenever the protocol surface changes.

- `schemas/`: JSON Schema definitions for canonical models and websocket events
- `vectors/`: Golden JSON fixtures that demonstrate valid payloads per schema

## Capturing Live Fixtures

Before refreshing vectors, capture current REST payloads by running:

```
go test ./internal/test -run CoinbaseCaptureFixtures
go test ./internal/test -run KrakenCaptureFixtures
```

Provide real API credentials and output file locations via the environment variables documented in `internal/test/coinbase_capture_test.go` and `internal/test/kraken_capture_test.go`. Replace synthetic fixtures with captured payloads before promoting schema updates.

## Validating Conformance Assets

Run `go test ./conformance` to lint schemas, validate golden vectors, and ensure providers declare the required capabilities.
