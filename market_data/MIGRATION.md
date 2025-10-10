# Migration Guide: Parser Pipeline to Router Processors

## Overview

The legacy `market_data/framework/parser` package has been deprecated in favor of
the typed processor and routing framework introduced in branch
`003-we-ve-built`. The parser pipeline remains available for backward
compatibility, but new development should rely on processors registered with the
framework router.

## Migration Checklist

1. **Implement a Processor**
   - Create a struct in `market_data/processors` that satisfies the
     `Processor` interface.
   - Decode raw payload bytes inside `Process(ctx, raw)` and return a typed
     model. Reuse existing helpers in the processors package for decimal
     parsing and validation.

2. **Describe Detection Rules**
   - Add a `MessageTypeDescriptor` to the routing table (see
     `market_data/framework/router/message_types.go`).
   - Use field-based detection rules to match the new message type.

3. **Register the Processor**
   - Call `routing.InitializeRouter()` or the exchange-specific registration
     helper to wire in the new processor.
   - Set the default processor for unrecognized payloads if needed.

4. **Retire Parser Usage**
   - Remove `parser.NewJSONPipeline` and `parser.NewValidationStage` from your
     connection code. Instead, forward raw frames to the router dispatcher, which
     will invoke the appropriate processor and publish metrics automatically.
   - Delete custom validator functions; perform validation inside processor
     `Process` implementations and return typed errors via `errs.New`.

5. **Update Tests**
   - Replace parser pipeline tests with processor unit tests and router
     integration tests (`go test ./market_data/processors` and
     `go test ./market_data/framework/router`).

## Validation

After completing the migration, run the following commands to ensure the new
path is healthy:

```bash
go test ./market_data/processors/...
go test ./market_data/framework/router/...
go test ./exchanges/binance/... # if integrating with the Binance adapter
```

## Support

For questions about migrating complex parser pipelines, consult the architecture
plan in `specs/003-we-ve-built/plan.md` or reach out to the routing framework
maintainers listed in `docs/README.md`.
