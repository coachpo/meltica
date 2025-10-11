package layers

import "context"

// Routing coordinates subscriptions, message parsing, and delivery.
//
// Contracts:
//   - Subscribe must be idempotent for identical requests.
//   - Unsubscribe must be safe to call repeatedly.
//   - Message handlers receive events in arrival order.
//   - ParseMessage validates and normalizes raw payloads.
type Routing interface {
	// Subscribe requests delivery of the stream described by req.
	Subscribe(ctx context.Context, req SubscriptionRequest) error

	// Unsubscribe ceases delivery for the stream described by req.
	Unsubscribe(ctx context.Context, req SubscriptionRequest) error

	// OnMessage registers the handler invoked for normalized messages.
	OnMessage(handler MessageHandler)

	// ParseMessage converts raw exchange payloads into normalized messages.
	ParseMessage(raw []byte) (NormalizedMessage, error)
}

// WSRouting augments Routing with WebSocket stream tooling.
type WSRouting interface {
	Routing

	// ManageListenKey initializes and maintains listen-key lifecycles for
	// private subscriptions, returning the initial token.
	ManageListenKey(ctx context.Context) (string, error)

	// GetStreamURL resolves the WebSocket endpoint for a subscription.
	GetStreamURL(req SubscriptionRequest) (string, error)
}

// RESTRouting augments Routing with REST request/response helpers.
type RESTRouting interface {
	Routing

	// BuildRequest prepares an HTTP request for the underlying REST connection.
	BuildRequest(ctx context.Context, req APIRequest) (*HTTPRequest, error)

	// ParseResponse normalizes a REST response into an internal structure.
	ParseResponse(resp *HTTPResponse) (NormalizedResponse, error)
}
