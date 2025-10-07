package pipeline

import "time"

// TransportType identifies the transport that delivered an event within the Level-3/Level-4 boundary.
type TransportType uint8

const (
	TransportUnknown TransportType = iota
	TransportPublicWS
	TransportPrivateWS
	TransportREST
	TransportHybrid
)

func (t TransportType) String() string {
	switch t {
	case TransportPublicWS:
		return "public_ws"
	case TransportPrivateWS:
		return "private_ws"
	case TransportREST:
		return "rest"
	case TransportHybrid:
		return "hybrid"
	default:
		return "unknown"
	}
}

// Payload represents the canonical payload types shared between Level-3 adapters and the Level-4 pipeline.
type Payload interface {
	isPayload()
}

// Event is the Level-3/Level-4 contract used inside the pipeline.
type Event struct {
	Transport     TransportType
	Symbol        string
	At            time.Time
	Payload       Payload
	CorrelationID string
	Metadata      map[string]any
}
