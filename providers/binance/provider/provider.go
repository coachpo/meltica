package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/providers/binance/infra/rest"
	"github.com/coachpo/meltica/providers/binance/infra/wsinfra"
	"github.com/coachpo/meltica/providers/binance/routing"
)

type Provider struct {
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

func New(apiKey, secret string) (*Provider, error) {
	restClient := rest.NewClient(rest.Config{APIKey: apiKey, Secret: secret})
	restRouter := routing.NewRESTRouter(restClient)
	wsInfra := wsinfra.NewClient()

	p := &Provider{
		name:       "binance",
		restClient: restClient,
		restRouter: restRouter,
		wsInfra:    wsInfra,
		instCache:  map[string]core.Instrument{},
		binToCanon: map[string]string{},
	}
	p.wsRouter = routing.NewWSRouter(wsInfra, p)
	return p, nil
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Capabilities() core.ProviderCapabilities { return capabilities }

func (p *Provider) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (p *Provider) Spot(ctx context.Context) core.SpotAPI             { return spotAPI{p} }
func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI { return linearAPI{p} }
func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI {
	return inverseAPI{p}
}

func (p *Provider) WS() core.WS { return newWSService(p.wsRouter) }

func (p *Provider) Close() error {
	if p.wsRouter != nil {
		_ = p.wsRouter.Close()
	}
	return nil
}

// CanonicalSymbol converts Binance native symbols to canonical form with caching support.
func (p *Provider) CanonicalSymbol(binanceSymbol string) string {
	s := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if s == "" {
		fmt.Fprintf(os.Stderr, "ERROR: binance: empty symbol in WSCanonicalSymbol\n")
		panic("binance: empty symbol in WSCanonicalSymbol")
	}

	if canonical, ok := p.binToCanon[s]; ok {
		return canonical
	}

	if s == "BTCUSDT" {
		p.binToCanon[s] = "BTC-USDT"
		return "BTC-USDT"
	}

	for i := len(s) - 4; i >= 3; i-- {
		if s[i:] == "USDT" || s[i:] == "BUSD" || s[i:] == "USDC" {
			base := s[:i]
			quote := s[i:]
			canonical := fmt.Sprintf("%s-%s", base, quote)
			p.binToCanon[s] = canonical
			return canonical
		}
	}

	fmt.Fprintf(os.Stderr, "ERROR: binance: unsupported symbol '%s'\n", s)
	panic(fmt.Errorf("binance: unsupported symbol %s", s))
}

func (p *Provider) CreateListenKey(ctx context.Context) (string, error) {
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodPost, Path: "/api/v3/userDataStream"}
	if err := p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (p *Provider) KeepAliveListenKey(ctx context.Context, key string) error {
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodPut, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return p.restRouter.Dispatch(ctx, msg, nil)
}

func (p *Provider) CloseListenKey(ctx context.Context, key string) error {
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodDelete, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return p.restRouter.Dispatch(ctx, msg, nil)
}

func (p *Provider) DepthSnapshot(ctx context.Context, symbol string, limit int) (coreprovider.BookEvent, int64, error) {
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol), "limit": fmt.Sprintf("%d", limit)}
	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/depth", Query: params}
	if err := p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return coreprovider.BookEvent{}, 0, err
	}
	bids := parseDepthLevels(resp.Bids)
	asks := parseDepthLevels(resp.Asks)
	event := coreprovider.BookEvent{Symbol: symbol, Bids: bids, Asks: asks, Time: time.Now()}
	return event, resp.LastUpdateID, nil
}

func (p *Provider) timeInForceCode(t core.TimeInForce) string {
	return MapTimeInForce(t)
}
