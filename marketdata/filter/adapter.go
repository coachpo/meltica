package filter

import (
	"context"

	corestreams "github.com/coachpo/meltica/core/streams"
)

// Capabilities describes which feeds and channels an adapter can provide.
type Capabilities struct {
	Books         bool
	Trades        bool
	Tickers       bool
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
	ExecuteREST(ctx context.Context, req InteractionRequest) (<-chan EventEnvelope, <-chan error, error)

	// Lifecycle hooks
	InitPrivateSession(ctx context.Context, auth *AuthContext) error
	Close()
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
	Kind   EventKind
	Events <-chan EventEnvelope
	Errors <-chan error
}
