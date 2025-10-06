package binance

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type Exchange struct {
	name string

	restClient coretransport.RESTClient
	restRouter routingrest.RESTDispatcher

	wsInfra  coretransport.StreamClient
	wsRouter wsRouter

	factories transportFactories

	instCache map[core.Market]map[string]core.Instrument
	symbols   *symbolRegistry
	symbolsMu sync.RWMutex
	cfg       config.Settings
	cfgMu     sync.Mutex
}

var capabilities = core.Capabilities(
	core.CapabilitySpotPublicREST,
	core.CapabilitySpotTradingREST,
	core.CapabilityLinearPublicREST,
	core.CapabilityLinearTradingREST,
	core.CapabilityInversePublicREST,
	core.CapabilityInverseTradingREST,
	core.CapabilityWebsocketPublic,
	core.CapabilityWebsocketPrivate,
)

type wsRouter interface {
	SubscribePublic(ctx context.Context, topics ...string) (bnrouting.Subscription, error)
	SubscribePrivate(ctx context.Context) (bnrouting.Subscription, error)
	Close() error
	WSNativeSymbol(canonical string) (string, error)
	WSCanonicalSymbol(native string) (string, error)
	OrderBookSnapshot(symbol string) (corestreams.BookEvent, bool)
	InitializeOrderBook(ctx context.Context, symbol string) error
}

func New(apiKey, secret string, opts ...Option) (*Exchange, error) {
	params := defaultConstructionParams()
	params.cfgOpts = append(params.cfgOpts, config.WithBinanceAPI(apiKey, secret))
	for _, opt := range opts {
		if opt != nil {
			opt(&params)
		}
	}
	settings := config.FromEnv()
	if len(params.cfgOpts) > 0 {
		settings = config.Apply(settings, params.cfgOpts...)
	}
	return newExchangeWithFactories(settings, params.factories)
}

func NewWithSettings(settings config.Settings, opts ...Option) (*Exchange, error) {
	params := defaultConstructionParams()
	for _, opt := range opts {
		if opt != nil {
			opt(&params)
		}
	}
	if len(params.cfgOpts) > 0 {
		settings = config.Apply(settings, params.cfgOpts...)
	}
	return newExchangeWithFactories(settings, params.factories)
}

func newExchangeWithFactories(settings config.Settings, factories transportFactories) (*Exchange, error) {
	binCfg := resolveBinanceSettings(settings)
	restCfg := rest.Config{
		APIKey:         binCfg.Credentials.APIKey,
		Secret:         binCfg.Credentials.APISecret,
		SpotBaseURL:    binCfg.REST[config.BinanceRESTSurfaceSpot],
		LinearBaseURL:  binCfg.REST[config.BinanceRESTSurfaceLinear],
		InverseBaseURL: binCfg.REST[config.BinanceRESTSurfaceInverse],
		Timeout:        binCfg.HTTPTimeout,
	}
	restClient := factories.newRESTClient(restCfg)
	if restClient == nil {
		restClient = rest.NewClient(restCfg)
	}
	restRouter := factories.newRESTRouter(restClient)
	if restRouter == nil {
		restRouter = bnrouting.NewRESTRouter(restClient)
	}
	wsCfg := ws.Config{
		PublicURL:        binCfg.Websocket.PublicURL,
		PrivateURL:       binCfg.Websocket.PrivateURL,
		HandshakeTimeout: binCfg.HandshakeTimeout,
	}
	wsInfra := factories.newWSClient(wsCfg)
	if wsInfra == nil {
		wsInfra = ws.NewClient(wsCfg)
	}
	x := &Exchange{
		name:       "binance",
		restClient: restClient,
		restRouter: restRouter,
		wsInfra:    wsInfra,
		factories:  factories,
		instCache:  make(map[core.Market]map[string]core.Instrument),
		symbols:    newSymbolRegistry(),
		cfg:        config.Apply(settings),
	}
	x.wsRouter = factories.newWSRouter(wsInfra, x)
	if x.wsRouter == nil {
		x.wsRouter = bnrouting.NewWSRouter(wsInfra, x)
	}
	return x, nil
}

// Config returns a snapshot of the active configuration.
func (x *Exchange) Config() config.Settings {
	x.cfgMu.Lock()
	defer x.cfgMu.Unlock()
	return config.Apply(x.cfg)
}

