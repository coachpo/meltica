package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	sharedrouting "github.com/coachpo/meltica/exchanges/shared/routing"
	"github.com/coachpo/meltica/internal/numeric"
)

// OrderTranslatorConfig wires venue-specific behaviour required to construct REST payloads.
type OrderTranslatorConfig struct {
	Market            core.Market
	API               string
	OrderPath         string
	ResolveNative     func(context.Context, string) (string, error)
	LookupInstrument  func(context.Context, string) (core.Instrument, error)
	TimeInForce       func(core.TimeInForce) string
	SupportReduceOnly bool
}

// NewOrderTranslator returns a Binance-specific implementation of the shared order translator.
func NewOrderTranslator(cfg OrderTranslatorConfig) (sharedrouting.OrderTranslator, error) {
	if cfg.ResolveNative == nil {
		return nil, fmt.Errorf("binance: ResolveNative must be provided")
	}
	if cfg.LookupInstrument == nil {
		return nil, fmt.Errorf("binance: LookupInstrument must be provided")
	}
	if strings.TrimSpace(cfg.API) == "" {
		return nil, fmt.Errorf("binance: API surface must be provided")
	}
	if strings.TrimSpace(cfg.OrderPath) == "" {
		return nil, fmt.Errorf("binance: order path must be provided")
	}
	return &orderTranslator{cfg: cfg}, nil
}

type orderTranslator struct {
	cfg OrderTranslatorConfig
}

func (t *orderTranslator) PrepareCreate(ctx context.Context, req core.OrderRequest) (sharedrouting.DispatchSpec, error) {
	native, instrument, err := t.resolve(ctx, req.Symbol)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	params := map[string]string{
		"symbol": native,
	}
	if side, err := sharedrouting.OrderSideString(req.Side); err == nil {
		params["side"] = side
	} else {
		return sharedrouting.DispatchSpec{}, err
	}
	if orderType, err := sharedrouting.OrderTypeString(req.Type); err == nil {
		params["type"] = orderType
	} else {
		return sharedrouting.DispatchSpec{}, err
	}
	if tif := t.timeInForce(req.TimeInForce); tif != "" {
		params["timeInForce"] = tif
	}
	if req.Quantity != nil {
		params["quantity"] = numeric.Format(req.Quantity, instrument.QtyScale)
	}
	if req.Price != nil {
		params["price"] = numeric.Format(req.Price, instrument.PriceScale)
	}
	if req.ClientID != "" {
		params["newClientOrderId"] = req.ClientID
	}
	if t.cfg.SupportReduceOnly && req.ReduceOnly {
		params["reduceOnly"] = "true"
	}
	return sharedrouting.DispatchSpec{
		Message: sharedrouting.RESTMessage{API: t.cfg.API, Method: http.MethodPost, Path: t.cfg.OrderPath, Query: params, Signed: true},
		Into:    &orderEnvelope{},
		Decode:  t.decodeOrder(req.Symbol),
		OnError: t.annotateActionError(req.Symbol, sharedrouting.ActionPlace),
	}, nil
}

func (t *orderTranslator) PrepareAmend(ctx context.Context, req sharedrouting.OrderAmendRequest) (sharedrouting.DispatchSpec, error) {
	native, instrument, err := t.resolve(ctx, req.Symbol)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	params, err := t.identifierParams(native, req.OrderID, req.ClientOrderID)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	if req.NewQuantity != nil {
		params["quantity"] = numeric.Format(req.NewQuantity, instrument.QtyScale)
	}
	if req.NewPrice != nil {
		params["price"] = numeric.Format(req.NewPrice, instrument.PriceScale)
	}
	if tif := t.timeInForce(req.NewTimeInForce); tif != "" {
		params["timeInForce"] = tif
	}
	return sharedrouting.DispatchSpec{
		Message: sharedrouting.RESTMessage{API: t.cfg.API, Method: http.MethodPut, Path: t.cfg.OrderPath, Query: params, Signed: true},
		Into:    &orderEnvelope{},
		Decode:  t.decodeOrder(req.Symbol),
		OnError: t.annotateActionError(req.Symbol, sharedrouting.ActionAmend),
	}, nil
}

