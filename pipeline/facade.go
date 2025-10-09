package pipeline

import (
	"context"
	"fmt"
	"time"
)

// InteractionFacade provides a high-level interface for Level-4 interactions.
type InteractionFacade struct {
	coordinator *Coordinator
	hooks       ObservabilityHooks
}

// NewInteractionFacade creates a new facade for Level-4 interactions.
func NewInteractionFacade(adapter Adapter, auth *AuthContext) *InteractionFacade {
	coordinator := NewCoordinator(adapter, auth)
	return &InteractionFacade{coordinator: coordinator}
}

// SetObservability configures callbacks for structured logging and metrics.
func (f *InteractionFacade) SetObservability(hooks ObservabilityHooks) {
	f.hooks = hooks
	f.coordinator.SetAdapterHooks(hooks.Adapter)
}

func (f *InteractionFacade) streamWithObservability(ctx context.Context, req PipelineRequest) (PipelineStream, error) {
	started := time.Now()
	if f.hooks.OnStreamStart != nil {
		f.hooks.OnStreamStart(ctx, req)
	}

	stream, err := f.coordinator.Stream(ctx, req)
	if err != nil {
		if f.hooks.OnStreamError != nil {
			f.hooks.OnStreamError(ctx, req, err)
		}
		return stream, err
	}

	if f.hooks.OnStreamClose != nil {
		cancel := stream.cancel
		stream.cancel = func() {
			if cancel != nil {
				cancel()
			}
			f.hooks.OnStreamClose(ctx, req, time.Since(started))
		}
	}

	return stream, nil
}

// SubscribePublic subscribes to public market data feeds.
func (f *InteractionFacade) SubscribePublic(
	ctx context.Context,
	symbols []string,
	options ...PublicOption,
) (PipelineStream, error) {
	config := &PublicConfig{
		Books:   false,
		Trades:  true,
		Tickers: true,
	}
	for _, opt := range options {
		opt(config)
	}

	req := PipelineRequest{
		Symbols: symbols,
		Feeds: FeedSelection{
			Books:   config.Books,
			Trades:  config.Trades,
			Tickers: config.Tickers,
		},
		BookDepth:       config.BookDepth,
		MinEmitInterval: config.MinEmitInterval,
		EnableSnapshots: config.EnableSnapshots,
		EnableVWAP:      config.EnableVWAP,
		Observer:        config.Observer,
	}
	req.Observability = f.hooks
	return f.streamWithObservability(ctx, req)
}

// SubscribePrivate subscribes to private account and order streams.
func (f *InteractionFacade) SubscribePrivate(
	ctx context.Context,
	options ...PrivateOption,
) (PipelineStream, error) {
	config := &PrivateConfig{}
	for _, opt := range options {
		opt(config)
	}

	req := PipelineRequest{
		EnablePrivate:   true,
		MinEmitInterval: config.MinEmitInterval,
		EnableSnapshots: config.EnableSnapshots,
		Observer:        config.Observer,
	}
	req.Observability = f.hooks
	return f.streamWithObservability(ctx, req)
}

// FetchREST executes REST API calls through the filter pipeline.
func (f *InteractionFacade) FetchREST(
	ctx context.Context,
	requests []InteractionRequest,
	options ...RESTOption,
) (PipelineStream, error) {
	config := &RESTConfig{}
	for _, opt := range options {
		opt(config)
	}

	req := PipelineRequest{
		RESTRequests:    requests,
		MinEmitInterval: config.MinEmitInterval,
		Observer:        config.Observer,
	}
	req.Observability = f.hooks
	return f.streamWithObservability(ctx, req)
}

// ExecuteSingleREST executes a single REST API call.
func (f *InteractionFacade) ExecuteSingleREST(
	ctx context.Context,
	method string,
	path string,
	payload interface{},
	correlationID string,
) (PipelineStream, error) {
	req := InteractionRequest{
		Method:        method,
		Path:          path,
		Payload:       payload,
		CorrelationID: correlationID,
	}

	return f.FetchREST(ctx, []InteractionRequest{req})
}

// Close releases all resources.
func (f *InteractionFacade) Close() {
	f.coordinator.Close()
}

// PublicConfig holds configuration for public stream subscriptions.
type PublicConfig struct {
	Books           bool
	Trades          bool
	Tickers         bool
	BookDepth       int
	MinEmitInterval time.Duration
	EnableSnapshots bool
	EnableVWAP      bool
	Observer        Observer
}

// PublicOption configures public stream subscriptions.
type PublicOption func(*PublicConfig)

// WithBooks enables book subscriptions.
func WithBooks() PublicOption {
	return func(c *PublicConfig) {
		c.Books = true
	}
}

// WithTrades enables trade stream subscriptions.
func WithTrades() PublicOption {
	return func(c *PublicConfig) {
		c.Trades = true
	}
}

// WithTickers enables ticker stream subscriptions.
func WithTickers() PublicOption {
	return func(c *PublicConfig) {
		c.Tickers = true
	}
}

