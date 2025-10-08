package routing

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/errs"
)

// OrderAction enumerates the supported routing operations.
type OrderAction string

const (
	ActionPlace  OrderAction = "place"
	ActionAmend  OrderAction = "amend"
	ActionGet    OrderAction = "get"
	ActionCancel OrderAction = "cancel"
)

// DispatchSpec configures a REST dispatch along with optional decoding behaviour.
type DispatchSpec struct {
	Message RESTMessage
	Into    any
	Decode  func(any) (core.Order, error)
	OnError func(error) error
}

// OrderTranslator builds venue-specific REST requests for canonical trading actions.
type OrderTranslator interface {
	PrepareCreate(context.Context, core.OrderRequest) (DispatchSpec, error)
	PrepareAmend(context.Context, OrderAmendRequest) (DispatchSpec, error)
	PrepareGet(context.Context, OrderQueryRequest) (DispatchSpec, error)
	PrepareCancel(context.Context, OrderCancelRequest) (DispatchSpec, error)
}

// OrderAmendRequest carries canonical amendment parameters.
type OrderAmendRequest struct {
	Symbol         string
	OrderID        string
	ClientOrderID  string
	NewQuantity    *big.Rat
	NewPrice       *big.Rat
	NewTimeInForce core.TimeInForce
}

// OrderQueryRequest defines canonical order lookup parameters.
type OrderQueryRequest struct {
	Symbol        string
	OrderID       string
	ClientOrderID string
}

// OrderCancelRequest defines canonical cancellation parameters.
type OrderCancelRequest struct {
	Symbol        string
	OrderID       string
	ClientOrderID string
}

// OrderRouter coordinates capability checks and dispatcher execution for trading actions.
type OrderRouter struct {
	dispatcher   RESTDispatcher
	translator   OrderTranslator
	capabilities capabilities.Set
	requirements map[OrderAction][]capabilities.Capability
}

// OrderRouterOption customises router construction.
type OrderRouterOption func(*OrderRouter) error

// NewOrderRouter constructs an OrderRouter with the supplied dispatcher and translator.
func NewOrderRouter(dispatcher RESTDispatcher, translator OrderTranslator, opts ...OrderRouterOption) (*OrderRouter, error) {
	if dispatcher == nil {
		return nil, fmt.Errorf("routing: dispatcher must not be nil")
	}
	if translator == nil {
		return nil, fmt.Errorf("routing: translator must not be nil")
	}
	r := &OrderRouter{
		dispatcher:   dispatcher,
		translator:   translator,
		requirements: make(map[OrderAction][]capabilities.Capability),
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(r); err != nil {
			return nil, err
		}
	}
	return r, nil
}

// WithCapabilities sets the capability set advertised by the exchange adapter.
func WithCapabilities(set capabilities.Set) OrderRouterOption {
	return func(r *OrderRouter) error {
		r.capabilities = set
		return nil
	}
}

// WithCapabilityRequirement declares capability requirements for the given action.
func WithCapabilityRequirement(action OrderAction, caps ...capabilities.Capability) OrderRouterOption {
	return func(r *OrderRouter) error {
		if len(caps) == 0 {
			return nil
		}
		reqs := append([]capabilities.Capability(nil), caps...)
		r.requirements[action] = append(r.requirements[action], reqs...)
		return nil
	}
}

// PlaceOrder submits a new canonical order via the underlying dispatcher.
func (r *OrderRouter) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	if err := r.ensureCapabilities(ActionPlace); err != nil {
		return core.Order{}, err
	}
	spec, err := r.translator.PrepareCreate(ctx, req)
	if err != nil {
		return core.Order{}, err
	}
	return r.execute(ctx, spec)
}

// AmendOrder updates an existing order according to the provided parameters.
func (r *OrderRouter) AmendOrder(ctx context.Context, req OrderAmendRequest) (core.Order, error) {
	if err := r.ensureCapabilities(ActionAmend); err != nil {
		return core.Order{}, err
	}
	spec, err := r.translator.PrepareAmend(ctx, req)
	if err != nil {
		return core.Order{}, err
	}
	return r.execute(ctx, spec)
}

// GetOrder retrieves a canonical order snapshot.
func (r *OrderRouter) GetOrder(ctx context.Context, req OrderQueryRequest) (core.Order, error) {
	if err := r.ensureCapabilities(ActionGet); err != nil {
		return core.Order{}, err
	}
	spec, err := r.translator.PrepareGet(ctx, req)
	if err != nil {
		return core.Order{}, err
	}
	return r.execute(ctx, spec)
}

// CancelOrder cancels an existing order if supported by the venue.
func (r *OrderRouter) CancelOrder(ctx context.Context, req OrderCancelRequest) error {
	if err := r.ensureCapabilities(ActionCancel); err != nil {
		return err
	}
	spec, err := r.translator.PrepareCancel(ctx, req)
	if err != nil {
		return err
	}
	_, err = r.execute(ctx, spec)
	return err
}

func (r *OrderRouter) execute(ctx context.Context, spec DispatchSpec) (core.Order, error) {
	if err := r.dispatcher.Dispatch(ctx, spec.Message, spec.Into); err != nil {
		if spec.OnError != nil {
			err = spec.OnError(err)
		}
		return core.Order{}, err
	}
	if spec.Decode == nil {
		return core.Order{}, nil
	}
	return spec.Decode(spec.Into)
}

func (r *OrderRouter) ensureCapabilities(action OrderAction) error {
	if r.Supports(action) {
		return nil
	}
	required := r.requirements[action]
	if len(required) == 0 {
		return nil
	}
	missing := r.capabilities.Missing(required...)
	return capabilityError(action, missing)
}

// Capabilities exposes the configured capability set for the router.
func (r *OrderRouter) Capabilities() capabilities.Set {
	return r.capabilities
}

// Supports reports whether the router satisfies the capability requirements for the action.
func (r *OrderRouter) Supports(action OrderAction) bool {
	required := r.requirements[action]
	if len(required) == 0 {
		return true
	}
	return r.capabilities.All(required...)
}

func capabilityError(action OrderAction, missing []capabilities.Capability) error {
	parts := make([]string, len(missing))
	for i, cap := range missing {
		parts[i] = fmt.Sprintf("0x%X", uint64(cap))
	}
	msg := fmt.Sprintf("missing capabilities for %s: %s", action, strings.Join(parts, ", "))
	return errs.NotSupported(msg)
}
