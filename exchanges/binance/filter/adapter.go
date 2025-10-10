package filter

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/bridge"
	"github.com/coachpo/meltica/pipeline"
)

type bookSubscriber interface {
	BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

type publicSubscriber interface {
	SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error)
}

type privateSubscriber interface {
	SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error)
}

// Adapter implements filter.Adapter for Binance.
type Adapter struct {
	books      bookSubscriber
	ws         publicSubscriber
	privateWS  privateSubscriber
	restRouter bridge.Dispatcher
	sessionMgr *SessionManager
	hooks      pipeline.AdapterHooks
}

// NewAdapter constructs a Binance filter adapter.
func NewAdapter(books bookSubscriber, ws publicSubscriber) (*Adapter, error) {
	if books == nil && ws == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}
	return &Adapter{
		books: books,
		ws:    ws,
	}, nil
}

// NewAdapterWithPrivate constructs a Binance filter adapter with private capabilities.
func NewAdapterWithPrivate(books bookSubscriber, ws publicSubscriber, privateWS privateSubscriber, restRouter bridge.Dispatcher) (*Adapter, error) {
	if books == nil && ws == nil && privateWS == nil && restRouter == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}
	return &Adapter{
		books:      books,
		ws:         ws,
		privateWS:  privateWS,
		restRouter: restRouter,
	}, nil
}

// NewAdapterWithREST constructs a Binance filter adapter with REST capabilities using a bridge.
func NewAdapterWithREST(books bookSubscriber, ws publicSubscriber, privateWS privateSubscriber, restRouter bridge.Dispatcher) (*Adapter, error) {
	if books == nil && ws == nil && privateWS == nil && restRouter == nil {
		return nil, fmt.Errorf("binance filter: no feed providers available")
	}
	// Create session manager for private streams
	var sessionMgr *SessionManager
	if privateWS != nil {
		sessionMgr = NewSessionManager(restRouter, DefaultSessionConfig())
	}

	return &Adapter{
		books:      books,
		ws:         ws,
		privateWS:  privateWS,
		restRouter: restRouter,
		sessionMgr: sessionMgr,
	}, nil
}

// Capabilities declares supported feeds.
func (a *Adapter) Capabilities() pipeline.Capabilities {
	return pipeline.Capabilities{
		Books:          a.books != nil,
		Trades:         a.ws != nil,
		Tickers:        a.ws != nil,
		PrivateStreams: a.privateWS != nil,
		RESTEndpoints:  a.restRouter != nil,
	}
}

// SetAdapterHooks configures observability callbacks for adapter activity.
func (a *Adapter) SetAdapterHooks(h pipeline.AdapterHooks) {
	a.hooks = h
}

func (a *Adapter) emitSubscribe(ctx context.Context, feed, symbol string) {
	if a.hooks.OnSubscribe == nil {
		return
	}
	a.hooks.OnSubscribe(ctx, pipeline.AdapterEvent{
		Exchange: a.ExchangeName(),
		Feed:     feed,
		Symbol:   symbol,
		Metadata: map[string]string{"adapter": "binance"},
	})
}

func (a *Adapter) emitError(ctx context.Context, feed, symbol string, err error) {
	if err == nil || a.hooks.OnError == nil {
		return
	}
	a.hooks.OnError(ctx, pipeline.AdapterEvent{
		Exchange: a.ExchangeName(),
		Feed:     feed,
		Symbol:   symbol,
		Metadata: map[string]string{"adapter": "binance"},
		Err:      err,
	})
}

func (a *Adapter) ExchangeName() core.ExchangeName {
	return core.ExchangeName("binance")
}

// BookSources subscribes to book feeds for the requested symbols.
func (a *Adapter) BookSources(ctx context.Context, symbols []string) ([]pipeline.BookSource, error) {
	if a.books == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]pipeline.BookSource, 0, len(symbols))
	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		events, errs, err := a.books.BookSnapshots(ctx, symbol)
		if err != nil {
			a.emitError(ctx, "books", symbol, err)
			return nil, err
		}
		a.emitSubscribe(ctx, "books", symbol)
		sources = append(sources, pipeline.BookSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}
	return sources, nil
}

// TradeSources subscribes to trade streams for each symbol and forwards canonical events.
func (a *Adapter) TradeSources(ctx context.Context, symbols []string) ([]pipeline.TradeSource, error) {
	if a.ws == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]pipeline.TradeSource, 0, len(symbols))

	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		topic := core.MustCanonicalTopic(core.TopicTrade, symbol)
		sub, err := a.ws.SubscribePublic(ctx, topic)
		if err != nil {
			a.emitError(ctx, "trades", symbol, err)
			return nil, fmt.Errorf("trade subscription %s: %w", symbol, err)
		}
		events := make(chan corestreams.TradeEvent, 32)
		errs := make(chan error, 1)
		a.emitSubscribe(ctx, "trades", symbol)

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
						a.emitError(ctx, "trades", sym, err)
						select {
						case errs <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(symbol, sub)

		sources = append(sources, pipeline.TradeSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}

	return sources, nil
}

