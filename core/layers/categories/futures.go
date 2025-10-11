package categories

import (
	"context"

	"github.com/coachpo/meltica/core/layers"
)

// FuturesConnection augments a connection with futures-specific metadata.
type FuturesConnection interface {
	layers.Connection

	// GetFuturesEndpoint returns the REST or WebSocket endpoint for futures.
	GetFuturesEndpoint() string

	// GetContractDetails provides the contract metadata for the connection.
	GetContractDetails() layers.ContractInfo
}

// FuturesRouting adds futures market routing capabilities.
type FuturesRouting interface {
	layers.Routing

	// SubscribeFundingRate subscribes to funding-rate streams for the symbol.
	SubscribeFundingRate(ctx context.Context, symbol string) error
}
