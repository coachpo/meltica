package binance

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/bootstrap"
	exchangecap "github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/core/layers"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
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
	bookSvc            *BookService
	trading            tradingSuite
	transportFactories bootstrap.TransportFactories
	routerFactories    bootstrap.RouterFactories
	cfg                config.Settings
	cfgMutex           sync.Mutex

	// Symbol refresh controls
	symbolRefreshInterval time.Duration
	symbolRefreshCancel   context.CancelFunc
	symbolRefreshDone     chan struct{}
}

type tradingSuite struct {
	spot    bootstrap.TradingService
	linear  bootstrap.TradingService
	inverse bootstrap.TradingService
}

type restDispatcherProvider interface {
	LegacyRESTDispatcher() routingrest.RESTDispatcher
}

func resolveRESTRouting(router interface{}) (layers.RESTRouting, routingrest.RESTDispatcher) {
	if router == nil {
		return nil, nil
	}
	switch v := router.(type) {
	case layers.RESTRouting:
		return v, extractRESTDispatcher(v)
	case interface{ AsLayerInterface() layers.RESTRouting }:
		restRouting := v.AsLayerInterface()
		return restRouting, extractRESTDispatcher(restRouting)
	case routingrest.RESTDispatcher:
		restRouting := bnrouting.LegacyRESTRoutingAdapter(v)
		return restRouting, v
	default:
		return nil, nil
	}
}

func extractRESTDispatcher(r layers.RESTRouting) routingrest.RESTDispatcher {
	if provider, ok := r.(restDispatcherProvider); ok {
		return provider.LegacyRESTDispatcher()
	}
	return nil
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
	core.CapabilityMarketTrades,
	core.CapabilityMarketTicker,
	core.CapabilityMarketOrderBook,
)

// Capabilities reports the capability bitset exposed by the Binance adapter.
func Capabilities() core.ExchangeCapabilities { return capabilities }

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
	_, restDispatcher := resolveRESTRouting(bundle.Router())
	if restDispatcher == nil {
		return nil, fmt.Errorf("binance: rest router unavailable")
	}
	symbolSvc := newSymbolService(restDispatcher)
	listenKeySvc := newListenKeyService(restDispatcher)
	wsDeps := newWSDependencies(exchangeName, symbolSvc, listenKeySvc, nil)
	bundle.SetWS(routers.NewWSRouter(bundle.WSInfra(), wsDeps))

	bookSvc := newBookService(bundle.WS().(wsRouter), restDispatcher, symbolSvc)

	x := &Exchange{
		name:                  string(exchangeName),
		transportBundle:       bundle,
		symbolSvc:             symbolSvc,
		listenKeySvc:          listenKeySvc,
		bookSvc:               bookSvc,
		transportFactories:    transports,
		routerFactories:       routers,
		cfg:                   config.Apply(settings),
		symbolRefreshInterval: binCfg.SymbolRefreshInterval,
		symbolRefreshDone:     make(chan struct{}),
	}
	services, err := x.buildTradingServices(restDispatcher, symbolSvc)
	if err != nil {
		return nil, err
	}
	x.trading = services

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
	_, restDispatcher := resolveRESTRouting(transportBundle.Router())
	if restDispatcher == nil {
		return fmt.Errorf("binance: rest router unavailable")
	}
	symbolSvc := newSymbolService(restDispatcher)
	listenKeySvc := newListenKeyService(restDispatcher)
	wsDeps := newWSDependencies(exchangeName, symbolSvc, listenKeySvc, nil)
	transportBundle.SetWS(x.routerFactories.NewWSRouter(transportBundle.WSInfra(), wsDeps))

	bookSvc := newBookService(transportBundle.WS().(wsRouter), restDispatcher, symbolSvc)
	services, err := x.buildTradingServices(restDispatcher, symbolSvc)
	if err != nil {
		return err
	}

	x.cfgMutex.Lock()
	oldBundle := x.transportBundle
	x.transportBundle = transportBundle
	x.symbolSvc = symbolSvc
	x.listenKeySvc = listenKeySvc
	x.bookSvc = bookSvc
	x.trading = services
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

func (x *Exchange) BookDepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if x.bookSvc == nil {
		return corestreams.BookEvent{}, 0, internal.Exchange("depth snapshot service unavailable")
	}
	return x.bookSvc.Snapshot(ctx, symbol, limit)
}

func (x *Exchange) BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	if x.bookSvc == nil {
		return nil, nil, internal.Exchange("book service unavailable")
	}
	return x.bookSvc.Subscribe(ctx, symbol)
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

func (x *Exchange) tradingServiceFor(market core.Market) bootstrap.TradingService {
	switch market {
	case core.MarketSpot:
		return x.trading.spot
	case core.MarketLinearFutures:
		return x.trading.linear
	case core.MarketInverseFutures:
		return x.trading.inverse
	default:
		return nil
	}
}

