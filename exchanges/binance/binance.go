package binance

import (
	"context"
	"strings"
	"sync"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type Exchange struct {
	name string

	transports         *transportBundle
	symbols            *symbolService
	listenKeys         *listenKeyService
	depths             *depthSnapshotService
	orderBooks         *OrderBookService
	transportFactories transportFactories
	routerFactories    routerFactories
	cfg                config.Settings
	cfgMu              sync.Mutex
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
	return newExchangeWithFactories(settings, params.transports, params.routers)
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
	return newExchangeWithFactories(settings, params.transports, params.routers)
}

func newExchangeWithFactories(settings config.Settings, transports transportFactories, routers routerFactories) (*Exchange, error) {
	binCfg := resolveBinanceSettings(settings)
	restCfg := rest.Config{
		APIKey:         binCfg.Credentials.APIKey,
		Secret:         binCfg.Credentials.APISecret,
		SpotBaseURL:    binCfg.REST[config.BinanceRESTSurfaceSpot],
		LinearBaseURL:  binCfg.REST[config.BinanceRESTSurfaceLinear],
		InverseBaseURL: binCfg.REST[config.BinanceRESTSurfaceInverse],
		Timeout:        binCfg.HTTPTimeout,
	}
	wsCfg := ws.Config{
		PublicURL:        binCfg.Websocket.PublicURL,
		PrivateURL:       binCfg.Websocket.PrivateURL,
		HandshakeTimeout: binCfg.HandshakeTimeout,
	}
	bundle := buildTransportBundle(transports, routers, restCfg, wsCfg)
	symbols := newSymbolService(bundle.Router())
	listenKeys := newListenKeyService(bundle.Router())
	depths := newDepthSnapshotService(bundle.Router(), symbols)
	wsDeps := newWSDependencies(symbols, listenKeys, depths)
	bundle.SetWS(routers.newWSRouter(bundle.WSInfra(), wsDeps))

	orderBooks := newOrderBookService(bundle.WS(), depths, symbols)

	x := &Exchange{
		name:               "binance",
		transports:         bundle,
		symbols:            symbols,
		listenKeys:         listenKeys,
		depths:             depths,
		orderBooks:         orderBooks,
		transportFactories: transports,
		routerFactories:    routers,
		cfg:                config.Apply(settings),
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
	wsCfg := ws.Config{
		PublicURL:        binCfg.Websocket.PublicURL,
		PrivateURL:       binCfg.Websocket.PrivateURL,
		HandshakeTimeout: binCfg.HandshakeTimeout,
	}

	newBundle := buildTransportBundle(x.transportFactories, x.routerFactories, restCfg, wsCfg)
	newSymbols := newSymbolService(newBundle.Router())
	newListenKeys := newListenKeyService(newBundle.Router())
	newDepths := newDepthSnapshotService(newBundle.Router(), newSymbols)
	wsDeps := newWSDependencies(newSymbols, newListenKeys, newDepths)
	newBundle.SetWS(x.routerFactories.newWSRouter(newBundle.WSInfra(), wsDeps))

	x.cfgMu.Lock()
	oldBundle := x.transports
	x.transports = newBundle
	x.symbols = newSymbols
	x.listenKeys = newListenKeys
	x.depths = newDepths
	x.cfg = newCfg
	x.cfgMu.Unlock()

	if oldBundle != nil {
		_ = oldBundle.Close()
	}

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

func (x *Exchange) WS() core.WS { return newWSService(x.wsRouter()) }

func (x *Exchange) Close() error {
	if x.transports != nil {
		return x.transports.Close()
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
	if x.listenKeys == nil {
		return "", internal.Exchange("listen key service unavailable")
	}
	return x.listenKeys.Create(ctx)
}

func (x *Exchange) KeepAliveListenKey(ctx context.Context, key string) error {
	if x.listenKeys == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return x.listenKeys.KeepAlive(ctx, key)
}

func (x *Exchange) CloseListenKey(ctx context.Context, key string) error {
	if x.listenKeys == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return x.listenKeys.Close(ctx, key)
}

func (x *Exchange) DepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if x.depths == nil {
		return corestreams.BookEvent{}, 0, internal.Exchange("depth snapshot service unavailable")
	}
	return x.depths.Snapshot(ctx, symbol, limit)
}

func (x *Exchange) timeInForceCode(t core.TimeInForce) string {
	return internal.MapTimeInForce(t)
}

func (x *Exchange) nativeSymbolForMarkets(ctx context.Context, canonical string, markets ...core.Market) (string, error) {
	if x.symbols == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	return x.symbols.nativeForMarkets(ctx, canonical, markets...)
}

func (x *Exchange) canonicalSymbolForMarkets(ctx context.Context, binanceSymbol string, markets ...core.Market) (string, error) {
	if x.symbols == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	return x.symbols.canonicalForMarkets(ctx, binanceSymbol, markets...)
}

func (x *Exchange) ensureMarketSymbols(ctx context.Context, market core.Market) error {
	if x.symbols == nil {
		return internal.Exchange("symbol service unavailable")
	}
	return x.symbols.ensureMarket(ctx, market)
}

func (x *Exchange) instrumentsForMarket(ctx context.Context, market core.Market) ([]core.Instrument, error) {
	if x.symbols == nil {
		return nil, internal.Exchange("symbol service unavailable")
	}
	return x.symbols.instruments(ctx, market)
}

func (x *Exchange) instrument(ctx context.Context, market core.Market, symbol string) (core.Instrument, bool, error) {
	if x.symbols == nil {
		return core.Instrument{}, false, internal.Exchange("symbol service unavailable")
	}
	return x.symbols.instrument(ctx, market, symbol)
}

func (x *Exchange) restRouter() routingrest.RESTDispatcher {
	if x.transports == nil {
		return nil
	}
	return x.transports.Router()
}

func (x *Exchange) wsRouter() wsRouter {
	if x.transports == nil {
		return nil
	}
	return x.transports.WS()
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
