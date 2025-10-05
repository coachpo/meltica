# Onboarding A New Exchange (Super Simple Guide)

Follow these tiny steps to add a brand-new exchange to Meltica. Each step has a clear goal so you always know when you are done. Take it slow, read every line, and you will be fine—even if you are five!

## Step 1 – Pick a Name
**Goal:** Decide how everyone will call the exchange inside Meltica.
1. Choose a short lowercase name, like `okx`.
2. Add a constant in `config/config.go` using the new `Exchange` type, for example `ExchangeOKX Exchange = "okx"`.

## Step 2 – Give It Default Settings
**Goal:** Make Meltica know the basic URLs and timeouts.
1. In `config.Default()`, add a new entry in the `Exchanges` map for your exchange.
2. Fill in:
   - REST base URLs in `REST` (use the keys you need, such as `spot`, `futures`, etc.).
   - WebSocket URLs in `Websocket`.
   - Timeouts (HTTP + handshake) with safe defaults.

## Step 3 – Support Environment Overrides
**Goal:** Let users change settings without editing code.
1. In `config.FromEnv()`, read env vars such as `OKX_SPOT_BASE_URL` and put them into the new settings entry.
2. Only save trimmed, non-empty values.
3. Leave binance code as-is; do not break it.

## Step 4 – Create Option Helpers (Nice to Have)
**Goal:** Allow Go code to tweak settings easily.
1. Add helper functions such as `WithOKXRESTEndpoints` that call `WithExchangeRESTEndpoint` with your exchange name.
2. Keep names clear so other engineers understand what they do.

## Step 5 – Make a New Package Folder
**Goal:** Start the real adapter code.
1. Copy the folder structure from `exchanges/binance` into a new folder like `exchanges/okx`.
2. Keep only the files you plan to implement; stubs are fine while you wire things up.

## Step 6 – Configure The Exchange
**Goal:** Add exchange configuration to the config system.
1. Add exchange constant in `config/config.go`:
   ```go
   const ExchangeOKX Exchange = "okx"
   ```
2. Add default settings to `config.Default()` function.
3. Add environment variable support in `config.FromEnv()`.

## Step 7 – Wire Up Construction In `exchanges/okx`
**Goal:** Build the concrete `Exchange` type.
1. Follow `exchanges/binance/binance.go` as the template for your `Exchange` struct.
2. Use `config.DefaultExchangeSettings` plus overrides from `cfg.Exchange(...)` to obtain runtime settings.
3. Create REST/WS infrastructure with helpers from `exchanges/shared/infra` and routers from `exchanges/shared/routing`.

Example construction pattern:
```go
func NewWithSettings(settings config.Settings) (*Exchange, error) {
    okxCfg := resolveOKXSettings(settings)
    
    restClient := rest.NewClient(rest.Config{
        APIKey:      okxCfg.Credentials.APIKey,
        Secret:      okxCfg.Credentials.APISecret,
        SpotBaseURL: okxCfg.REST["spot"],
        Timeout:     okxCfg.HTTPTimeout,
    })
    
    restRouter := routing.NewRESTRouter(restClient)
    wsInfra := ws.NewClient(ws.Config{
        PublicURL:        okxCfg.Websocket.PublicURL,
        PrivateURL:       okxCfg.Websocket.PrivateURL,
        HandshakeTimeout: okxCfg.HandshakeTimeout,
    })

    x := &Exchange{
        name:       "okx",
        restClient: restClient,
        restRouter: restRouter,
        wsInfra:    wsInfra,
        instCache:  make(map[core.Market]map[string]core.Instrument),
        symbols:    newSymbolRegistry(),
        cfg:        settings,
    }
    x.wsRouter = routing.NewWSRouter(wsInfra, x)
    return x, nil
}
```

## Step 8 – Implement REST And WebSocket Logic
**Goal:** Actually talk to the exchange.
1. Map REST endpoints (tickers, balances, orders) using the shared REST router and `core/transport` contracts.
2. Map WebSocket streams using the shared WS router patterns and implement `core/transport.StreamClient` and `core/streams` routing helpers.
3. Normalize data into Meltica core types (`core/streams` structs, topics from `core/topics`).
4. Reuse numeric helpers from `exchanges/shared/infra/numeric`.

## Step 9 – Update Docs And Tests
**Goal:** Tell others how to use the new exchange and prove it works.
1. Add the exchange to `docs/getting-started/CONTEXT.md` and any other lists.
2. Write unit tests similar to Binance ones (REST router, WS router, parsing).
3. Run `go test ./...` and the Makefile targets (`make test`, `make build`, `make build-linux-arm64`).

## Step 10 – Smoke Test (Optional But Helpful)
**Goal:** Check everything really works end-to-end.
1. Create a simple command in `cmd/` or reuse an existing validation tool.
2. Use sandbox/testnet credentials when possible.
3. Check logs for normalized events and orders.

## Step 11 – Share Your Success
**Goal:** Let the team know the new exchange is ready.
1. Push your branch, open a pull request, and describe the steps you followed.
2. Mention any missing pieces or future TODOs.
3. Ask for reviews from folks who own similar adapters.

That's it! You now have a friendly roadmap to bring a new exchange into Meltica. Go slow, check each goal, and celebrate when you finish.
