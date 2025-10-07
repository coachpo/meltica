package binance

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/bootstrap"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

const exchangeName core.ExchangeName = "binance"

type Exchange struct {
	name string

	transportBundle    *bootstrap.TransportBundle
	symbolSvc          *symbolService
	listenKeySvc       *listenKeyService
	orderBookSvc       *OrderBookService
	transportFactories bootstrap.TransportFactories
	routerFactories    bootstrap.RouterFactories
	cfg                config.Settings
	cfgMutex           sync.Mutex

	// Symbol refresh controls
	symbolRefreshInterval time.Duration
	symbolRefreshCancel   context.CancelFunc
	symbolRefreshDone     chan struct{}
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
	params.ConfigOpts = append(params.ConfigOpts, config.WithBinanceAPI(apiKey, secret))
	bootstrap.ApplyOptions(params, opts...)

	settings := config.FromEnv()
	if len(params.ConfigOpts) > 0 {
		settings = config.Apply(settings, params.ConfigOpts...)
	}
	return newExchangeWithFactories(settings, params.Transports, params.Routers)
}

func newExchangeWithFactories(settings config.Settings, transports bootstrap.TransportFactories, routers bootstrap.RouterFactories) (*Exchange, error) {
	binCfg := resolveBinanceSettings(settings)
	transportCfg := bootstrap.TransportConfig{
		APIKey:                binCfg.Credentials.APIKey,
		Secret:                binCfg.Credentials.APISecret,
		SpotBaseURL:           binCfg.REST[config.BinanceRESTSurfaceSpot],
		LinearBaseURL:         binCfg.REST[config.BinanceRESTSurfaceLinear],
		InverseBaseURL:        binCfg.REST[config.BinanceRESTSurfaceInverse],
		HTTPTimeout:           binCfg.HTTPTimeout,
		PublicURL:             binCfg.Websocket.PublicURL,
		PrivateURL:            binCfg.Websocket.PrivateURL,
		HandshakeTimeout:      binCfg.HandshakeTimeout,
		SymbolRefreshInterval: binCfg.SymbolRefreshInterval,
	}
	bundle := bootstrap.BuildTransportBundle(transports, routers, transportCfg)
	restRouter := bundle.Router().(routingrest.RESTDispatcher)
	symbolSvc := newSymbolService(restRouter)
	listenKeySvc := newListenKeyService(restRouter)
	wsDeps := newWSDependencies(exchangeName, symbolSvc, listenKeySvc, nil)
	bundle.SetWS(routers.NewWSRouter(bundle.WSInfra(), wsDeps))

	orderBookSvc := newOrderBookService(bundle.WS().(wsRouter), restRouter, symbolSvc)

	x := &Exchange{
		name:                  string(exchangeName),
		transportBundle:       bundle,
		symbolSvc:             symbolSvc,
		listenKeySvc:          listenKeySvc,
		orderBookSvc:          orderBookSvc,
		transportFactories:    transports,
		routerFactories:       routers,
		cfg:                   config.Apply(settings),
		symbolRefreshInterval: binCfg.SymbolRefreshInterval,
		symbolRefreshDone:     make(chan struct{}),
	}

	// Start background symbol refresh if configured
	if binCfg.SymbolRefreshInterval > 0 {
		ctx, cancel := context.WithCancel(context.Background())
		x.symbolRefreshCancel = cancel
		go x.symbolRefreshLoop(ctx)
	}

	return x, nil
}

// Config returns a snapshot of the active configuration.
func (x *Exchange) Config() config.Settings {
	x.cfgMutex.Lock()
	defer x.cfgMutex.Unlock()
	return config.Apply(x.cfg)
}

// UpdateConfig applies configuration overrides at runtime, rebuilding clients and clearing caches.
func (x *Exchange) UpdateConfig(opts ...config.Option) error {
	base := x.Config()
	newCfg := config.Apply(base, opts...)
	binCfg := resolveBinanceSettings(newCfg)
	transportCfg := bootstrap.TransportConfig{
		APIKey:                binCfg.Credentials.APIKey,
		Secret:                binCfg.Credentials.APISecret,
		SpotBaseURL:           binCfg.REST[config.BinanceRESTSurfaceSpot],
		LinearBaseURL:         binCfg.REST[config.BinanceRESTSurfaceLinear],
		InverseBaseURL:        binCfg.REST[config.BinanceRESTSurfaceInverse],
		HTTPTimeout:           binCfg.HTTPTimeout,
		PublicURL:             binCfg.Websocket.PublicURL,
		PrivateURL:            binCfg.Websocket.PrivateURL,
		HandshakeTimeout:      binCfg.HandshakeTimeout,
		SymbolRefreshInterval: binCfg.SymbolRefreshInterval,
	}

	transportBundle := bootstrap.BuildTransportBundle(x.transportFactories, x.routerFactories, transportCfg)
	restRouter := transportBundle.Router().(routingrest.RESTDispatcher)
	symbolSvc := newSymbolService(restRouter)
	listenKeySvc := newListenKeyService(restRouter)
	wsDeps := newWSDependencies(exchangeName, symbolSvc, listenKeySvc, nil)
	transportBundle.SetWS(x.routerFactories.NewWSRouter(transportBundle.WSInfra(), wsDeps))

	orderBookSvc := newOrderBookService(transportBundle.WS().(wsRouter), restRouter, symbolSvc)

	x.cfgMutex.Lock()
	oldBundle := x.transportBundle
	x.transportBundle = transportBundle
	x.symbolSvc = symbolSvc
	x.listenKeySvc = listenKeySvc
	x.orderBookSvc = orderBookSvc
	x.cfg = newCfg
	x.cfgMutex.Unlock()

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
	// Stop background symbol refresh if running
	if x.symbolRefreshCancel != nil {
		x.symbolRefreshCancel()
		<-x.symbolRefreshDone
	}

	if x.transportBundle != nil {
		return x.transportBundle.Close()
	}
	return nil
}

