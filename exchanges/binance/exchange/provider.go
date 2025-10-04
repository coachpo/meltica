package exchange

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

type Exchange struct {
	name string

	restClient *rest.Client
	restRouter *routing.RESTRouter

	wsInfra  *ws.Client
	wsRouter *routing.WSRouter

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

func New(apiKey, secret string, opts ...config.Option) (*Exchange, error) {
	settings := config.FromEnv()
	settings = config.Apply(settings, opts...)
	if v := strings.TrimSpace(apiKey); v != "" {
		settings.Binance.APIKey = v
	}
	if v := strings.TrimSpace(secret); v != "" {
		settings.Binance.APISecret = v
	}
	return NewWithSettings(settings)
}

func NewWithSettings(settings config.Settings) (*Exchange, error) {
	restClient := rest.NewClient(rest.Config{
		APIKey:         settings.Binance.APIKey,
		Secret:         settings.Binance.APISecret,
		SpotBaseURL:    settings.Binance.SpotBaseURL,
		LinearBaseURL:  settings.Binance.LinearBaseURL,
		InverseBaseURL: settings.Binance.InverseBaseURL,
		Timeout:        settings.Binance.HTTPTimeout,
	})
	restRouter := routing.NewRESTRouter(restClient)
	wsInfra := ws.NewClient(ws.Config{
		PublicURL:        settings.Binance.PublicWSURL,
		PrivateURL:       settings.Binance.PrivateWSURL,
		HandshakeTimeout: settings.Binance.HandshakeTimeout,
	})

	x := &Exchange{
		name:       "binance",
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

// Config returns a snapshot of the active configuration.
func (x *Exchange) Config() config.Settings {
	x.cfgMu.Lock()
	defer x.cfgMu.Unlock()
	return x.cfg
}

// UpdateConfig applies configuration overrides at runtime, rebuilding clients and clearing caches.
func (x *Exchange) UpdateConfig(opts ...config.Option) error {
	base := x.Config()
	newCfg := config.Apply(base, opts...)
	restClient := rest.NewClient(rest.Config{
		APIKey:         newCfg.Binance.APIKey,
		Secret:         newCfg.Binance.APISecret,
		SpotBaseURL:    newCfg.Binance.SpotBaseURL,
		LinearBaseURL:  newCfg.Binance.LinearBaseURL,
		InverseBaseURL: newCfg.Binance.InverseBaseURL,
		Timeout:        newCfg.Binance.HTTPTimeout,
	})
	restRouter := routing.NewRESTRouter(restClient)
	wsInfra := ws.NewClient(ws.Config{
		PublicURL:        newCfg.Binance.PublicWSURL,
		PrivateURL:       newCfg.Binance.PrivateWSURL,
		HandshakeTimeout: newCfg.Binance.HandshakeTimeout,
	})

	x.cfgMu.Lock()
	if x.wsRouter != nil {
		_ = x.wsRouter.Close()
	}
	x.restClient = restClient
	x.restRouter = restRouter
	x.wsInfra = wsInfra
	x.wsRouter = routing.NewWSRouter(wsInfra, x)
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

func (x *Exchange) Spot(ctx context.Context) core.SpotAPI             { return spotAPI{x} }
func (x *Exchange) LinearFutures(ctx context.Context) core.FuturesAPI { return linearAPI{x} }
func (x *Exchange) InverseFutures(ctx context.Context) core.FuturesAPI {
	return inverseAPI{x}
}

func (x *Exchange) WS() core.WS { return newWSService(x.wsRouter) }

func (x *Exchange) Close() error {
	if x.wsRouter != nil {
		_ = x.wsRouter.Close()
	}
	return nil
}

// CanonicalSymbol converts Binance native symbols to canonical form with caching support.
func (x *Exchange) CanonicalSymbol(binanceSymbol string) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if trimmed == "" {
		return "", internal.Invalid("unsupported symbol %s", binanceSymbol)
	}
	if err := x.ensureAllSymbols(context.Background()); err != nil {
		return "", err
	}
	if canonical, ok := x.symbols.canonical(trimmed); ok {
		return canonical, nil
	}
	return "", internal.Exchange("unsupported symbol %s", trimmed)
}

func (x *Exchange) NativeSymbol(canonical string) (string, error) {
	if err := x.ensureAllSymbols(context.Background()); err != nil {
		return "", err
	}
	if native, ok := x.symbols.nativeAny(canonical); ok {
		return native, nil
	}
	return "", internal.Invalid("unsupported symbol %s", canonical)
}

func (x *Exchange) CreateListenKey(ctx context.Context) (string, error) {
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodPost, Path: "/api/v3/userDataStream"}
	if err := x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (x *Exchange) KeepAliveListenKey(ctx context.Context, key string) error {
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodPut, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return x.restRouter.Dispatch(ctx, msg, nil)
}

func (x *Exchange) CloseListenKey(ctx context.Context, key string) error {
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodDelete, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return x.restRouter.Dispatch(ctx, msg, nil)
}

func (x *Exchange) DepthSnapshot(ctx context.Context, symbol string, limit int) (coreexchange.BookEvent, int64, error) {
	if err := x.ensureMarketSymbols(ctx, core.MarketSpot); err != nil {
		return coreexchange.BookEvent{}, 0, err
	}
	native, ok := x.symbols.native(core.MarketSpot, symbol)
	if !ok {
		if fallback, found := x.symbols.nativeAny(symbol); found {
			native = fallback
		} else {
			return coreexchange.BookEvent{}, 0, internal.Invalid("depth snapshot: unsupported symbol %s", symbol)
		}
	}
	params := map[string]string{"symbol": native, "limit": fmt.Sprintf("%d", limit)}
	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/depth", Query: params}
	if err := x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return coreexchange.BookEvent{}, 0, err
	}
	bids := parseDepthLevels(resp.Bids)
	asks := parseDepthLevels(resp.Asks)
	event := coreexchange.BookEvent{Symbol: symbol, Bids: bids, Asks: asks, Time: time.Now()}
	return event, resp.LastUpdateID, nil
}

func (x *Exchange) timeInForceCode(t core.TimeInForce) string {
	return internal.MapTimeInForce(t)
}
