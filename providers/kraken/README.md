# Kraken Provider Notes

## REST Rate Limits

- Kraken private REST endpoints allow approximately 15 authenticated calls every 3 seconds.
- The shared `transport.Client` applies exponential backoff and honors `Retry-After`. For sustained ingestion, use an external token bucket (e.g., 5 requests/second) to stay below the hard ceiling.
- When the API responds with HTTP 429, the client sleeps for the advertised duration before retrying.

## Pagination

- `Spot.Trades` issues paginated `TradesHistory` calls using the `ofs` cursor. When a `since` value (unix timestamp in seconds) is provided, it is mapped to Kraken's `start` parameter.
- The method keeps requesting pages until it reaches the exchange-provided `count` or a page returns no new trades.
- Callers should persist the latest trade timestamp and pass it as the next `since` to avoid reprocessing history.

## WebSocket Tokens

- Private WebSocket feeds require a short-lived token fetched from `/0/private/GetWebSocketsToken`. The implementation caches the token until its expiry and refreshes automatically.

