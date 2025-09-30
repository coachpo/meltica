# 05 — Error/Status Mapping, Symbols, Decimals, Enums (Team E)

**Goal:** Produce canonical errors and enums; enforce symbol and numeric policies across all code paths.

---
## What needs to be done
1) Implement error normalization returning `*errs.E` with canonical codes and raw exchange details.
2) Implement exhaustive mappings for `OrderStatus`, `OrderSide`, `OrderType`, `TimeInForce`.
3) Enforce canonical symbols and `*big.Rat` numerics everywhere; marshal with `core.FormatDecimal`.

---
## How to do it (follow exactly)
1) **errors.go**
   - Provide helpers like `wrapHTTP(provider, httpStatus, rawCode, rawMsg) *errs.E`.
   - Map buckets: `auth`, `rate_limited`, `invalid_request`, `exchange_error`, `network` to canonical `errs.Code` constants.
   - Every public function returning `error` must return or wrap `*errs.E`.

2) **status.go (and enum mapping files)**
   - Implement `mapOrderStatus`, `mapOrderType`, `mapOrderSide`, `mapTIF` with a `switch` that covers **all** provider values.
   - In `default`, return `*errs.E` (do **not** default to success).

3) **Symbols & numerics**
   - Before constructing models/events, call `core.ToCanonicalSymbol` (or equivalent).
   - Replace any floats with `*big.Rat`; ensure (Un)MarshalJSON calls `core.FormatDecimal`.

---
## How to validate that it is complete
1) **Static analysis:**
   ```bash
   ./protolint ./providers/<name>
   ```
   Confirms:
   - No floats in exported fields or public APIs.
   - Enum mapping exhaustive (no silent defaults).
   - WS decoders return typed events.
   - Errors are canonical `*errs.E`.

2) **Golden tests for errors/status:**
   ```bash
   go test ./providers/<name> -run TestErrorAndStatusGolden -count=1
   ```
3) **Schema/vector validation (where applicable):**
   ```bash
   go run ./cmd/validate-schemas
   ```
