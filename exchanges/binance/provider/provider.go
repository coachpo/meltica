package exchange

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/exchanges/binance/common"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/wsinfra"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

type Exchange struct {
	name string

	restClient *rest.Client
	restRouter *routing.RESTRouter

	wsInfra  *wsinfra.Client
	wsRouter *routing.WSRouter

	instCache  map[string]core.Instrument
	binToCanon map[string]string
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

func New(apiKey, secret string) (*Exchange, error) {
	restClient := rest.NewClient(rest.Config{APIKey: apiKey, Secret: secret})
	restRouter := routing.NewRESTRouter(restClient)
	wsInfra := wsinfra.NewClient()

	x := &Exchange{
		name:       "binance",
		restClient: restClient,
		restRouter: restRouter,
		wsInfra:    wsInfra,
		instCache:  map[string]core.Instrument{},
		binToCanon: map[string]string{},
	}
	x.wsRouter = routing.NewWSRouter(wsInfra, x)
	return x, nil
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
func (x *Exchange) CanonicalSymbol(binanceSymbol string) string {
	s := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if s == "" {
		fmt.Fprintf(os.Stderr, "ERROR: binance: empty symbol in WSCanonicalSymbol\n")
		panic("binance: empty symbol in WSCanonicalSymbol")
	}

	if canonical, ok := x.binToCanon[s]; ok {
		return canonical
	}

	if s == "BTCUSDT" {
		x.binToCanon[s] = "BTC-USDT"
		return "BTC-USDT"
	}

	for i := len(s) - 4; i >= 3; i-- {
		if s[i:] == "USDT" || s[i:] == "BUSD" || s[i:] == "USDC" {
			base := s[:i]
			quote := s[i:]
			canonical := fmt.Sprintf("%s-%s", base, quote)
			x.binToCanon[s] = canonical
			return canonical
		}
	}

	fmt.Fprintf(os.Stderr, "ERROR: binance: unsupported symbol '%s'\n", s)
	panic(fmt.Errorf("binance: unsupported symbol %s", s))
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
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol), "limit": fmt.Sprintf("%d", limit)}
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
	return common.MapTimeInForce(t)
}
