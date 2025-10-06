package binance

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
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
	if err := f.x.ensureMarketSymbols(ctx, f.market); err != nil {
		return nil, err
	}
	return f.x.instrumentsForMarket(f.market), nil
}

func (f *futuresAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp tickerResponse
	native, err := f.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	params := map[string]string{"symbol": native}
	msg := routingrest.RESTMessage{API: f.endpoints.api, Method: http.MethodGet, Path: f.endpoints.tickerPath, Query: params}
	if err := f.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Ticker{}, err
	}
	return buildTicker(symbol, resp.BidPrice, resp.AskPrice), nil
}

func (f *futuresAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	nativeSymbol, err := f.resolveNativeSymbol(ctx, req.Symbol)
	if err != nil {
		return core.Order{}, err
	}
	q := map[string]string{
		"symbol": nativeSymbol,
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := f.x.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	inst := f.lookupInstrument(ctx, req.Symbol)
	if req.Quantity != nil {
		q["quantity"] = numeric.Format(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		q["price"] = numeric.Format(req.Price, inst.PriceScale)
	}
	var resp struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	msg := routingrest.RESTMessage{API: f.endpoints.api, Method: http.MethodPost, Path: f.endpoints.orderPath, Query: q, Signed: true}
	if err := f.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: internal.MapOrderStatus(resp.Status)}, nil
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
	if err := f.x.restRouter.Dispatch(ctx, msg, &raw); err != nil {
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

func (f *futuresAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if err := f.x.ensureMarketSymbols(ctx, f.market); err != nil {
		return core.Instrument{}
	}
	inst, _ := f.x.instrument(f.market, symbol)
	return inst
}

func (f *futuresAPI) resolveNativeSymbol(ctx context.Context, canonical string) (string, error) {
	return f.x.nativeSymbolForMarkets(ctx, canonical, f.market)
}

func (f *futuresAPI) resolveCanonicalSymbol(ctx context.Context, native string) (string, error) {
	return f.x.canonicalSymbolForMarkets(ctx, native, f.market)
}

// compile-time assertions to ensure futuresAPI satisfies the contract.
var _ core.FuturesAPI = (*futuresAPI)(nil)
