package binance

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type spotAPI struct{ x *Exchange }

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var resp struct {
		ServerTime int64 `json:"serverTime"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/time"}
	router := s.x.restRouter()
	if router == nil {
		return time.Time{}, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(resp.ServerTime), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	return s.x.instrumentsForMarket(ctx, core.MarketSpot)
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp tickerResponse
	native, err := s.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	params := map[string]string{"symbol": native}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/ticker/bookTicker", Query: params}
	router := s.x.restRouter()
	if router == nil {
		return core.Ticker{}, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return core.Ticker{}, err
	}
	return buildTicker(symbol, resp.BidPrice, resp.AskPrice), nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/account", Signed: true}
	router := s.x.restRouter()
	if router == nil {
		return nil, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Balance, 0, len(resp))
	for _, b := range resp {
		avail, _ := numeric.Parse(b.Free)
		out = append(out, core.Balance{Asset: b.Asset, Available: avail, Time: time.Now()})
	}
	return out, nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	native, err := s.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return nil, err
	}
	params := map[string]string{"symbol": native}
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
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/myTrades", Query: params, Signed: true}
	router := s.x.restRouter()
	if router == nil {
		return nil, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}

	out := make([]core.Trade, 0, len(resp))
	for _, tr := range resp {
		price, _ := numeric.Parse(tr.Price)
		qty, _ := numeric.Parse(tr.Qty)
		side := core.SideSell
		if tr.IsBuyer {
			side = core.SideBuy
		}
		out = append(out, core.Trade{Symbol: symbol, ID: fmt.Sprintf("%d", tr.ID), Price: price, Quantity: qty, Side: side, Time: time.UnixMilli(tr.Time)})
	}
	return out, nil
}

func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	nativeSymbol, err := s.resolveNativeSymbol(ctx, req.Symbol)
	if err != nil {
		return core.Order{}, err
	}
	q := map[string]string{
		"symbol": nativeSymbol,
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if req.ClientID != "" {
		q["newClientOrderId"] = req.ClientID
	}
	if tif := s.x.timeInForceCode(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	if req.Quantity != nil || req.Price != nil {
		inst := s.lookupInstrument(ctx, req.Symbol)
		if req.Quantity != nil {
			q["quantity"] = numeric.Format(req.Quantity, inst.QtyScale)
		}
		if req.Price != nil {
			q["price"] = numeric.Format(req.Price, inst.PriceScale)
		}
	}

	var resp struct {
		OrderID int64  `json:"orderId"`
		Symbol  string `json:"symbol"`
		Status  string `json:"status"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodPost, Path: "/api/v3/order", Query: q, Signed: true}
	router := s.x.restRouter()
	if router == nil {
		return core.Order{}, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: req.Symbol, Status: internal.MapOrderStatus(resp.Status)}, nil
}

func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	nativeSymbol, err := s.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return core.Order{}, err
	}
	q := map[string]string{"symbol": nativeSymbol}
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
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/order", Query: q, Signed: true}
	router := s.x.restRouter()
	if router == nil {
		return core.Order{}, internal.Invalid("rest router unavailable")
	}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderID), Symbol: symbol, Status: internal.MapOrderStatus(resp.Status)}, nil
}

func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	nativeSymbol, err := s.resolveNativeSymbol(ctx, symbol)
	if err != nil {
		return err
	}
	q := map[string]string{"symbol": nativeSymbol}
	if id != "" {
		q["orderId"] = id
	}
	if clientID != "" {
		q["origClientOrderId"] = clientID
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodDelete, Path: "/api/v3/order", Query: q, Signed: true}
	router := s.x.restRouter()
	if router == nil {
		return internal.Invalid("rest router unavailable")
	}
	return router.Dispatch(ctx, msg, nil)
}

func (s spotAPI) OrderBookDepthSnapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	return s.x.OrderBookDepthSnapshot(ctx, symbol, limit)
}

func (s spotAPI) lookupInstrument(ctx context.Context, symbol string) core.Instrument {
	inst, ok, err := s.x.instrument(ctx, core.MarketSpot, symbol)
	if err != nil || !ok {
		return core.Instrument{}
	}
	return inst
}

func (s spotAPI) resolveNativeSymbol(ctx context.Context, canonical string) (string, error) {
	return s.x.nativeSymbolForMarkets(ctx, canonical, core.MarketSpot)
}

func (s spotAPI) resolveCanonicalSymbol(ctx context.Context, native string) (string, error) {
	return s.x.canonicalSymbolForMarkets(ctx, native, core.MarketSpot)
}
