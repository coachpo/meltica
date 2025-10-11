// Package contracts defines the interface contracts for the four-layer architecture.
// This is a DESIGN DOCUMENT showing the target interfaces.
// Actual implementation will be in core/layers/ package.
package contracts

import (
	"context"
	"time"
)

// =============================================================================
// Layer 1: Connection Layer Interfaces
// =============================================================================

// Connection manages the lifecycle of network connections to exchanges.
// Implementations must be safe for concurrent use.
//
// Contract guarantees:
// - Connect is idempotent (multiple calls safe)
// - Close releases all resources
// - Close can be called multiple times safely
// - SetReadDeadline affects subsequent Read operations
type Connection interface {
	// Connect establishes the connection to the exchange.
	// Returns error if connection fails.
	// Subsequent calls to Connect on an already-connected instance should
	// either return nil (no-op) or return the same error as initial failure.
	Connect(ctx context.Context) error

	// Close terminates the connection and releases resources.
	// Safe to call multiple times (subsequent calls are no-ops).
	// After Close, the Connection cannot be reused.
	Close() error

	// IsConnected returns true if the connection is currently established.
	IsConnected() bool

	// SetReadDeadline sets the deadline for read operations.
	// A zero value disables the deadline.
	SetReadDeadline(t time.Time) error

	// SetWriteDeadline sets the deadline for write operations.
	// A zero value disables the deadline.
	SetWriteDeadline(t time.Time) error
}

// WSConnection extends Connection for WebSocket-specific operations.
type WSConnection interface {
	Connection

	// ReadMessage reads the next message from the WebSocket.
	// Returns message type and data, or error.
	// Blocks until message available or deadline exceeded.
	ReadMessage() (messageType int, data []byte, err error)

	// WriteMessage writes a message to the WebSocket.
	// messageType should be TextMessage or BinaryMessage.
	WriteMessage(messageType int, data []byte) error

	// Ping sends a ping frame and waits for pong response.
	// Returns error if pong not received within timeout.
	Ping(ctx context.Context) error

	// OnPong registers a handler for pong frames.
	// Handler is called when pong frame received.
	OnPong(handler func())
}

// RESTConnection extends Connection for HTTP REST operations.
type RESTConnection interface {
	Connection

	// Do executes an HTTP request and returns the response.
	// Request should be prepared by Routing layer.
	Do(ctx context.Context, req *HTTPRequest) (*HTTPResponse, error)

	// SetRateLimit configures rate limiting for this connection.
	// Requests are delayed to comply with rate limits.
	SetRateLimit(requestsPerSecond int)
}

// =============================================================================
// Layer 2: Routing Layer Interfaces
// =============================================================================

// Routing handles message routing, translation, and request/response formatting.
//
// Contract guarantees:
// - Subscribe is idempotent for the same subscription
// - Unsubscribe is safe to call multiple times
// - Messages are delivered in order received
// - Parse methods validate and normalize exchange formats
type Routing interface {
	// Subscribe initiates a subscription for market data.
	// Returns error if subscription fails.
	Subscribe(ctx context.Context, req SubscriptionRequest) error

	// Unsubscribe cancels a subscription.
	// Safe to call multiple times (subsequent calls are no-ops).
	Unsubscribe(ctx context.Context, req SubscriptionRequest) error

	// OnMessage registers a handler for incoming messages.
	// Handler is called for each message matching subscriptions.
	OnMessage(handler MessageHandler)

	// ParseMessage converts raw exchange message to normalized format.
	// Returns error if message format invalid.
	ParseMessage(raw []byte) (NormalizedMessage, error)
}

// WSRouting extends Routing for WebSocket-specific routing.
type WSRouting interface {
	Routing

	// ManageListenKey handles listen key lifecycle for private streams.
	// Returns initial listen key and manages keepalive automatically.
	ManageListenKey(ctx context.Context) (string, error)

	// GetStreamURL returns the WebSocket URL for given subscription.
	GetStreamURL(req SubscriptionRequest) (string, error)
}

// RESTRouting extends Routing for REST-specific routing.
type RESTRouting interface {
	Routing

	// BuildRequest constructs an HTTP request for an API call.
	// Handles URL construction, headers, authentication, etc.
	BuildRequest(ctx context.Context, req APIRequest) (*HTTPRequest, error)

	// ParseResponse converts HTTP response to normalized format.
	// Handles error responses and rate limit headers.
	ParseResponse(resp *HTTPResponse) (NormalizedResponse, error)
}

// =============================================================================
// Layer 3: Business Layer Interfaces
// =============================================================================

// Business implements domain and business logic.
//
// Contract guarantees:
// - Methods are safe for concurrent use
// - Errors are typed (errs.E) with appropriate codes
// - State changes are atomic
type Business interface {
	// Process handles a normalized message and applies business logic.
	// Returns processed result or error.
	Process(ctx context.Context, msg NormalizedMessage) (BusinessResult, error)

	// Validate checks if a request is valid according to business rules.
	// Returns error describing validation failure, or nil if valid.
	Validate(ctx context.Context, req BusinessRequest) error

	// GetState returns current business state for monitoring/debugging.
	GetState() BusinessState
}

