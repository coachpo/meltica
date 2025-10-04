package exchange

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/binance/routing"
	"github.com/coachpo/meltica/exchanges/infra/numeric"
)

type linearAPI struct{ x *Exchange }

func (f linearAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	if err := f.x.ensureMarketSymbols(ctx, core.MarketLinearFutures); err != nil {
		return nil, err
	}
	return f.x.instrumentsForMarket(core.MarketLinearFutures), nil
}

func (f linearAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	native, err := f.FutureNativeSymbol(symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	params := map[string]string{"symbol": native}
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v1/ticker/bookTicker", Query: params}
	if err := f.x.restRouter.Dispatch(ctx, msg, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}

func (f linearAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	nativeSymbol, err := f.FutureNativeSymbol(req.Symbol)
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
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodPost, Path: "/fapi/v1/order", Query: q, Signed: true}
	if err := f.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: internal.MapOrderStatus(resp.Status)}, nil
}

func (f linearAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		nativeSymbol, err := f.FutureNativeSymbol(symbols[0])
		if err != nil {
			return nil, err
		}
		q["symbol"] = nativeSymbol
	}
	var raw []map[string]any
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v2/positionRisk", Query: q, Signed: true}
	if err := f.x.restRouter.Dispatch(ctx, msg, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, d := range raw {
		nativeSym, _ := d["symbol"].(string)
		sym, err := f.x.CanonicalSymbol(nativeSym)
		if err != nil {
			return nil, err
		}
		qStr, _ := d["positionAmt"].(string)
		epStr, _ := d["entryPrice"].(string)
		upStr, _ := d["unRealizedProfit"].(string)
		var qty, ep, up *big.Rat
		if v, ok := parseDecimalToRat(qStr); ok {
			qty = v
		}
		if v, ok := parseDecimalToRat(epStr); ok {
			ep = v
		}
		if v, ok := parseDecimalToRat(upStr); ok {
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

func (f linearAPI) FutureNativeSymbol(canonical string) (string, error) {
	if err := f.x.ensureMarketSymbols(context.Background(), core.MarketLinearFutures); err != nil {
		return "", err
	}
	if native, ok := f.x.symbols.native(core.MarketLinearFutures, canonical); ok {
		return native, nil
	}
	return "", internal.Invalid("linear futures: unsupported canonical symbol %s", canonical)
}

func (f linearAPI) FutureCanonicalSymbol(native string) (string, error) {
	if err := f.x.ensureMarketSymbols(context.Background(), core.MarketLinearFutures); err != nil {
		return "", err
	}
	canonical, ok := f.x.symbols.canonical(native)
	if !ok {
		return "", internal.Invalid("linear futures: unsupported native symbol %s", native)
	}
	if _, ok := f.x.symbols.native(core.MarketLinearFutures, canonical); !ok {
		return "", internal.Invalid("linear futures: unsupported native symbol %s", native)
	}
	return canonical, nil
}

func (f linearAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if err := f.x.ensureMarketSymbols(ctx, core.MarketLinearFutures); err != nil {
		return core.Instrument{}
	}
	inst, _ := f.x.instrument(core.MarketLinearFutures, symbol)
	return inst
}
