package filter

import (
	"context"

	corestreams "github.com/coachpo/meltica/core/streams"
)

// Capabilities describes which feeds an adapter can provide.
type Capabilities struct {
	Books   bool
	Trades  bool
	Tickers bool
}

// Adapter exposes exchange-specific feed sourcing to the filter pipeline.
type Adapter interface {
	Capabilities() Capabilities
	BookSources(ctx context.Context, symbols []string) ([]BookSource, error)
	TradeSources(ctx context.Context, symbols []string) ([]TradeSource, error)
	TickerSources(ctx context.Context, symbols []string) ([]TickerSource, error)
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
