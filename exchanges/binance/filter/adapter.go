package filter

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/shared/routing"
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
	sessionMgr *SessionManager
	parser     *PrivateMessageParser
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

// NewAdapterWithREST constructs a Binance filter adapter with REST capabilities using a bridge.
func NewAdapterWithREST(orderBooks orderBookSubscriber, ws publicSubscriber, privateWS privateSubscriber, restRouter routing.RESTDispatcher) (*Adapter, error) {
	if orderBooks == nil && ws == nil && privateWS == nil && restRouter == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}

	// Wrap the router with the Level-4 bridge
	bridge := NewRESTBridge(restRouter)

	// Create session manager for private streams
	var sessionMgr *SessionManager
	var parser *PrivateMessageParser
	if privateWS != nil {
		sessionMgr = NewSessionManager(restRouter, DefaultSessionConfig())
		parser = NewPrivateMessageParser()
	}

	return &Adapter{
		orderBooks: orderBooks,
		ws:         ws,
		privateWS:  privateWS,
		restRouter: bridge,
		sessionMgr: sessionMgr,
		parser:     parser,
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

		// Apply timeout if specified
		if req.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, time.Duration(req.Timeout))
			defer cancel()
		}

		// Execute REST call with retries if specified
		var result interface{}
		var err error

		if req.RetryPolicy != nil && req.RetryPolicy.MaxAttempts > 1 {
			err = a.executeWithRetry(ctx, req, &result)
		} else {
			err = a.restRouter.Dispatch(ctx, req, &result)
		}

		// Parse response and create appropriate envelope
		envelope := a.createResponseEnvelope(req, result, err)

		if err != nil {
			select {
			case errors <- err:
			case <-ctx.Done():
			}
			return
		}

		select {
		case events <- envelope:
		case <-ctx.Done():
		}
	}()

	return events, errors, nil
}

// executeWithRetry executes a REST request with retry logic
func (a *Adapter) executeWithRetry(ctx context.Context, req filter.InteractionRequest, result interface{}) error {
	policy := req.RetryPolicy
	baseDelay := time.Duration(policy.BaseDelay)
	maxDelay := time.Duration(policy.MaxDelay)

	var lastErr error

	for attempt := 0; attempt < policy.MaxAttempts; attempt++ {
		if attempt > 0 {
			// Calculate backoff delay
			delay := baseDelay * time.Duration(policy.BackoffMultiplier*float64(attempt))
			if delay > maxDelay {
				delay = maxDelay
			}

			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return ctx.Err()
			}
		}

		err := a.restRouter.Dispatch(ctx, req, result)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !a.isRetryableError(err, policy) {
			break
		}
	}

	return lastErr
}

// isRetryableError checks if an error should be retried based on the retry policy
func (a *Adapter) isRetryableError(err error, policy *filter.RetryPolicy) bool {
	// Check if error message matches retryable patterns
	errStr := err.Error()
	for _, pattern := range policy.RetryableErrors {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// TODO: Check status codes if we have access to HTTP response
	// For now, we'll retry on network errors and rate limiting
	return strings.Contains(errStr, "timeout") ||
		   strings.Contains(errStr, "network") ||
		   strings.Contains(errStr, "rate limit") ||
		   strings.Contains(errStr, "429")
}

// createResponseEnvelope creates an event envelope from REST response
func (a *Adapter) createResponseEnvelope(req filter.InteractionRequest, result interface{}, err error) filter.EventEnvelope {
	envelope := filter.EventEnvelope{
		Kind:           filter.EventKindRestResponse,
		Channel:        filter.ChannelREST,
		Symbol:         req.Symbol,
		Timestamp:      time.Now(),
		CorrelationID:  req.CorrelationID,
	}

	if err != nil {
		envelope.RestResponse = &filter.RestResponse{
			RequestID:  req.CorrelationID,
			Method:     req.Method,
			Path:       req.Path,
			StatusCode: 500, // Default to 500 for errors
			Body:       nil,
			Error:      err,
		}
	} else {
		envelope.RestResponse = &filter.RestResponse{
			RequestID:  req.CorrelationID,
			Method:     req.Method,
			Path:       req.Path,
			StatusCode: 200,
			Body:       result,
			Error:      nil,
		}
	}

	return envelope
}

// InitPrivateSession initializes private session with authentication.
func (a *Adapter) InitPrivateSession(ctx context.Context, auth *filter.AuthContext) error {
	if a.sessionMgr == nil {
		return fmt.Errorf("session manager not available")
	}

	return a.sessionMgr.InitPrivateSession(ctx, auth)
}

// Close releases exchange resources.
func (a *Adapter) Close() {
	if a.sessionMgr != nil {
		a.sessionMgr.Close()
	}
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
		case msg, ok := <-sub.C():
			if !ok {
				return
			}

			// Convert core.Message to streams.RoutedMessage
			routedMsg := corestreams.RoutedMessage{
				Topic: msg.Topic,
				Raw:   msg.Raw,
				At:    msg.At,
				Route: "private", // Default route for private messages
				Parsed: msg.Parsed,
			}

			// Parse private message using the parser
			envelope, err := a.parser.ParseMessage(routedMsg)
			if err != nil {
				// Create error envelope for parsing failures
				errorEnvelope := a.parser.ParseError(err, msg.Raw)
				select {
				case errors <- err:
				case <-ctx.Done():
					return
				}
				select {
				case events <- errorEnvelope:
				case <-ctx.Done():
					return
				}
				continue
			}

			// Filter by stream type if needed
			if (streamType == "account" && envelope.Kind == filter.EventKindAccount) ||
			   (streamType == "orders" && envelope.Kind == filter.EventKindOrder) {
				select {
				case events <- envelope:
				case <-ctx.Done():
					return
				}
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
