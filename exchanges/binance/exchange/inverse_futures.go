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
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type inverseAPI struct{ x *Exchange }

func (d inverseAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	if err := d.x.ensureMarketSymbols(ctx, core.MarketInverseFutures); err != nil {
		return nil, err
	}
	return d.x.instrumentsForMarket(core.MarketInverseFutures), nil
}

func (d inverseAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	native, err := d.FutureNativeSymbol(symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	params := map[string]string{"symbol": native}
	msg := routingrest.RESTMessage{API: string(rest.InverseAPI), Method: http.MethodGet, Path: "/dapi/v1/ticker/bookTicker", Query: params}
	if err := d.x.restRouter.Dispatch(ctx, msg, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}

func (d inverseAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	nativeSymbol, err := d.FutureNativeSymbol(req.Symbol)
	if err != nil {
		return core.Order{}, err
	}
	q := map[string]string{
		"symbol": nativeSymbol,
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := d.x.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	inst := d.lookupInstrument(ctx, req.Symbol)
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
	msg := routingrest.RESTMessage{API: string(rest.InverseAPI), Method: http.MethodPost, Path: "/dapi/v1/order", Query: q, Signed: true}
	if err := d.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: internal.MapOrderStatus(resp.Status)}, nil
}

func (d inverseAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		nativeSymbol, err := d.FutureNativeSymbol(symbols[0])
		if err != nil {
			return nil, err
		}
		q["symbol"] = nativeSymbol
	}
	var raw []map[string]any
	msg := routingrest.RESTMessage{API: string(rest.InverseAPI), Method: http.MethodGet, Path: "/dapi/v1/positionRisk", Query: q, Signed: true}
	if err := d.x.restRouter.Dispatch(ctx, msg, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, drec := range raw {
		nativeSym, _ := drec["symbol"].(string)
		sym, err := d.x.CanonicalSymbol(nativeSym)
		if err != nil {
			return nil, err
		}
		qStr, _ := drec["positionAmt"].(string)
		epStr, _ := drec["entryPrice"].(string)
		upStr, _ := drec["unRealizedProfit"].(string)
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

func (d inverseAPI) FutureNativeSymbol(canonical string) (string, error) {
	if err := d.x.ensureMarketSymbols(context.Background(), core.MarketInverseFutures); err != nil {
		return "", err
	}
	if native, ok := d.x.symbols.native(core.MarketInverseFutures, canonical); ok {
		return native, nil
	}
	return "", internal.Invalid("inverse futures: unsupported canonical symbol %s", canonical)
}

func (d inverseAPI) FutureCanonicalSymbol(native string) (string, error) {
	if err := d.x.ensureMarketSymbols(context.Background(), core.MarketInverseFutures); err != nil {
		return "", err
	}
	canonical, ok := d.x.symbols.canonical(native)
	if !ok {
		return "", internal.Invalid("inverse futures: unsupported native symbol %s", native)
	}
	if _, ok := d.x.symbols.native(core.MarketInverseFutures, canonical); !ok {
		return "", internal.Invalid("inverse futures: unsupported native symbol %s", native)
	}
	return canonical, nil
}

func (d inverseAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if err := d.x.ensureMarketSymbols(ctx, core.MarketInverseFutures); err != nil {
		return core.Instrument{}
	}
	inst, _ := d.x.instrument(core.MarketInverseFutures, symbol)
	return inst
}
