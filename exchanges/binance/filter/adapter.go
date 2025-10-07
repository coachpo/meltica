package filter

import (
	"context"
	"fmt"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/marketdata/filter"
)

type orderBookSubscriber interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

// Adapter implements filter.Adapter for Binance.
type Adapter struct {
	exchange orderBookSubscriber
}

// NewAdapter constructs a Binance filter adapter.
func NewAdapter(exchange orderBookSubscriber) (*Adapter, error) {
	if exchange == nil {
		return nil, fmt.Errorf("nil exchange provided")
	}
	return &Adapter{exchange: exchange}, nil
}

// Capabilities declares supported feeds.
func (a *Adapter) Capabilities() filter.Capabilities {
	return filter.Capabilities{
		Books: true,
	}
}

// BookSources subscribes to order book feeds for the requested symbols.
func (a *Adapter) BookSources(ctx context.Context, symbols []string) ([]filter.BookSource, error) {
	sources := make([]filter.BookSource, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		events, errs, err := a.exchange.OrderBookSnapshots(ctx, symbol)
		if err != nil {
			return nil, err
		}
		sources = append(sources, filter.BookSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}
	return sources, nil
}

// Close releases exchange resources.
func (a *Adapter) Close() {
	// Adapter currently relies on the exchange lifecycle managed elsewhere.
}