// TickerSources subscribes to ticker streams for each symbol and forwards canonical events.
func (a *Adapter) TickerSources(ctx context.Context, symbols []string) ([]pipeline.TickerSource, error) {
	if a.ws == nil {
		return nil, nil
	}
	symbols = uniqueSymbols(symbols)
	sources := make([]pipeline.TickerSource, 0, len(symbols))

	for _, symbol := range symbols {
		if symbol == "" {
			continue
		}
		topic := core.MustCanonicalTopic(core.TopicTicker, symbol)
		sub, err := a.ws.SubscribePublic(ctx, topic)
		if err != nil {
			a.emitError(ctx, "tickers", symbol, err)
			return nil, fmt.Errorf("ticker subscription %s: %w", symbol, err)
		}
		events := make(chan corestreams.TickerEvent, 32)
		errs := make(chan error, 1)
		a.emitSubscribe(ctx, "tickers", symbol)

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
						a.emitError(ctx, "tickers", sym, err)
						select {
						case errs <- err:
						case <-ctx.Done():
							return
						}
					}
				}
			}
		}(symbol, sub)

		sources = append(sources, pipeline.TickerSource{
			Symbol: symbol,
			Events: events,
			Errors: errs,
		})
	}

	return sources, nil
}

// PrivateSources subscribes to private account and order streams.
func (a *Adapter) PrivateSources(ctx context.Context, auth *pipeline.AuthContext) ([]pipeline.PrivateSource, error) {
	if a.privateWS == nil {
		return nil, nil
	}

	sources := make([]pipeline.PrivateSource, 0, 2) // account + orders

	// Subscribe to account updates
	accountEvents := make(chan pipeline.Event, 32)
	accountErrors := make(chan error, 1)
	a.emitSubscribe(ctx, "private_account", "")
	go a.forwardPrivateEvents(ctx, "account", accountEvents, accountErrors)

	sources = append(sources, pipeline.PrivateSource{
		Events: accountEvents,
		Errors: accountErrors,
	})

	// Subscribe to order updates
	orderEvents := make(chan pipeline.Event, 32)
	orderErrors := make(chan error, 1)
	a.emitSubscribe(ctx, "private_orders", "")
	go a.forwardPrivateEvents(ctx, "orders", orderEvents, orderErrors)

	sources = append(sources, pipeline.PrivateSource{
		Events: orderEvents,
		Errors: orderErrors,
	})

	return sources, nil
}

// ExecuteREST executes REST API calls through the filter pipeline.
func (a *Adapter) ExecuteREST(ctx context.Context, req pipeline.InteractionRequest) (<-chan pipeline.Event, <-chan error, error) {
	if a.restRouter == nil {
		a.emitError(ctx, "rest", req.Symbol, fmt.Errorf("REST router not available"))
		return nil, nil, fmt.Errorf("REST router not available")
	}

	events := make(chan pipeline.Event, 1)
	errors := make(chan error, 1)
	a.emitSubscribe(ctx, "rest", req.Symbol)

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
		event := a.createResponseEvent(req, result, err)

		if err != nil {
			a.emitError(ctx, "rest", req.Symbol, err)
			select {
			case errors <- err:
			case <-ctx.Done():
			}
			return
		}

		select {
		case events <- event:
		case <-ctx.Done():
		}
	}()

	return events, errors, nil
}

