package filter

import (
	"math/big"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
)

// EventKind classifies canonical market-data events that flow through the filter pipeline.
type EventKind string

const (
	EventKindUnknown EventKind = "unknown"
	EventKindBook    EventKind = "book"
	EventKindTrade   EventKind = "trade"
	EventKindTicker  EventKind = "ticker"
	EventKindVWAP    EventKind = "vwap"
)

// EventEnvelope wraps a normalized event emitted by the filter pipeline.
type EventEnvelope struct {
	Kind      EventKind
	Symbol    string
	Timestamp time.Time

	Book   *corestreams.BookEvent
	Trade  *corestreams.TradeEvent
	Ticker *corestreams.TickerEvent
	Stats  *AnalyticsEvent
}

// AnalyticsEvent contains derived metrics computed by the filter pipeline.
type AnalyticsEvent struct {
	Symbol     string
	VWAP       *big.Rat
	TradeCount int64
}

// Observer receives callbacks for events and errors flowing through the filter pipeline.
type Observer interface {
	OnEvent(EventEnvelope)
	OnError(error)
}

// FilterRequest declares the desired feeds and filter policies for a session.
type FilterRequest struct {
	Symbols []string
	Feeds   FeedSelection

	// BookDepth limits the number of order book levels emitted. Values <= 0 keep full depth.
	BookDepth int

	// SamplingInterval throttles event emission when set to a non-zero duration.
	SamplingInterval time.Duration

	// MinEmitInterval enforces a minimum time between events per symbol/kind.
	MinEmitInterval time.Duration

	// EnableSnapshots controls whether the coordinator stores last-known envelopes per feed.
	EnableSnapshots bool

	// EnableVWAP toggles generation of running VWAP analytics events.
	EnableVWAP bool

	// Observer receives event/error callbacks after reliability handling.
	Observer Observer
}

// FeedSelection toggles the exchange feeds that should be sourced.
type FeedSelection struct {
	Books   bool
	Trades  bool
	Tickers bool
}

// FilterStream is the multiplexed output of a filter pipeline.
type FilterStream struct {
	Events <-chan EventEnvelope
	Errors <-chan error
	cancel func()
	cache  *snapshotCache
}

// Close cancels the underlying filter pipeline.
func (s FilterStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// Snapshot returns the most recent envelope for the given kind and symbol.
func (s FilterStream) Snapshot(kind EventKind, symbol string) (EventEnvelope, bool) {
	if s.cache == nil {
		return EventEnvelope{}, false
	}
	return s.cache.Get(kind, symbol)
}
