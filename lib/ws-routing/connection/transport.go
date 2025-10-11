package connection

import "github.com/coachpo/meltica/lib/ws-routing/internal"

// Event is the transport event type shared across ws-routing packages.
type Event = internal.Event

// AbstractTransport defines the contract adapters must satisfy to stream events.
type AbstractTransport interface {
	Receive() <-chan Event
	Send(Event) error
	Close() error
}
