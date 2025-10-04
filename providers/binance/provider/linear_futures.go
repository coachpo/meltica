package provider

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/providers/binance/infra/rest"
	"github.com/coachpo/meltica/providers/binance/provider/status"
	"github.com/coachpo/meltica/providers/binance/routing"
)

type linearAPI struct{ p *Provider }

func (f linearAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Base    string `json:"baseAsset"`
			Quote   string `json:"quoteAsset"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v1/exchangeInfo"}
	if err := f.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		var priceScale, qtyScale int
		for _, ft := range sdef.Filters {
			switch ft.FilterType {
			case "PRICE_FILTER":
				priceScale = scaleFromStep(ft.TickSize)
			case "LOT_SIZE":
				qtyScale = scaleFromStep(ft.StepSize)
			}
		}
		sym := core.CanonicalSymbol(sdef.Base, sdef.Quote)
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketLinearFutures, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		f.p.instCache[sym] = inst
	}
	return out, nil
}

func (f linearAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v1/ticker/bookTicker", Query: params}
	if err := f.p.restRouter.Dispatch(ctx, msg, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}

func (f linearAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := f.p.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	inst := f.lookupInstrument(ctx, req.Symbol)
	if req.Quantity != nil {
		q["quantity"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		q["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	var resp struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodPost, Path: "/fapi/v1/order", Query: q, Signed: true}
	if err := f.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: status.MapOrderStatus(resp.Status)}, nil
}

func (f linearAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		q["symbol"] = core.CanonicalToBinance(symbols[0])
	}
	var raw []map[string]any
	msg := routing.RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v2/positionRisk", Query: q, Signed: true}
	if err := f.p.restRouter.Dispatch(ctx, msg, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, d := range raw {
		nativeSym, _ := d["symbol"].(string)
		sym := f.p.CanonicalSymbol(nativeSym)
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

func (f linearAPI) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance futuresAPI: unsupported canonical symbol %s", canonical))
}

func (f linearAPI) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance futuresAPI: unsupported native symbol %s", native))
}

func (f linearAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if inst, ok := f.p.instCache[symbol]; ok {
		return inst
	}
	insts, err := f.Instruments(ctx)
	if err != nil {
		return core.Instrument{}
	}
	_ = insts
	return f.p.instCache[symbol]
}