// UpdateConfig applies configuration overrides at runtime, rebuilding clients and clearing caches.
func (x *Exchange) UpdateConfig(opts ...config.Option) error {
	base := x.Config()
	newCfg := config.Apply(base, opts...)
	binCfg := resolveBinanceSettings(newCfg)
	restCfg := rest.Config{
		APIKey:         binCfg.Credentials.APIKey,
		Secret:         binCfg.Credentials.APISecret,
		SpotBaseURL:    binCfg.REST[config.BinanceRESTSurfaceSpot],
		LinearBaseURL:  binCfg.REST[config.BinanceRESTSurfaceLinear],
		InverseBaseURL: binCfg.REST[config.BinanceRESTSurfaceInverse],
		Timeout:        binCfg.HTTPTimeout,
	}
	restClient := x.factories.newRESTClient(restCfg)
	if restClient == nil {
		restClient = rest.NewClient(restCfg)
	}
	restRouter := x.factories.newRESTRouter(restClient)
	if restRouter == nil {
		restRouter = bnrouting.NewRESTRouter(restClient)
	}
	wsCfg := ws.Config{
		PublicURL:        binCfg.Websocket.PublicURL,
		PrivateURL:       binCfg.Websocket.PrivateURL,
		HandshakeTimeout: binCfg.HandshakeTimeout,
	}
	wsInfra := x.factories.newWSClient(wsCfg)
	if wsInfra == nil {
		wsInfra = ws.NewClient(wsCfg)
	}
	wsRouter := x.factories.newWSRouter(wsInfra, x)
	if wsRouter == nil {
		wsRouter = bnrouting.NewWSRouter(wsInfra, x)
	}

	x.cfgMu.Lock()
	if x.wsRouter != nil {
		_ = x.wsRouter.Close()
	}
	x.restClient = restClient
	x.restRouter = restRouter
	x.wsInfra = wsInfra
	x.wsRouter = wsRouter
	x.cfg = newCfg
	x.cfgMu.Unlock()

	x.symbolsMu.Lock()
	x.symbols = newSymbolRegistry()
	x.instCache = make(map[core.Market]map[string]core.Instrument)
	x.symbolsMu.Unlock()

	return nil
}

func (x *Exchange) Name() string { return x.name }

func (x *Exchange) Capabilities() core.ExchangeCapabilities { return capabilities }

func (x *Exchange) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (x *Exchange) Spot(ctx context.Context) core.SpotAPI { return spotAPI{x} }
func (x *Exchange) LinearFutures(ctx context.Context) core.FuturesAPI {
	return newLinearFuturesAPI(x)
}
func (x *Exchange) InverseFutures(ctx context.Context) core.FuturesAPI {
	return newInverseFuturesAPI(x)
}

func (x *Exchange) WS() core.WS { return newWSService(x.wsRouter) }

func (x *Exchange) Close() error {
	if x.wsRouter != nil {
		_ = x.wsRouter.Close()
	}
	return nil
}

func resolveBinanceSettings(cfg config.Settings) config.ExchangeSettings {
	defaults, _ := config.DefaultExchangeSettings(config.ExchangeBinance)
	override, ok := cfg.Exchange(config.ExchangeBinance)
	if !ok {
		return defaults
	}

	merged := defaults
	for k, v := range override.REST {
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			merged.REST[k] = trimmed
		}
	}
	if pub := strings.TrimSpace(override.Websocket.PublicURL); pub != "" {
		merged.Websocket.PublicURL = pub
	}
	if priv := strings.TrimSpace(override.Websocket.PrivateURL); priv != "" {
		merged.Websocket.PrivateURL = priv
	}
	if override.HTTPTimeout > 0 {
		merged.HTTPTimeout = override.HTTPTimeout
	}
	if override.HandshakeTimeout > 0 {
		merged.HandshakeTimeout = override.HandshakeTimeout
	}
	if key := strings.TrimSpace(override.Credentials.APIKey); key != "" {
		merged.Credentials.APIKey = key
	}
	if secret := strings.TrimSpace(override.Credentials.APISecret); secret != "" {
		merged.Credentials.APISecret = secret
	}
	return merged
}

