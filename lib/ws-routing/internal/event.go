package internal

import "time"

// Event represents an immutable message routed through ws-routing.
type Event struct {
	Symbol    string
	Type      string
	Payload   []byte
	Timestamp time.Time
}
