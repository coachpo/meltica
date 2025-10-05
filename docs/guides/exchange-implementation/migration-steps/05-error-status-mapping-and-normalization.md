# 05 — Error/Status Mapping, Symbols, Decimals, Enums (Team E)

**Goal:** Produce canonical errors and enums; enforce symbol and numeric policies across all code paths.

---
## What needs to be done
1) Implement error normalization returning `*errs.E` with canonical codes and raw exchange details.
2) Implement exhaustive mappings for `OrderStatus`, `OrderSide`, `OrderType`, `TimeInForce`.
3) Enforce canonical symbols and `*big.Rat` numerics everywhere.

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
   - Before constructing models/events, call symbol conversion functions.
   - Replace any floats with `*big.Rat`; ensure proper numeric handling.

---
## Current Error System

The error system uses the `errs` package with the following structure:

```go
type Code string

const (
    CodeRateLimited Code = "rate_limited"
    CodeAuth        Code = "auth"
    CodeInvalid     Code = "invalid_request"
    CodeExchange    Code = "exchange_error"
    CodeNetwork     Code = "network"
)

type E struct {
    Exchange string
    Code     Code
    HTTP     int
    RawCode  string
    RawMsg   string
    Message  string
    cause error
}
```

## How to validate that it is complete
1) **Build & unit tests:**
   ```bash
   go build ./... && go test ./exchanges/<name> -count=1
   ```
2) **Error handling validation:**
   - Verify all errors use `*errs.E` structure
   - Verify canonical error codes are used consistently
   - Verify raw exchange details are preserved
3) **Symbols & decimals validation:**
   - Verify all symbols use `BASE-QUOTE` format
   - Verify all numeric fields use `*big.Rat`
   - Verify proper numeric handling in JSON marshaling
