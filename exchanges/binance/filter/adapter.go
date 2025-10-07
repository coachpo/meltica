package filter

import (
	"context"
	"fmt"
	"strings"
	"time"

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

type privateSubscriber interface {
	SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error)
}

type restExecutor interface {
	Dispatch(ctx context.Context, msg interface{}, result interface{}) error
}

// Adapter implements filter.Adapter for Binance.
type Adapter struct {
	orderBooks orderBookSubscriber
	ws         publicSubscriber
	privateWS  privateSubscriber
	restRouter restExecutor
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

// NewAdapterWithPrivate constructs a Binance filter adapter with private capabilities.
func NewAdapterWithPrivate(orderBooks orderBookSubscriber, ws publicSubscriber, privateWS privateSubscriber, restRouter restExecutor) (*Adapter, error) {
	if orderBooks == nil && ws == nil && privateWS == nil && restRouter == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}
	return &Adapter{
		orderBooks: orderBooks,
		ws:         ws,
		privateWS:  privateWS,
		restRouter: restRouter,
	}, nil
}

// Capabilities declares supported feeds.
func (a *Adapter) Capabilities() filter.Capabilities {
	return filter.Capabilities{
		Books:         a.orderBooks != nil,
		Trades:        a.ws != nil,
		Tickers:       a.ws != nil,
		PrivateStreams: a.privateWS != nil,
		RESTEndpoints:  a.restRouter != nil,
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

// PrivateSources subscribes to private account and order streams.
func (a *Adapter) PrivateSources(ctx context.Context, auth *filter.AuthContext) ([]filter.PrivateSource, error) {
	if a.privateWS == nil {
		return nil, nil
	}

	sources := make([]filter.PrivateSource, 0, 2) // account + orders

	// Subscribe to account updates
	accountEvents := make(chan filter.EventEnvelope, 32)
	accountErrors := make(chan error, 1)

	go a.forwardPrivateEvents(ctx, "account", accountEvents, accountErrors)

	sources = append(sources, filter.PrivateSource{
		Kind:   filter.EventKindAccount,
		Events: accountEvents,
		Errors: accountErrors,
	})

	// Subscribe to order updates
	orderEvents := make(chan filter.EventEnvelope, 32)
	orderErrors := make(chan error, 1)

	go a.forwardPrivateEvents(ctx, "orders", orderEvents, orderErrors)

	sources = append(sources, filter.PrivateSource{
		Kind:   filter.EventKindOrder,
		Events: orderEvents,
		Errors: orderErrors,
	})

	return sources, nil
}

// ExecuteREST executes REST API calls through the filter pipeline.
func (a *Adapter) ExecuteREST(ctx context.Context, req filter.InteractionRequest) (<-chan filter.EventEnvelope, <-chan error, error) {
	if a.restRouter == nil {
		return nil, nil, fmt.Errorf("REST router not available")
	}

	events := make(chan filter.EventEnvelope, 1)
	errors := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errors)

		// Execute REST call
		var result interface{}
		err := a.restRouter.Dispatch(ctx, req, &result)

		if err != nil {
			select {
			case errors <- err:
			case <-ctx.Done():
			}
			return
		}

		// Create response envelope
		envelope := filter.EventEnvelope{
			Kind:           filter.EventKindRestResponse,
			Channel:        filter.ChannelREST,
			Symbol:         req.Symbol,
			Timestamp:      time.Now(),
			CorrelationID:  req.CorrelationID,
			RestResponse: &filter.RestResponse{
				RequestID:  req.CorrelationID,
				Method:     req.Method,
				Path:       req.Path,
				StatusCode: 200,
				Body:       result,
			},
		}

		select {
		case events <- envelope:
		case <-ctx.Done():
		}
	}()

	return events, errors, nil
}

// InitPrivateSession initializes private session with authentication.
func (a *Adapter) InitPrivateSession(ctx context.Context, auth *filter.AuthContext) error {
	// Binance private streams are managed by the router, no explicit initialization needed
	return nil
}

// Close releases exchange resources.
func (a *Adapter) Close() {
	// Adapter relies on exchange lifecycle managed elsewhere.
}

func (a *Adapter) forwardPrivateEvents(ctx context.Context, streamType string, events chan<- filter.EventEnvelope, errors chan<- error) {
	defer close(events)
	defer close(errors)

	sub, err := a.privateWS.SubscribePrivate(ctx)
	if err != nil {
		select {
		case errors <- fmt.Errorf("subscribe private %s: %w", streamType, err):
		case <-ctx.Done():
		}
		return
	}
	defer sub.Close()

	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-sub.C():
			if !ok {
				return
			}

			// Parse private message and create envelope
			envelope := filter.EventEnvelope{
				Channel:   filter.ChannelPrivateWS,
				Timestamp: time.Now(),
			}

			// TODO: Parse Binance private stream messages and populate appropriate event fields
			// For now, we'll create a placeholder envelope
			switch streamType {
			case "account":
				envelope.Kind = filter.EventKindAccount
				// envelope.AccountEvent = parseAccountEvent(msg)
			case "orders":
				envelope.Kind = filter.EventKindOrder
				// envelope.OrderEvent = parseOrderEvent(msg)
			}

			select {
			case events <- envelope:
			case <-ctx.Done():
				return
			}

		case err, ok := <-sub.Err():
			if !ok {
				return
			}
			if err != nil {
				select {
				case errors <- err:
				case <-ctx.Done():
					return
				}
			}
		}
	}
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