// CanonicalSymbol converts Binance native symbols to canonical form with caching support.
func (x *Exchange) CanonicalSymbol(binanceSymbol string) (string, error) {
	return x.canonicalSymbolForMarkets(context.Background(), binanceSymbol)
}

func (x *Exchange) NativeSymbol(canonical string) (string, error) {
	return x.nativeSymbolForMarkets(context.Background(), canonical)
}

func (x *Exchange) CreateListenKey(ctx context.Context) (string, error) {
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodPost, Path: "/api/v3/userDataStream"}
	if err := x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (x *Exchange) KeepAliveListenKey(ctx context.Context, key string) error {
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodPut, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return x.restRouter.Dispatch(ctx, msg, nil)
}

func (x *Exchange) CloseListenKey(ctx context.Context, key string) error {
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodDelete, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return x.restRouter.Dispatch(ctx, msg, nil)
}

func (x *Exchange) DepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if err := x.ensureMarketSymbols(ctx, core.MarketSpot); err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	native, ok := x.symbols.native(core.MarketSpot, symbol)
	if !ok {
		if fallback, found := x.symbols.nativeAny(symbol); found {
			native = fallback
		} else {
			return corestreams.BookEvent{}, 0, internal.Invalid("depth snapshot: unsupported symbol %s", symbol)
		}
	}
	params := map[string]string{"symbol": native, "limit": fmt.Sprintf("%d", limit)}
	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/depth", Query: params}
	if err := x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	bids := parseDepthLevels(resp.Bids)
	asks := parseDepthLevels(resp.Asks)
	event := corestreams.BookEvent{Symbol: symbol, Bids: bids, Asks: asks, Time: time.Now()}
	return event, resp.LastUpdateID, nil
}

func (x *Exchange) timeInForceCode(t core.TimeInForce) string {
	return internal.MapTimeInForce(t)
}

func (x *Exchange) nativeSymbolForMarkets(ctx context.Context, canonical string, markets ...core.Market) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(canonical))
	if trimmed == "" {
		return "", internal.Invalid("unsupported symbol %s", canonical)
	}
	targetMarkets := marketsOrAll(markets...)
	for _, market := range targetMarkets {
		if native, ok := x.symbols.native(market, trimmed); ok {
			return native, nil
		}
	}
	for _, market := range targetMarkets {
		if err := x.ensureMarketSymbols(ctx, market); err != nil {
			return "", err
		}
		if native, ok := x.symbols.native(market, trimmed); ok {
			return native, nil
		}
	}
	if len(markets) == 0 {
		if native, ok := x.symbols.nativeAny(trimmed); ok {
			return native, nil
		}
	}
	return "", internal.Invalid("unsupported symbol %s", trimmed)
}

func (x *Exchange) canonicalSymbolForMarkets(ctx context.Context, binanceSymbol string, markets ...core.Market) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if trimmed == "" {
		return "", internal.Invalid("unsupported symbol %s", binanceSymbol)
	}
	targetMarkets := marketsOrAll(markets...)
	belongsToTarget := func(canonical string) bool {
		if len(markets) == 0 {
			return true
		}
		for _, market := range targetMarkets {
			if _, ok := x.symbols.native(market, canonical); ok {
				return true
			}
		}
		return false
	}
	if canonical, ok := x.symbols.canonical(trimmed); ok && belongsToTarget(canonical) {
		return canonical, nil
	}
	for _, market := range targetMarkets {
		if err := x.ensureMarketSymbols(ctx, market); err != nil {
			return "", err
		}
		if canonical, ok := x.symbols.canonical(trimmed); ok && belongsToTarget(canonical) {
			return canonical, nil
		}
	}
	return "", internal.Exchange("unsupported symbol %s", trimmed)
}

func marketsOrAll(markets ...core.Market) []core.Market {
	if len(markets) == 0 {
		return []core.Market{core.MarketSpot, core.MarketLinearFutures, core.MarketInverseFutures}
	}
	seen := make(map[core.Market]struct{}, len(markets))
	out := make([]core.Market, 0, len(markets))
	for _, m := range markets {
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	if len(out) == 0 {
		return []core.Market{core.MarketSpot, core.MarketLinearFutures, core.MarketInverseFutures}
	}
	return out
}