func (t *orderTranslator) PrepareGet(ctx context.Context, req sharedrouting.OrderQueryRequest) (sharedrouting.DispatchSpec, error) {
	native, _, err := t.resolve(ctx, req.Symbol)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	params, err := t.identifierParams(native, req.OrderID, req.ClientOrderID)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	return sharedrouting.DispatchSpec{
		Message: sharedrouting.RESTMessage{API: t.cfg.API, Method: http.MethodGet, Path: t.cfg.OrderPath, Query: params, Signed: true},
		Into:    &orderEnvelope{},
		Decode:  t.decodeOrder(req.Symbol),
		OnError: t.annotateActionError(req.Symbol, sharedrouting.ActionGet),
	}, nil
}

func (t *orderTranslator) PrepareCancel(ctx context.Context, req sharedrouting.OrderCancelRequest) (sharedrouting.DispatchSpec, error) {
	native, _, err := t.resolve(ctx, req.Symbol)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	params, err := t.identifierParams(native, req.OrderID, req.ClientOrderID)
	if err != nil {
		return sharedrouting.DispatchSpec{}, err
	}
	return sharedrouting.DispatchSpec{
		Message: sharedrouting.RESTMessage{API: t.cfg.API, Method: http.MethodDelete, Path: t.cfg.OrderPath, Query: params, Signed: true},
		OnError: t.annotateActionError(req.Symbol, sharedrouting.ActionCancel),
	}, nil
}

func (t *orderTranslator) resolve(ctx context.Context, symbol string) (string, core.Instrument, error) {
	native, err := t.cfg.ResolveNative(ctx, symbol)
	if err != nil {
		return "", core.Instrument{}, t.annotateSymbolError(symbol, err)
	}
	inst, err := t.cfg.LookupInstrument(ctx, symbol)
	if err != nil {
		return "", core.Instrument{}, t.annotateSymbolError(symbol, err)
	}
	return native, inst, nil
}

func (t *orderTranslator) identifierParams(native, id, client string) (map[string]string, error) {
	params := map[string]string{"symbol": native}
	if id != "" {
		params["orderId"] = id
	}
	if client != "" {
		params["origClientOrderId"] = client
	}
	if id == "" && client == "" {
		return nil, internal.Invalid("order identifier required")
	}
	return params, nil
}

func (t *orderTranslator) decodeOrder(symbol string) func(any) (core.Order, error) {
	return func(out any) (core.Order, error) {
		env, ok := out.(*orderEnvelope)
		if !ok || env == nil {
			return core.Order{}, fmt.Errorf("binance: unexpected decode target %T", out)
		}
		orderID := env.OrderID.String()
		order := core.Order{
			ID:     orderID,
			Symbol: symbol,
			Status: internal.MapOrderStatus(env.Status),
		}
		if qty, ok := numeric.Parse(env.ExecutedQty); ok {
			order.FilledQty = qty
		}
		if price, ok := numeric.Parse(env.AvgPrice); ok {
			order.AvgPrice = price
		}
		return order, nil
	}
}

func (t *orderTranslator) timeInForce(tif core.TimeInForce) string {
	if t.cfg.TimeInForce != nil {
		return t.cfg.TimeInForce(tif)
	}
	return internal.MapTimeInForce(tif)
}

func (t *orderTranslator) annotateSymbolError(symbol string, err error) error {
	var e *errs.E
	if errors.As(err, &e) {
		if e.Canonical == "" || e.Canonical == errs.CanonicalUnknown {
			errs.WithCanonicalCode(errs.CanonicalInvalidSymbol)(e)
		}
		errs.WithVenueMetadata(map[string]string{
			"symbol": symbol,
			"market": string(t.cfg.Market),
			"api":    t.cfg.API,
		})(e)
		return e
	}
	return err
}

func (t *orderTranslator) annotateActionError(symbol string, action sharedrouting.OrderAction) func(error) error {
	return func(err error) error {
		var e *errs.E
		if errors.As(err, &e) {
			errs.WithVenueMetadata(map[string]string{
				"symbol": symbol,
				"market": string(t.cfg.Market),
				"api":    t.cfg.API,
				"action": string(action),
			})(e)
			return e
		}
		return err
	}
}

type orderEnvelope struct {
	OrderID     json.Number `json:"orderId"`
	Status      string      `json:"status"`
	ExecutedQty string      `json:"executedQty"`
	AvgPrice    string      `json:"avgPrice"`
}
