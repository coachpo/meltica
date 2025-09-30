# Coinbase Provider Notes

## REST Rate Limits

- Coinbase Advanced Trade enforces burst limits of ~10 authenticated calls per second and ~1000 calls per hour.
- The `transport.Client` is configured with exponential backoff and respects `Retry-After` headers. When the exchange answers with HTTP 429, requests are retried automatically.
- For long-running collectors, prefer staging requests through an external rate limiter and monitor the `Cb-After` cursors to avoid replaying pages.

## Pagination

- `Spot.Trades` handles cursor-based pagination automatically. It requests up to five pages (500 fills) per invocation and follows the `Cb-After` header returned by Coinbase.
- The `since` argument should be set to the last observed `trade_id`. The method will send it as the `after` cursor on the first call.
- When more history is required, continue calling `Trades` with the most recent `trade_id` until no new rows are returned.

## WebSocket

- Authenticated channels require the REST passphrase and HMAC-SHA256 signature. The implementation reuses the REST signer to produce the payload accepted by Coinbase.

