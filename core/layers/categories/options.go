package categories

import (
	"context"

	"github.com/coachpo/meltica/core/layers"
)

// OptionsConnection extends a connection with options-market helpers.
type OptionsConnection interface {
	layers.Connection

	// GetOptionsEndpoint returns the endpoint used for options market data.
	GetOptionsEndpoint() string

	// GetContractChain returns the tradeable contracts for the underlying.
	GetContractChain(ctx context.Context, underlying string) ([]layers.OptionContract, error)
}

// OptionsRouting enables options-specific subscription flows.
type OptionsRouting interface {
	layers.Routing

	// SubscribeGreeks subscribes to greeks updates for the provided symbol.
	SubscribeGreeks(ctx context.Context, symbol string) error
}
