package bootstrap

import (
	"context"
	"fmt"
	"math/big"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/exchanges/shared/routing"
)

// TradingService exposes canonical trading operations backed by a shared order router.
type TradingService interface {
	Capabilities() capabilities.Set
	Supports(action routing.OrderAction) bool
	Place(ctx context.Context, req core.OrderRequest) (core.Order, error)
	Amend(ctx context.Context, req routing.OrderAmendRequest) (core.Order, error)
	Get(ctx context.Context, req routing.OrderQueryRequest) (core.Order, error)
	Cancel(ctx context.Context, req routing.OrderCancelRequest) error
}

// TradingServiceConfig configures the creation of a canonical trading service.
type TradingServiceConfig struct {
	// Router provides the underlying capability-aware router responsible for dispatching
	// canonical requests to venue-specific transports.
	Router *routing.OrderRouter
	// Capabilities optionally overrides the capability bitset reported by the service. When
	// unset, the capability set advertised by Router is used.
	Capabilities capabilities.Set
}

// NewTradingService constructs a TradingService backed by the provided order router.
func NewTradingService(cfg TradingServiceConfig) (TradingService, error) {
	if cfg.Router == nil {
		return nil, fmt.Errorf("bootstrap: trading service requires a router")
	}
	caps := cfg.Capabilities
	if caps == 0 {
		caps = cfg.Router.Capabilities()
	}
	return &tradingService{router: cfg.Router, caps: caps}, nil
}

type tradingService struct {
	router *routing.OrderRouter
	caps   capabilities.Set
}

func (s *tradingService) Capabilities() capabilities.Set { return s.caps }

func (s *tradingService) Supports(action routing.OrderAction) bool {
	return s.router.Supports(action)
}

func (s *tradingService) Place(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	return s.router.PlaceOrder(ctx, req)
}

func (s *tradingService) Amend(ctx context.Context, req routing.OrderAmendRequest) (core.Order, error) {
	return s.router.AmendOrder(ctx, req)
}

func (s *tradingService) Get(ctx context.Context, req routing.OrderQueryRequest) (core.Order, error) {
	return s.router.GetOrder(ctx, req)
}

func (s *tradingService) Cancel(ctx context.Context, req routing.OrderCancelRequest) error {
	return s.router.CancelOrder(ctx, req)
}

// OrderAmendRequestFromParts constructs a canonical amend request from primitive values,
// applying the provided symbol identifiers and optional rational adjustments.
func OrderAmendRequestFromParts(symbol, orderID, clientOrderID string, quantity, price *big.Rat, tif core.TimeInForce) routing.OrderAmendRequest {
	return routing.OrderAmendRequest{
		Symbol:         symbol,
		OrderID:        orderID,
		ClientOrderID:  clientOrderID,
		NewQuantity:    quantity,
		NewPrice:       price,
		NewTimeInForce: tif,
	}
}