// RefreshSymbols manually triggers a reload of symbol metadata for the specified markets.
// If no markets are specified, all markets are refreshed.
func (x *Exchange) RefreshSymbols(ctx context.Context, markets ...core.Market) error {
	if x.symbolSvc == nil {
		return internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.Refresh(ctx, markets...)
}

// symbolRefreshLoop periodically refreshes symbol metadata in the background.
func (x *Exchange) symbolRefreshLoop(ctx context.Context) {
	defer close(x.symbolRefreshDone)

	ticker := time.NewTicker(x.symbolRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Use a timeout context for each refresh attempt
			refreshCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := x.symbolSvc.Refresh(refreshCtx, marketsOrAll()...); err != nil {
				// Log error but keep previous snapshot - don't tear down
				log.Printf("binance symbol refresh failed: %v", err)
			}
			cancel()
		}
	}
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

// CanonicalSymbol converts Binance native symbolSvc to canonical form with caching support.
func (x *Exchange) CanonicalSymbol(binanceSymbol string) (string, error) {
	return x.canonicalSymbolForMarkets(context.Background(), binanceSymbol)
}

func (x *Exchange) NativeSymbol(canonical string) (string, error) {
	return x.nativeSymbolForMarkets(context.Background(), canonical)
}

func (x *Exchange) CreateListenKey(ctx context.Context) (string, error) {
	if x.listenKeySvc == nil {
		return "", internal.Exchange("listen key service unavailable")
	}
	return x.listenKeySvc.Create(ctx)
}

func (x *Exchange) KeepAliveListenKey(ctx context.Context, key string) error {
	if x.listenKeySvc == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return x.listenKeySvc.KeepAlive(ctx, key)
}

func (x *Exchange) CloseListenKey(ctx context.Context, key string) error {
	if x.listenKeySvc == nil {
		return internal.Exchange("listen key service unavailable")
	}
	return x.listenKeySvc.Close(ctx, key)
}

func (x *Exchange) OrderBookDepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if x.orderBookSvc == nil {
		return corestreams.BookEvent{}, 0, internal.Exchange("depth snapshot service unavailable")
	}
	return x.orderBookSvc.Snapshot(ctx, symbol, limit)
}

func (x *Exchange) OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	if x.orderBookSvc == nil {
		return nil, nil, internal.Exchange("order book service unavailable")
	}
	return x.orderBookSvc.Subscribe(ctx, symbol)
}

func (x *Exchange) timeInForceCode(t core.TimeInForce) string {
	return internal.MapTimeInForce(t)
}

func (x *Exchange) nativeSymbolForMarkets(ctx context.Context, canonical string, markets ...core.Market) (string, error) {
	if x.symbolSvc == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.nativeForMarkets(ctx, canonical, markets...)
}

func (x *Exchange) canonicalSymbolForMarkets(ctx context.Context, binanceSymbol string, markets ...core.Market) (string, error) {
	if x.symbolSvc == nil {
		return "", internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.canonicalForMarkets(ctx, binanceSymbol, markets...)
}

func (x *Exchange) ensureMarketSymbols(ctx context.Context, market core.Market) error {
	if x.symbolSvc == nil {
		return internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.ensureMarket(ctx, market)
}

func (x *Exchange) instrumentsForMarket(ctx context.Context, market core.Market) ([]core.Instrument, error) {
	if x.symbolSvc == nil {
		return nil, internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.instruments(ctx, market)
}

func (x *Exchange) instrument(ctx context.Context, market core.Market, symbol string) (core.Instrument, bool, error) {
	if x.symbolSvc == nil {
		return core.Instrument{}, false, internal.Exchange("symbol service unavailable")
	}
	return x.symbolSvc.instrument(ctx, market, symbol)
}

func (x *Exchange) restRouter() routingrest.RESTDispatcher {
	if x.transportBundle == nil {
		return nil
	}
	router := x.transportBundle.Router()
	if router == nil {
		return nil
	}
	return router.(routingrest.RESTDispatcher)
}

func (x *Exchange) wsRouter() wsRouter {
	if x.transportBundle == nil {
		return nil
	}
	router := x.transportBundle.WS()
	if router == nil {
		return nil
	}
	return router.(wsRouter)
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
