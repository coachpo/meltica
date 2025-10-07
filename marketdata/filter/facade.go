package filter

import (
	"context"
	"time"
)

// InteractionFacade provides a high-level interface for Level-4 interactions.
type InteractionFacade struct {
	coordinator *Coordinator
}

// NewInteractionFacade creates a new facade for Level-4 interactions.
func NewInteractionFacade(adapter Adapter, auth *AuthContext) *InteractionFacade {
	coordinator := NewCoordinator(adapter, auth)
	return &InteractionFacade{coordinator: coordinator}
}

// SubscribePublic subscribes to public market data feeds.
func (f *InteractionFacade) SubscribePublic(
	ctx context.Context,
	symbols []string,
	options ...PublicOption,
) (FilterStream, error) {
	config := &PublicConfig{
		Books:   false,
		Trades:  true,
		Tickers: true,
	}
	for _, opt := range options {
		opt(config)
	}

	req := FilterRequest{
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

	return f.coordinator.Stream(ctx, req)
}

// SubscribePrivate subscribes to private account and order streams.
func (f *InteractionFacade) SubscribePrivate(
	ctx context.Context,
	options ...PrivateOption,
) (FilterStream, error) {
	config := &PrivateConfig{}
	for _, opt := range options {
		opt(config)
	}

	req := FilterRequest{
		EnablePrivate: true,
		MinEmitInterval: config.MinEmitInterval,
		EnableSnapshots: config.EnableSnapshots,
		Observer:        config.Observer,
	}

	return f.coordinator.Stream(ctx, req)
}

// FetchREST executes REST API calls through the filter pipeline.
func (f *InteractionFacade) FetchREST(
	ctx context.Context,
	requests []InteractionRequest,
	options ...RESTOption,
) (FilterStream, error) {
	config := &RESTConfig{}
	for _, opt := range options {
		opt(config)
	}

	req := FilterRequest{
		RESTRequests:   requests,
		MinEmitInterval: config.MinEmitInterval,
		Observer:        config.Observer,
	}

	return f.coordinator.Stream(ctx, req)
}

// ExecuteSingleREST executes a single REST API call.
func (f *InteractionFacade) ExecuteSingleREST(
	ctx context.Context,
	method string,
	path string,
	payload interface{},
	correlationID string,
) (FilterStream, error) {
	req := InteractionRequest{
		Channel:       ChannelREST,
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

// WithBooks enables order book subscriptions.
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

// WithBookDepth sets the maximum depth for order books.
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

// WithSnapshots enables snapshot caching.
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

// SimpleObserver is a simple implementation of the Observer interface.
type SimpleObserver struct {
	OnEventFunc func(EventEnvelope)
	OnErrorFunc func(error)
}

// OnEvent handles incoming events.
func (o *SimpleObserver) OnEvent(evt EventEnvelope) {
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
		Channel:       ChannelREST,
		Method:        "GET",
		Path:          "/api/v3/account",
		CorrelationID: correlationID,
	}
}

// GetOpenOrders creates a request to fetch open orders.
func GetOpenOrders(symbol, correlationID string) InteractionRequest {
	return InteractionRequest{
		Channel:       ChannelREST,
		Method:        "GET",
		Path:          "/api/v3/openOrders",
		Symbol:        symbol,
		CorrelationID: correlationID,
	}
}

// PlaceOrder creates a request to place a new order.
func PlaceOrder(symbol, correlationID string, orderData interface{}) InteractionRequest {
	return InteractionRequest{
		Channel:       ChannelREST,
		Method:        "POST",
		Path:          "/api/v3/order",
		Symbol:        symbol,
		Payload:       orderData,
		CorrelationID: correlationID,
	}
}