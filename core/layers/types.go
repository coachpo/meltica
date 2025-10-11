package layers

import "time"

// HTTPRequest represents a normalized HTTP request issued by routing layers.
type HTTPRequest struct {
	Method  string
	URL     string
	Headers map[string]string
	Body    []byte
}

// HTTPResponse captures the exchange response with raw payload and metadata.
type HTTPResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
}

// APIRequest describes a logical API operation that routing layers translate
// into HTTP requests for REST connections.
type APIRequest struct {
	Endpoint   string
	Method     string
	Parameters map[string]any
	Signed     bool
}

// NormalizedResponse returns a parsed representation of an HTTP response.
type NormalizedResponse struct {
	Data  any
	Error error
}

// SubscriptionRequest defines an intent to consume a specific data stream.
type SubscriptionRequest struct {
	Symbol    string
	Type      string
	Category  string
	IsPrivate bool
}

// MessageHandler is invoked for each normalized message flowing through a
// routing layer.
type MessageHandler func(msg NormalizedMessage)

// NormalizedMessage represents a decoded exchange payload.
type NormalizedMessage struct {
	Type      string
	Symbol    string
	Timestamp time.Time
	Data      any
}

// BusinessResult captures the outcome of business-layer processing.
type BusinessResult struct {
	Success bool
	Data    any
	Error   error
}

// BusinessRequest conveys an operation targeted at the business layer.
type BusinessRequest struct {
	Type string
	Data any
}

// BusinessState carries snapshot information for diagnostics.
type BusinessState struct {
	Status     string
	Metrics    map[string]any
	LastUpdate time.Time
}

// OrderRequest is a normalized order placement instruction.
type OrderRequest struct {
	Symbol   string
	Side     string
	Type     string
	Quantity float64
	Price    float64
}

// OrderStatus reflects the state of a previously submitted order.
type OrderStatus struct {
	OrderID   string
	Status    string
	Filled    float64
	Remaining float64
}

// BookUpdate carries a snapshot or delta for an order book.
type BookUpdate struct {
	Symbol     string
	Bids       []PriceLevel
	Asks       []PriceLevel
	Timestamp  time.Time
	IsSnapshot bool
}

// PriceLevel retains price and quantity with string precision guarantees.
type PriceLevel struct {
	Price    string
	Quantity string
}

// OrderBook expresses the current state of a market's depth.
type OrderBook struct {
	Symbol    string
	Bids      []PriceLevel
	Asks      []PriceLevel
	Timestamp time.Time
}

// Event represents a pipeline event flowing through filters.
type Event struct {
	Type      string
	Payload   any
	Metadata  map[string]string
	Timestamp time.Time
}

// DataSource exposes a read-only stream of events.
type DataSource interface {
	Events() <-chan Event
	Close() error
}

// ContractInfo provides metadata for derivative contracts.
type ContractInfo struct {
	Symbol         string
	TickSize       string
	ContractSize   string
	ExpirationDate time.Time
}

// OptionContract enumerates a tradeable option instrument.
type OptionContract struct {
	Symbol     string
	Strike     string
	Expiration time.Time
	Type       string
}
