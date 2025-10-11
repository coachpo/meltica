package layers

import "context"

// Business encapsulates domain-specific logic operating on normalized data.
//
// Contracts:
//   - Implementations must be safe for concurrent use.
//   - Errors should be typed for precise classification.
//   - State mutations must be atomic from the caller's perspective.
type Business interface {
	// Process applies business rules to the provided message and returns a
	// result representing downstream work.
	Process(ctx context.Context, msg NormalizedMessage) (BusinessResult, error)

	// Validate checks whether an inbound request satisfies business invariants.
	Validate(ctx context.Context, req BusinessRequest) error

	// GetState exposes a snapshot for monitoring or debugging consumers.
	GetState() BusinessState
}

// OrderBusiness extends Business with order-management semantics.
type OrderBusiness interface {
	Business

	// PlaceOrder submits a new order and returns its identifier.
	PlaceOrder(ctx context.Context, order OrderRequest) (string, error)

	// CancelOrder attempts to cancel the referenced order.
	CancelOrder(ctx context.Context, orderID string) error

	// GetOrderStatus retrieves the current status for the referenced order.
	GetOrderStatus(ctx context.Context, orderID string) (OrderStatus, error)
}

// BookBusiness extends Business with order book handling capabilities.
type BookBusiness interface {
	Business

	// UpdateBook mutates the tracked book using a snapshot or delta.
	UpdateBook(ctx context.Context, update BookUpdate) error

	// GetBook returns the latest book snapshot for the supplied symbol.
	GetBook(symbol string) (OrderBook, error)

	// OnBookChange registers a handler invoked when the book changes.
	OnBookChange(handler func(symbol string, book OrderBook))
}