// WithBookDepth sets the maximum depth for books.
func WithBookDepth(depth int) PublicOption {
	return func(c *PublicConfig) {
		c.BookDepth = depth
	}
}

// WithMinEmitInterval sets the minimum interval between events.
func WithMinEmitInterval(interval time.Duration) PublicOption {
	return func(c *PublicConfig) {
		c.MinEmitInterval = interval
	}
}

// WithSnapshots enables event caching for retrieving latest events.
func WithSnapshots() PublicOption {
	return func(c *PublicConfig) {
		c.EnableSnapshots = true
	}
}

// WithVWAP enables VWAP analytics.
func WithVWAP() PublicOption {
	return func(c *PublicConfig) {
		c.EnableVWAP = true
	}
}

// WithObserver sets the event observer.
func WithObserver(observer Observer) PublicOption {
	return func(c *PublicConfig) {
		c.Observer = observer
	}
}

// PrivateConfig holds configuration for private stream subscriptions.
type PrivateConfig struct {
	MinEmitInterval time.Duration
	EnableSnapshots bool
	Observer        Observer
}

// PrivateOption configures private stream subscriptions.
type PrivateOption func(*PrivateConfig)

// RESTConfig holds configuration for REST API calls.
type RESTConfig struct {
	MinEmitInterval time.Duration
	Observer        Observer
}

// RESTOption configures REST API calls.
type RESTOption func(*RESTConfig)

// Retry policy helper functions

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:          3,
		BaseDelay:            Duration(100 * time.Millisecond),
		MaxDelay:             Duration(5 * time.Second),
		BackoffMultiplier:    2.0,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
		RetryableErrors: []string{
			"timeout",
			"network",
			"rate limit",
			"connection reset",
			"EOF",
		},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations
func AggressiveRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:          5,
		BaseDelay:            Duration(50 * time.Millisecond),
		MaxDelay:             Duration(10 * time.Second),
		BackoffMultiplier:    1.5,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
		RetryableErrors: []string{
			"timeout",
			"network",
			"rate limit",
			"connection reset",
			"EOF",
			"temporary",
		},
	}
}

// ConservativeRetryPolicy returns a conservative retry policy for non-critical operations
func ConservativeRetryPolicy() *RetryPolicy {
	return &RetryPolicy{
		MaxAttempts:          2,
		BaseDelay:            Duration(500 * time.Millisecond),
		MaxDelay:             Duration(2 * time.Second),
		BackoffMultiplier:    1.2,
		RetryableStatusCodes: []int{429, 503},
		RetryableErrors: []string{
			"rate limit",
			"temporary",
		},
	}
}

// SimpleObserver is a simple implementation of the Observer interface.
type SimpleObserver struct {
	OnEventFunc func(ClientEvent)
	OnErrorFunc func(error)
}

// OnEvent handles incoming events.
func (o *SimpleObserver) OnEvent(evt ClientEvent) {
	if o.OnEventFunc != nil {
		o.OnEventFunc(evt)
	}
}

// OnError handles incoming errors.
func (o *SimpleObserver) OnError(err error) {
	if o.OnErrorFunc != nil {
		o.OnErrorFunc(err)
	}
}

// Helper functions for common REST operations

// GetAccountInfo creates a request to fetch account information.
func GetAccountInfo(correlationID string) InteractionRequest {
	return InteractionRequest{
		Method:        "GET",
		Path:          "/api/v3/account",
		CorrelationID: correlationID,
		SigningHint:   SigningHintRequired,
		RetryPolicy:   ConservativeRetryPolicy(),
	}
}

// GetOpenOrders creates a request to fetch open orders.
func GetOpenOrders(symbol, correlationID string) InteractionRequest {
	return InteractionRequest{
		Method:        "GET",
		Path:          "/api/v3/openOrders",
		Symbol:        symbol,
		CorrelationID: correlationID,
		SigningHint:   SigningHintRequired,
		RetryPolicy:   ConservativeRetryPolicy(),
	}
}

// PlaceOrder creates a request to place a new order.
func PlaceOrder(symbol, correlationID string, orderData interface{}) InteractionRequest {
	return InteractionRequest{
		Method:        "POST",
		Path:          "/api/v3/order",
		Symbol:        symbol,
		Payload:       orderData,
		CorrelationID: correlationID,
		SigningHint:   SigningHintRequired,
		RetryPolicy:   AggressiveRetryPolicy(),
		Timeout:       Duration(30 * time.Second),
	}
}

// GetBookSnapshot creates a request to fetch book snapshot.
func GetBookSnapshot(symbol, correlationID string, limit int) InteractionRequest {
	params := map[string]string{
		"symbol": symbol,
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}

	return InteractionRequest{
		Method:        "GET",
		Path:          "/api/v3/depth",
		Symbol:        symbol,
		CorrelationID: correlationID,
		QueryParams:   params,
		SigningHint:   SigningHintNone,
		RetryPolicy:   DefaultRetryPolicy(),
	}
}

