package categories

import "github.com/coachpo/meltica/core/layers"

// SpotConnection exposes spot-market specific helpers on top of a connection.
type SpotConnection interface {
	layers.Connection

	// GetSpotEndpoint returns the spot market endpoint used by the connection.
	GetSpotEndpoint() string
}

// SpotRouting describes routing behaviours unique to spot markets.
type SpotRouting interface {
	layers.Routing
	// Spot-specific hooks may be added as the routing contracts evolve.
}
