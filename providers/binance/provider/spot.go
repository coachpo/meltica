package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/providers/binance/infra/rest"
	"github.com/coachpo/meltica/providers/binance/provider/status"
	"github.com/coachpo/meltica/providers/binance/routing"
)

type spotAPI struct{ p *Provider }

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var resp struct {
		ServerTime int64 `json:"serverTime"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/time"}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(resp.ServerTime), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
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
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/exchangeInfo"}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}

	out := make([]core.Instrument, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		var priceScale, qtyScale int
		for _, f := range sdef.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				priceScale = scaleFromStep(f.TickSize)
			case "LOT_SIZE":
				qtyScale = scaleFromStep(f.StepSize)
			}
		}
		sym := core.CanonicalSymbol(sdef.Base, sdef.Quote)
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketSpot, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		s.p.instCache[sym] = inst
		s.p.binToCanon[sdef.Symbol] = sym
	}
	return out, nil
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp struct {
		Bid string `json:"bidPrice"`
		Ask string `json:"askPrice"`
	}
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/ticker/bookTicker", Query: params}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Ticker{}, err
	}
	bid, _ := parseDecimalToRat(resp.Bid)
	ask, _ := parseDecimalToRat(resp.Ask)
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/account", Signed: true}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Balance, 0, len(resp))
	for _, b := range resp {
		avail, _ := parseDecimalToRat(b.Free)
		out = append(out, core.Balance{Asset: b.Asset, Available: avail, Time: time.Now()})
	}
	return out, nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	if since > 0 {
		params["startTime"] = fmt.Sprintf("%d", since)
	}
	var resp []struct {
		ID      int64  `json:"id"`
		Price   string `json:"price"`
		Qty     string `json:"qty"`
		IsBuyer bool   `json:"isBuyer"`
		Time    int64  `json:"time"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/myTrades", Query: params, Signed: true}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}

	out := make([]core.Trade, 0, len(resp))
	for _, tr := range resp {
		price, _ := parseDecimalToRat(tr.Price)
		qty, _ := parseDecimalToRat(tr.Qty)
		side := core.SideSell
		if tr.IsBuyer {
			side = core.SideBuy
		}
		out = append(out, core.Trade{Symbol: symbol, ID: fmt.Sprintf("%d", tr.ID), Price: price, Quantity: qty, Side: side, Time: time.UnixMilli(tr.Time)})
	}
	return out, nil
}

func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if req.ClientID != "" {
		q["newClientOrderId"] = req.ClientID
	}
	if tif := s.p.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	if req.Quantity != nil || req.Price != nil {
		inst := s.lookupInstrument(ctx, req.Symbol)
		if req.Quantity != nil {
			q["quantity"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
		}
		if req.Price != nil {
			q["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
		}
	}

	var resp struct {
		OrderID int64  `json:"orderId"`
		Symbol  string `json:"symbol"`
		Status  string `json:"status"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodPost, Path: "/api/v3/order", Query: q, Signed: true}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: status.MapOrderStatus(resp.Status)}, nil
}

func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	q := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	if id != "" {
		q["orderId"] = id
	}
	if clientID != "" {
		q["origClientOrderId"] = clientID
	}
	var resp struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/order", Query: q, Signed: true}
	if err := s.p.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: symbol, Status: status.MapOrderStatus(resp.Status)}, nil
}

func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	q := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	if id != "" {
		q["orderId"] = id
	}
	if clientID != "" {
		q["origClientOrderId"] = clientID
	}
	msg := routing.RESTMessage{API: rest.SpotAPI, Method: http.MethodDelete, Path: "/api/v3/order", Query: q, Signed: true}
	return s.p.restRouter.Dispatch(ctx, msg, nil)
}

func (s spotAPI) SpotNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance spotAPI: unsupported canonical symbol %s", canonical))
}

func (s spotAPI) SpotCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance spotAPI: unsupported native symbol %s", native))
}

func (s spotAPI) DepthSnapshot(ctx context.Context, symbol string, limit int) (coreprovider.BookEvent, int64, error) {
	return s.p.DepthSnapshot(ctx, symbol, limit)
}

func (s spotAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	if inst, ok := s.p.instCache[symbol]; ok {
		return inst
	}
	insts, err := s.Instruments(ctx)
	if err != nil {
		return core.Instrument{}
	}
	_ = insts
	return s.p.instCache[symbol]
}
