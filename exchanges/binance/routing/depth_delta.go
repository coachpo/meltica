package routing

import (
	"time"

	"github.com/coachpo/meltica/core"
)

// DepthDelta represents a single depth update from Binance.
type DepthDelta struct {
	Symbol        string
	VenueSymbol   string
	FirstUpdateID int64
	LastUpdateID  int64
	Bids          []core.BookDepthLevel
	Asks          []core.BookDepthLevel
	EventTime     time.Time
}
