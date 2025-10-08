package binance

import (
	"context"
	"math/big"
	"net/http"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
	"github.com/coachpo/meltica/internal/numeric"
)

type futuresEndpoints struct {
	api          string
	tickerPath   string
	orderPath    string
	positionPath string
}

type futuresAPI struct {
	x         *Exchange
	market    core.Market
	endpoints futuresEndpoints
}

func newFuturesAPI(x *Exchange, market core.Market, endpoints futuresEndpoints) *futuresAPI {
	return &futuresAPI{x: x, market: market, endpoints: endpoints}
}

func (f *futuresAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	return f.x.instrumentsForMarket(ctx, f.market)
}

func (f *futuresAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp tickerResponse
	native, err := f.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	params := map[string]string{"symbol": native}
	msg := routingrest.RESTMessage{API: f.endpoints.api, Method: http.MethodGet, Path: f.endpoints.tickerPath, Query: params}
	router := f.x.restRouter()
	if router == nil {
		return core.Ticker{}, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return core.Ticker{}, err
	}
	return buildTicker(symbol, resp.BidPrice, resp.AskPrice), nil
}

func (f *futuresAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	svc := f.x.tradingServiceFor(f.market)
	if svc == nil {
		return core.Order{}, internal.Invalid("trading service unavailable")
	}
	return svc.Place(ctx, req)
}

func (f *futuresAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		nativeSymbol, err := f.resolveNativeSymbol(ctx, symbols[0])
		if err != nil {
			return nil, err
		}
		q["symbol"] = nativeSymbol
	}
	var raw []map[string]any
	msg := routingrest.RESTMessage{API: f.endpoints.api, Method: http.MethodGet, Path: f.endpoints.positionPath, Query: q, Signed: true}
	router := f.x.restRouter()
	if router == nil {
		return nil, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, d := range raw {
		nativeSym, _ := d["symbol"].(string)
		sym, err := f.resolveCanonicalSymbol(ctx, nativeSym)
		if err != nil {
			return nil, err
		}
		qStr, _ := d["positionAmt"].(string)
		epStr, _ := d["entryPrice"].(string)
		upStr, _ := d["unRealizedProfit"].(string)
		var qty, ep, up *big.Rat
		if v, ok := numeric.Parse(qStr); ok {
			qty = v
		}
		if v, ok := numeric.Parse(epStr); ok {
			ep = v
		}
		if v, ok := numeric.Parse(upStr); ok {
			up = v
		}
		side := core.SideBuy
		if qty != nil && qty.Sign() < 0 {
			side = core.SideSell
			qty = new(big.Rat).Abs(qty)
		}
		out = append(out, core.Position{Symbol: sym, Side: side, Quantity: qty, EntryPrice: ep, Unrealized: up, UpdatedAt: time.Now()})
	}
	return out, nil
}

func (f *futuresAPI) resolveNativeSymbol(ctx context.Context, canonical string) (string, error) {
	return f.x.nativeSymbolForMarkets(ctx, canonical, f.market)
}

func (f *futuresAPI) resolveCanonicalSymbol(ctx context.Context, native string) (string, error) {
	return f.x.canonicalSymbolForMarkets(ctx, native, f.market)
}

// compile-time assertions to ensure futuresAPI satisfies the contract.
var _ core.FuturesAPI = (*futuresAPI)(nil)