func (x *Exchange) restRouter() routingrest.RESTDispatcher {
	if x.transportBundle == nil {
		return nil
	}
	_, dispatcher := resolveRESTRouting(x.transportBundle.Router())
	return dispatcher
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

func (x *Exchange) buildTradingServices(router routingrest.RESTDispatcher, symbols *symbolService) (tradingSuite, error) {
	if router == nil {
		return tradingSuite{}, internal.Exchange("trading: rest router unavailable")
	}
	if symbols == nil {
		return tradingSuite{}, internal.Exchange("trading: symbol service unavailable")
	}
	build := func(cfg bnrouting.OrderTranslatorConfig, caps exchangecap.Set, reqs map[routingrest.OrderAction][]exchangecap.Capability) (bootstrap.TradingService, error) {
		translator, err := bnrouting.NewOrderTranslator(cfg)
		if err != nil {
			return nil, err
		}
		options := []routingrest.OrderRouterOption{routingrest.WithCapabilities(caps)}
		for action, deps := range reqs {
			if len(deps) == 0 {
				continue
			}
			options = append(options, routingrest.WithCapabilityRequirement(action, deps...))
		}
		r, err := routingrest.NewOrderRouter(router, translator, options...)
		if err != nil {
			return nil, err
		}
		return bootstrap.NewTradingService(bootstrap.TradingServiceConfig{Router: r})
	}

	spotCfg := x.orderTranslatorConfig(symbols, core.MarketSpot, string(rest.SpotAPI), "/api/v3/order", false)
	spotCaps := exchangecap.Of(exchangecap.CapabilitySpotTradingREST, exchangecap.CapabilityTradingSpotAmend, exchangecap.CapabilityTradingSpotCancel)
	spotReqs := map[routingrest.OrderAction][]exchangecap.Capability{
		routingrest.ActionPlace:  {exchangecap.CapabilitySpotTradingREST},
		routingrest.ActionAmend:  {exchangecap.CapabilitySpotTradingREST, exchangecap.CapabilityTradingSpotAmend},
		routingrest.ActionGet:    {exchangecap.CapabilitySpotTradingREST},
		routingrest.ActionCancel: {exchangecap.CapabilityTradingSpotCancel},
	}
	spotService, err := build(spotCfg, spotCaps, spotReqs)
	if err != nil {
		return tradingSuite{}, err
	}

	linearCfg := x.orderTranslatorConfig(symbols, core.MarketLinearFutures, linearFuturesEndpoints.api, linearFuturesEndpoints.orderPath, true)
	linearCaps := exchangecap.Of(exchangecap.CapabilityLinearTradingREST, exchangecap.CapabilityTradingLinearAmend, exchangecap.CapabilityTradingLinearCancel)
	linearReqs := map[routingrest.OrderAction][]exchangecap.Capability{
		routingrest.ActionPlace:  {exchangecap.CapabilityLinearTradingREST},
		routingrest.ActionAmend:  {exchangecap.CapabilityLinearTradingREST, exchangecap.CapabilityTradingLinearAmend},
		routingrest.ActionGet:    {exchangecap.CapabilityLinearTradingREST},
		routingrest.ActionCancel: {exchangecap.CapabilityTradingLinearCancel},
	}
	linearService, err := build(linearCfg, linearCaps, linearReqs)
	if err != nil {
		return tradingSuite{}, err
	}

	inverseCfg := x.orderTranslatorConfig(symbols, core.MarketInverseFutures, inverseFuturesEndpoints.api, inverseFuturesEndpoints.orderPath, true)
	inverseCaps := exchangecap.Of(exchangecap.CapabilityInverseTradingREST, exchangecap.CapabilityTradingInverseAmend, exchangecap.CapabilityTradingInverseCancel)
	inverseReqs := map[routingrest.OrderAction][]exchangecap.Capability{
		routingrest.ActionPlace:  {exchangecap.CapabilityInverseTradingREST},
		routingrest.ActionAmend:  {exchangecap.CapabilityInverseTradingREST, exchangecap.CapabilityTradingInverseAmend},
		routingrest.ActionGet:    {exchangecap.CapabilityInverseTradingREST},
		routingrest.ActionCancel: {exchangecap.CapabilityTradingInverseCancel},
	}
	inverseService, err := build(inverseCfg, inverseCaps, inverseReqs)
	if err != nil {
		return tradingSuite{}, err
	}

	return tradingSuite{spot: spotService, linear: linearService, inverse: inverseService}, nil
}

func (x *Exchange) orderTranslatorConfig(symbols *symbolService, market core.Market, api, path string, reduceOnly bool) bnrouting.OrderTranslatorConfig {
	return bnrouting.OrderTranslatorConfig{
		Market:    market,
		API:       api,
		OrderPath: path,
		ResolveNative: func(ctx context.Context, symbol string) (string, error) {
			return symbols.nativeForMarkets(ctx, symbol, market)
		},
		LookupInstrument: func(ctx context.Context, symbol string) (core.Instrument, error) {
			inst, ok, err := symbols.instrument(ctx, market, symbol)
			if err != nil {
				return core.Instrument{}, err
			}
			if !ok {
				return core.Instrument{}, internal.Invalid("instrument not found for %s", symbol)
			}
			return inst, nil
		},
		TimeInForce:       x.timeInForceCode,
		SupportReduceOnly: reduceOnly,
	}
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