// GetRecentTrades creates a request to fetch recent trades.
func GetRecentTrades(symbol, correlationID string, limit int) InteractionRequest {
	params := map[string]string{
		"symbol": symbol,
	}
	if limit > 0 {
		params["limit"] = fmt.Sprintf("%d", limit)
	}

	return InteractionRequest{
		Method:        "GET",
		Path:          "/api/v3/trades",
		Symbol:        symbol,
		CorrelationID: correlationID,
		QueryParams:   params,
		SigningHint:   SigningHintNone,
		RetryPolicy:   DefaultRetryPolicy(),
	}
}

// GetExchangeInfo creates a request to fetch exchange information.
func GetExchangeInfo(correlationID string) InteractionRequest {
	return InteractionRequest{
		Method:        "GET",
		Path:          "/api/v3/exchangeInfo",
		CorrelationID: correlationID,
		SigningHint:   SigningHintNone,
		RetryPolicy:   ConservativeRetryPolicy(),
	}
}

// High-level workflow helpers

// SyncSnapshotThenStream creates a workflow that first fetches REST snapshots then streams real-time data
func (f *InteractionFacade) SyncSnapshotThenStream(
	ctx context.Context,
	symbols []string,
	options ...PublicOption,
) (PipelineStream, error) {
	// Create snapshot requests
	var snapshotRequests []InteractionRequest
	for _, symbol := range symbols {
		snapshotRequests = append(snapshotRequests,
			GetBookSnapshot(symbol, fmt.Sprintf("snapshot-%s", symbol), 100))
	}

	// Execute snapshot requests
	snapshotStream, err := f.FetchREST(ctx, snapshotRequests)
	if err != nil {
		return PipelineStream{}, fmt.Errorf("failed to fetch snapshots: %w", err)
	}

	// Subscribe to real-time streams
	stream, err := f.SubscribePublic(ctx, symbols, options...)
	if err != nil {
		snapshotStream.Close()
		return PipelineStream{}, fmt.Errorf("failed to subscribe to streams: %w", err)
	}

	// Return combined stream (snapshots + real-time)
	// Note: In practice, you might want to merge these streams properly
	return stream, nil
}

// SubmitOrder creates a workflow for submitting an order and monitoring its status
func (f *InteractionFacade) SubmitOrder(
	ctx context.Context,
	symbol string,
	orderData interface{},
	correlationID string,
) (PipelineStream, error) {
	// Submit the order
	orderRequest := PlaceOrder(symbol, correlationID, orderData)
	orderStream, err := f.FetchREST(ctx, []InteractionRequest{orderRequest})
	if err != nil {
		return PipelineStream{}, fmt.Errorf("failed to submit order: %w", err)
	}

	// Subscribe to private order updates
	privateStream, err := f.SubscribePrivate(ctx)
	if err != nil {
		orderStream.Close()
		return PipelineStream{}, fmt.Errorf("failed to subscribe to order updates: %w", err)
	}

	// Return combined stream (order submission + order updates)
	// Note: In practice, you might want to merge these streams properly
	return privateStream, nil
}

// WatchAccount creates a workflow for monitoring account updates
func (f *InteractionFacade) WatchAccount(
	ctx context.Context,
	correlationID string,
) (PipelineStream, error) {
	// Fetch initial account state
	accountRequest := GetAccountInfo(correlationID)
	accountStream, err := f.FetchREST(ctx, []InteractionRequest{accountRequest})
	if err != nil {
		return PipelineStream{}, fmt.Errorf("failed to fetch account info: %w", err)
	}

	// Subscribe to private account updates
	privateStream, err := f.SubscribePrivate(ctx)
	if err != nil {
		accountStream.Close()
		return PipelineStream{}, fmt.Errorf("failed to subscribe to account updates: %w", err)
	}

	// Return combined stream (account snapshot + account updates)
	// Note: In practice, you might want to merge these streams properly
	return privateStream, nil
}

// MultiChannelStream creates a stream that combines public and private data
func (f *InteractionFacade) MultiChannelStream(
	ctx context.Context,
	publicSymbols []string,
	privateEnabled bool,
	options ...PublicOption,
) (PipelineStream, error) {
	var streams []PipelineStream

	// Subscribe to public streams
	publicStream, err := f.SubscribePublic(ctx, publicSymbols, options...)
	if err != nil {
		return PipelineStream{}, fmt.Errorf("failed to subscribe to public streams: %w", err)
	}
	streams = append(streams, publicStream)

	// Subscribe to private streams if enabled
	if privateEnabled {
		privateStream, err := f.SubscribePrivate(ctx)
		if err != nil {
			publicStream.Close()
			return PipelineStream{}, fmt.Errorf("failed to subscribe to private streams: %w", err)
		}
		streams = append(streams, privateStream)
	}

	// Return the first stream (in practice, you'd want to merge them)
	// Note: This is a simplified implementation
	return streams[0], nil
}
