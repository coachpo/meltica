# Audit Results – Backward Compatibility Removal

## Code Imports

- `market_data/framework/connection/session_runtime.go`
  - Imports `github.com/coachpo/meltica/market_data/framework/parser`

## Test Imports

- `internal/benchmarks/market_data/framework/engine_bench_test.go`
- `tests/market_data/framework/unit/validation_stage_test.go`

## Package Dependencies

- `github.com/coachpo/meltica/market_data/framework/connection`
  - Depends on `market_data/framework/parser`
- `github.com/coachpo/meltica/market_data/framework/parser`
  - Internal package slated for deletion

## Documentation References

- `docs/guides/router_troubleshooting.md`
  - Mentions legacy helper `parser.WithInvalidThreshold`

## Notes

- No other Go source files currently import the deprecated parser package.
- Removal work must delete the parser package, update connection runtime, and migrate or remove associated tests and documentation references.
