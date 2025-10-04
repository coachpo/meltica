package exchange

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/common"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

type inverseAPI struct{ x *Exchange }

func (d inverseAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
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
	msg := routing.RESTMessage{API: rest.InverseAPI, Method: http.MethodGet, Path: "/dapi/v1/exchangeInfo"}
	if err := d.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
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
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketInverseFutures, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		d.x.instCache[sym] = inst
	}
	return out, nil
}

func (d inverseAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	msg := routing.RESTMessage{API: rest.InverseAPI, Method: http.MethodGet, Path: "/dapi/v1/ticker/bookTicker", Query: params}
	if err := d.x.restRouter.Dispatch(ctx, msg, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}

func (d inverseAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := d.x.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	inst := d.lookupInstrument(ctx, req.Symbol)
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
	msg := routing.RESTMessage{API: rest.InverseAPI, Method: http.MethodPost, Path: "/dapi/v1/order", Query: q, Signed: true}
	if err := d.x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: common.MapOrderStatus(resp.Status)}, nil
}

func (d inverseAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		q["symbol"] = core.CanonicalToBinance(symbols[0])
	}
	var raw []map[string]any
	msg := routing.RESTMessage{API: rest.InverseAPI, Method: http.MethodGet, Path: "/dapi/v1/positionRisk", Query: q, Signed: true}
	if err := d.x.restRouter.Dispatch(ctx, msg, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, drec := range raw {
		nativeSym, _ := drec["symbol"].(string)
		sym := d.x.CanonicalSymbol(nativeSym)
		qStr, _ := drec["positionAmt"].(string)
		epStr, _ := drec["entryPrice"].(string)
		upStr, _ := drec["unRealizedProfit"].(string)
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

func (d inverseAPI) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance inverse futuresAPI: unsupported canonical symbol %s", canonical))
}

func (d inverseAPI) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance inverse futuresAPI: unsupported native symbol %s", native))
}

func (d inverseAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if inst, ok := d.x.instCache[symbol]; ok {
		return inst
	}
	insts, err := d.Instruments(ctx)
	if err != nil {
		return core.Instrument{}
	}
	_ = insts
	return d.x.instCache[symbol]
}