// executeWithRetry executes a REST request with retry logic
func (a *Adapter) executeWithRetry(ctx context.Context, req pipeline.InteractionRequest, result interface{}) error {
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
func (a *Adapter) isRetryableError(err error, policy *pipeline.RetryPolicy) bool {
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

// createResponseEvent creates a pipeline event from REST response
func (a *Adapter) createResponseEvent(req pipeline.InteractionRequest, result interface{}, err error) pipeline.Event {
	payload := &pipeline.RestResponse{
		RequestID:  req.CorrelationID,
		Method:     req.Method,
		Path:       req.Path,
		StatusCode: 200,
		Body:       result,
		Error:      nil,
	}
	if err != nil {
		payload.StatusCode = 500
		payload.Body = nil
		payload.Error = err
	}
	return pipeline.Event{
		Transport:     pipeline.TransportREST,
		Symbol:        req.Symbol,
		At:            time.Now(),
		CorrelationID: req.CorrelationID,
		Payload:       pipeline.RestResponsePayload{Response: payload},
	}
}

// InitPrivateSession initializes private session with authentication.
func (a *Adapter) InitPrivateSession(ctx context.Context, auth *pipeline.AuthContext) error {
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

func (a *Adapter) forwardPrivateEvents(ctx context.Context, streamType string, events chan<- pipeline.Event, errors chan<- error) {
	defer close(events)
	defer close(errors)

	feedName := "private"
	if streamType != "" {
		feedName = "private_" + streamType
	}

	sub, err := a.privateWS.SubscribePrivate(ctx)
	if err != nil {
		wrapped := fmt.Errorf("subscribe private %s: %w", streamType, err)
		a.emitError(ctx, feedName, "", wrapped)
		select {
		case errors <- wrapped:
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

			event, convErr := convertPrivateMessage(msg)
			if convErr != nil {
				a.emitError(ctx, feedName, "", convErr)
				select {
				case errors <- convErr:
				case <-ctx.Done():
					return
				}
				select {
				case events <- buildPrivateErrorEvent(convErr, msg.Raw):
				case <-ctx.Done():
					return
				}
				continue
			}

			if event.Payload == nil {
				continue
			}

			if matchesPrivateStream(streamType, event.Payload) {
				select {
				case events <- event:
				case <-ctx.Done():
					return
				}
			}

		case err, ok := <-sub.Err():
			if !ok {
				return
			}
			if err != nil {
				a.emitError(ctx, feedName, "", err)
				select {
				case errors <- err:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func matchesPrivateStream(streamType string, payload pipeline.Payload) bool {
	switch streamType {
	case "account":
		_, ok := payload.(pipeline.AccountPayload)
		return ok
	case "orders":
		_, ok := payload.(pipeline.OrderPayload)
		return ok
	default:
		return true
	}
}

func convertPrivateMessage(msg core.Message) (pipeline.Event, error) {
	if msg.Parsed == nil {
		return pipeline.Event{}, fmt.Errorf("binance private: missing parsed payload")
	}

	event := pipeline.Event{
		Transport: pipeline.TransportPrivateWS,
		At:        msg.At,
	}

	switch payload := msg.Parsed.(type) {
	case *corestreams.OrderEvent:
		if payload == nil {
			return pipeline.Event{}, fmt.Errorf("binance private: order payload is nil")
		}
		order := &pipeline.OrderEvent{
			Symbol:      payload.Symbol,
			OrderID:     payload.OrderID,
			Side:        string(payload.Side),
			Price:       cloneRat(payload.Price),
			Quantity:    cloneRat(payload.Quantity),
			Status:      string(payload.Status),
			Type:        string(payload.Type),
			TimeInForce: string(payload.TimeInForce),
		}
		if order.Quantity == nil {
			order.Quantity = cloneRat(payload.FilledQty)
		}
		if order.Price == nil {
			order.Price = cloneRat(payload.AvgPrice)
		}
		event.Symbol = payload.Symbol
		event.Payload = pipeline.OrderPayload{Order: order}
		return event, nil

	case *corestreams.BalanceEvent:
		if payload == nil || len(payload.Balances) == 0 {
			return pipeline.Event{}, fmt.Errorf("binance private: balance payload empty")
		}
		balance := payload.Balances[0]
		total := cloneRat(balance.Total)
		available := cloneRat(balance.Available)
		locked := cloneRat(balance.Locked)
		if total == nil && available != nil {
			total = cloneRat(available)
		}
		if locked == nil && total != nil && available != nil {
			locked = new(big.Rat).Sub(cloneRat(total), available)
		}
		event.Symbol = balance.Asset
		event.Payload = pipeline.AccountPayload{Account: &pipeline.AccountEvent{
			Symbol:    balance.Asset,
			Balance:   total,
			Available: available,
			Locked:    locked,
		}}
		return event, nil
	default:
		event.Symbol = msg.Topic
		event.Payload = pipeline.UnknownPayload{Detail: payload}
		return event, nil
	}
}

func buildPrivateErrorEvent(err error, raw []byte) pipeline.Event {
	metadata := map[string]any{"error": err}
	if len(raw) > 0 {
		metadata["raw"] = string(raw)
	}
	return pipeline.Event{
		Transport: pipeline.TransportPrivateWS,
		At:        time.Now(),
		Payload:   pipeline.UnknownPayload{Detail: metadata},
		Metadata:  metadata,
	}
}

func cloneRat(src *big.Rat) *big.Rat {
	if src == nil {
		return nil
	}
	return new(big.Rat).Set(src)
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
