package pipeline

import (
	"context"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

// Capabilities describes which feeds and channels an adapter can provide.
type Capabilities struct {
	Books          bool
	Trades         bool
	Tickers        bool
	PrivateStreams bool
	RESTEndpoints  bool
}

// Adapter exposes exchange-specific feed sourcing to the filter pipeline.
type Adapter interface {
	Capabilities() Capabilities
	BookSources(ctx context.Context, symbols []string) ([]BookSource, error)
	TradeSources(ctx context.Context, symbols []string) ([]TradeSource, error)
	TickerSources(ctx context.Context, symbols []string) ([]TickerSource, error)

	// Multi-channel capabilities
	PrivateSources(ctx context.Context, auth *AuthContext) ([]PrivateSource, error)
	ExecuteREST(ctx context.Context, req InteractionRequest) (<-chan Event, <-chan error, error)

	// Lifecycle hooks
	InitPrivateSession(ctx context.Context, auth *AuthContext) error
	Close()

	// ExchangeName returns the canonical identifier for the adapter's venue.
	ExchangeName() core.ExchangeName
}

// BookSource represents a single symbol subscription to book events.
type BookSource struct {
	Symbol string
	Events <-chan corestreams.BookEvent
	Errors <-chan error
}

// TradeSource represents a single symbol subscription to trade events.
type TradeSource struct {
	Symbol string
	Events <-chan corestreams.TradeEvent
	Errors <-chan error
}

// TickerSource represents a single symbol subscription to ticker events.
type TickerSource struct {
	Symbol string
	Events <-chan corestreams.TickerEvent
	Errors <-chan error
}

// PrivateSource represents a subscription to private stream events.
type PrivateSource struct {
	Events <-chan Event
	Errors <-chan error
}
