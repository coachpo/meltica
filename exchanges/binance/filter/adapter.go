package filter

import (
	"context"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/marketdata/filter"
)

type orderBookSubscriber interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

type publicSubscriber interface {
	SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error)
}

// Adapter implements filter.Adapter for Binance.
type Adapter struct {
	orderBooks orderBookSubscriber
	ws         publicSubscriber
}

// NewAdapter constructs a Binance filter adapter.
func NewAdapter(orderBooks orderBookSubscriber, ws publicSubscriber) (*Adapter, error) {
	if orderBooks == nil && ws == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}
	return &Adapter{
		orderBooks: orderBooks,
		ws:         ws,
	}, nil
}

// Capabilities declares supported feeds.
func (a *Adapter) Capabilities() filter.Capabilities {
	return filter.Capabilities{
		Books:   a.orderBooks != nil,
		Trades:  a.ws != nil,
		Tickers: a.ws != nil,
	}
}

// BookSources subscribes to order book feeds for the requested symbols.
func (a *Adapter) BookSources(ctx context.Context, symbols []string) ([]filter.BookSource, error) {
	if a.orderBooks == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]filter.BookSource, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		events, errs, err := a.orderBooks.OrderBookSnapshots(ctx, symbol)
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

// TradeSources subscribes to trade streams for each symbol and forwards canonical events.
func (a *Adapter) TradeSources(ctx context.Context, symbols []string) ([]filter.TradeSource, error) {
	if a.ws == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]filter.TradeSource, 0, len(symbols))

	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		topic := core.MustCanonicalTopic(core.TopicTrade, symbol)
		sub, err := a.ws.SubscribePublic(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("trade subscription %s: %w", symbol, err)
		}
		events := make(chan corestreams.TradeEvent, 32)
		errs := make(chan error, 1)

		go func(sym string, subscription core.Subscription) {
			defer close(events)
			defer close(errs)
			defer subscription.Close()

			eventCh := subscription.C()
			errCh := subscription.Err()
			for eventCh != nil || errCh != nil {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-eventCh:
					if !ok {
						eventCh = nil
						continue
					}
					trade, ok := msg.Parsed.(*corestreams.TradeEvent)
					if !ok || trade == nil {
						continue
					}
					tradeCopy := *trade
					tradeCopy.Symbol = sym
					select {
					case events <- tradeCopy:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
						continue
					}
					if err != nil {
						select {
						case errs <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(symbol, sub)

		sources = append(sources, filter.TradeSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}

	return sources, nil
}

// TickerSources subscribes to ticker streams for each symbol and forwards canonical events.
func (a *Adapter) TickerSources(ctx context.Context, symbols []string) ([]filter.TickerSource, error) {
	if a.ws == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]filter.TickerSource, 0, len(symbols))

	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		topic := core.MustCanonicalTopic(core.TopicTicker, symbol)
		sub, err := a.ws.SubscribePublic(ctx, topic)
		if err != nil {
			return nil, fmt.Errorf("ticker subscription %s: %w", symbol, err)
		}
		events := make(chan corestreams.TickerEvent, 32)
		errs := make(chan error, 1)

		go func(sym string, subscription core.Subscription) {
			defer close(events)
			defer close(errs)
			defer subscription.Close()

			eventCh := subscription.C()
			errCh := subscription.Err()
			for eventCh != nil || errCh != nil {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-eventCh:
					if !ok {
						eventCh = nil
						continue
					}
					ticker, ok := msg.Parsed.(*corestreams.TickerEvent)
					if !ok || ticker == nil {
						continue
					}
					tickerCopy := *ticker
					tickerCopy.Symbol = sym
					select {
					case events <- tickerCopy:
					case <-ctx.Done():
						return
					}
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
						continue
					}
					if err != nil {
						select {
						case errs <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(symbol, sub)

		sources = append(sources, filter.TickerSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}

	return sources, nil
}

// Close releases exchange resources.
func (a *Adapter) Close() {
	// Adapter relies on exchange lifecycle managed elsewhere.
}

func uniqueSymbols(symbols []string) []string {
	seen := make(map[string]struct{}, len(symbols))
	out := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		symbol = normalize(symbol)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}

func normalize(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