// OrderBusiness extends Business for order management logic.
type OrderBusiness interface {
	Business

	// PlaceOrder creates a new order.
	// Returns order ID and placement result.
	PlaceOrder(ctx context.Context, order OrderRequest) (string, error)

	// CancelOrder cancels an existing order.
	CancelOrder(ctx context.Context, orderID string) error

	// GetOrderStatus retrieves current status of an order.
	GetOrderStatus(ctx context.Context, orderID string) (OrderStatus, error)
}

// BookBusiness extends Business for order book management.
type BookBusiness interface {
	Business

	// UpdateBook applies an update to the order book.
	// Handles both snapshots and deltas.
	UpdateBook(ctx context.Context, update BookUpdate) error

	// GetBook returns current order book snapshot.
	GetBook(symbol string) (OrderBook, error)

	// OnBookChange registers a handler for book changes.
	OnBookChange(handler func(symbol string, book OrderBook))
}

// =============================================================================
// Layer 4: Filter Layer Interfaces
// =============================================================================

// Filter creates exchange-agnostic market-data pipelines.
//
// Contract guarantees:
// - Filters are composable (can be chained)
// - Filters are stateless or manage their own state safely
// - Errors from one filter don't crash the pipeline
type Filter interface {
	// Apply applies the filter to an event stream.
	// Returns filtered/transformed events.
	Apply(ctx context.Context, events <-chan Event) (<-chan Event, error)

	// Name returns a unique identifier for this filter.
	Name() string

	// Close releases resources used by the filter.
	Close() error
}

// Pipeline coordinates multiple filters and data sources.
type Pipeline interface {
	// AddSource adds a data source to the pipeline.
	AddSource(source DataSource) error

	// AddFilter adds a filter stage to the pipeline.
	AddFilter(filter Filter) error

	// Start begins processing events through the pipeline.
	// Returns a channel for consuming filtered events.
	Start(ctx context.Context) (<-chan Event, error)

	// Stop gracefully shuts down the pipeline.
	Stop() error

	// OnError registers a handler for pipeline errors.
	OnError(handler func(error))
}

// =============================================================================
// Type-Specific Extensions (Market Categories)
// =============================================================================

// SpotConnection extends Connection with spot market specifics.
type SpotConnection interface {
	Connection
	GetSpotEndpoint() string
}

// FuturesConnection extends Connection with futures market specifics.
type FuturesConnection interface {
	Connection
	GetFuturesEndpoint() string
	GetContractDetails() ContractInfo
}

// OptionsConnection extends Connection with options market specifics.
type OptionsConnection interface {
	Connection
	GetOptionsEndpoint() string
	GetContractChain(underlying string) ([]OptionContract, error)
}

// SpotRouting extends Routing with spot market specifics.
type SpotRouting interface {
	Routing
	// Spot-specific routing methods
}

// FuturesRouting extends Routing with futures market specifics.
type FuturesRouting interface {
	Routing
	// Get funding rate stream
	SubscribeFundingRate(ctx context.Context, symbol string) error
}

// OptionsRouting extends Routing with options market specifics.
type OptionsRouting interface {
	Routing
	// Get options greeks stream
	SubscribeGreeks(ctx context.Context, symbol string) error
}

// =============================================================================
// Supporting Types (examples - not exhaustive)
// =============================================================================

type SubscriptionRequest struct {
	Symbol      string
	Type        string // "book", "trade", "ticker", etc.
	Category    string // "spot", "futures", "options"
	IsPrivate   bool
}

type MessageHandler func(msg NormalizedMessage)

type NormalizedMessage struct {
	Type      string
	Symbol    string
	Timestamp time.Time
	Data      interface{}
}

type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

type APIRequest struct {
	Endpoint   string
	Method     string
	Parameters map[string]interface{}
	Signed     bool
}

type NormalizedResponse struct {
	Data  interface{}
	Error error
}

type BusinessResult struct {
	Success bool
	Data    interface{}
	Error   error
}

type BusinessRequest struct {
	Type string
	Data interface{}
}

type BusinessState struct {
	Status     string
	Metrics    map[string]interface{}
	LastUpdate time.Time
}

type OrderRequest struct {
	Symbol   string
	Side     string // "buy" or "sell"
	Type     string // "limit", "market", etc.
	Quantity float64
	Price    float64
}

type OrderStatus struct {
	OrderID   string
	Status    string // "pending", "filled", "cancelled", etc.
	Filled    float64
	Remaining float64
}

type BookUpdate struct {
	Symbol    string
	Bids      []PriceLevel
	Asks      []PriceLevel
	Timestamp time.Time
	IsSnapshot bool
}

type PriceLevel struct {
	Price    string // Use string to preserve precision
	Quantity string
}

type OrderBook struct {
	Symbol    string
	Bids      []PriceLevel
	Asks      []PriceLevel
	Timestamp time.Time
}

type Event struct {
	Type      string
	Payload   interface{}
	Metadata  map[string]string
	Timestamp time.Time
}

type DataSource interface {
	Events() <-chan Event
	Close() error
}

type ContractInfo struct {
	Symbol         string
	TickSize       string
	ContractSize   string
	ExpirationDate time.Time
}

type OptionContract struct {
	Symbol     string
	Strike     string
	Expiration time.Time
	Type       string // "call" or "put"
}
