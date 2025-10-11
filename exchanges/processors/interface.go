// Package processors hosts message processors for market data routing.
package processors

import "context"

// Processor converts raw market data messages into typed models for downstream handlers.
//
// Initialize prepares the processor for use and must complete within five seconds while
// respecting context cancellation. Process transforms a payload into a domain model,
// returning descriptive errors for malformed inputs and reacting to cancellation within
// one hundred milliseconds. MessageTypeID identifies the message type handled and must
// match the router registration.
type Processor interface {
	Initialize(ctx context.Context) error
	Process(ctx context.Context, raw []byte) (interface{}, error)
	MessageTypeID() string
}
