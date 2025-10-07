package filter

import (
	"encoding/json"
	"math/big"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
)

// ChannelType classifies the communication channel for interactions.
type ChannelType string

const (
	ChannelPublicWS  ChannelType = "public_ws"
	ChannelPrivateWS ChannelType = "private_ws"
	ChannelREST      ChannelType = "rest"
	ChannelHybrid    ChannelType = "hybrid"
)

// EventKind classifies canonical market-data events that flow through the filter pipeline.
type EventKind string

const (
	EventKindUnknown      EventKind = "unknown"
	EventKindBook         EventKind = "book"
	EventKindTrade        EventKind = "trade"
	EventKindTicker       EventKind = "ticker"
	EventKindVWAP         EventKind = "vwap"
	EventKindAccount      EventKind = "account"
	EventKindOrder        EventKind = "order"
	EventKindRestResponse EventKind = "rest_response"
)

// EventEnvelope wraps a normalized event emitted by the filter pipeline.
type EventEnvelope struct {
	Kind          EventKind
	Channel       ChannelType
	Symbol        string
	Timestamp     time.Time
	CorrelationID string

	Book         *corestreams.BookEvent
	Trade        *corestreams.TradeEvent
	Ticker       *corestreams.TickerEvent
	Stats        *AnalyticsEvent
	AccountEvent *AccountEvent
	OrderEvent   *OrderEvent
	RestResponse *RestResponse
}

// AnalyticsEvent contains derived metrics computed by the filter pipeline.
type AnalyticsEvent struct {
	Symbol     string
	VWAP       *big.Rat
	TradeCount int64
}

// AccountEvent represents account-related events from private streams.
type AccountEvent struct {
	Symbol    string
	Balance   *big.Rat
	Available *big.Rat
	Locked    *big.Rat
}

// OrderEvent represents order-related events from private streams.
type OrderEvent struct {
	Symbol      string
	OrderID     string
	Side        string
	Price       *big.Rat
	Quantity    *big.Rat
	Status      string
	Type        string
	TimeInForce string
}

// RestResponse represents a REST API response.
type RestResponse struct {
	RequestID  string
	Method     string
	Path       string
	StatusCode int
	Body       interface{}
	Error      error
}

// Observer receives callbacks for events and errors flowing through the filter pipeline.
type Observer interface {
	OnEvent(EventEnvelope)
	OnError(error)
}

// InteractionRequest represents a single interaction with an exchange.
type InteractionRequest struct {
	Channel       ChannelType
	Symbol        string
	Method        string
	Path          string
	Payload       interface{}
	CorrelationID string
	AuthContext   *AuthContext

	// REST-specific fields
	QueryParams map[string]string
	Headers     map[string]string
	Timeout     Duration
	RetryPolicy *RetryPolicy
	SigningHint SigningHint
}

// Duration is a wrapper for time.Duration that supports JSON marshaling
type Duration time.Duration

// MarshalJSON implements json.Marshaler
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

// UnmarshalJSON implements json.Unmarshaler
func (d *Duration) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(dur)
	return nil
}

// RetryPolicy defines retry behavior for REST requests
type RetryPolicy struct {
	MaxAttempts          int
	BaseDelay            Duration
	MaxDelay             Duration
	BackoffMultiplier    float64
	RetryableStatusCodes []int
	RetryableErrors      []string
}

// SigningHint provides hints about request signing requirements
type SigningHint string

const (
	SigningHintAuto     SigningHint = "auto"     // Determine signing automatically based on path
	SigningHintRequired SigningHint = "required" // Force signing
	SigningHintNone     SigningHint = "none"     // Force no signing
)

// AuthContext contains authentication information for private interactions.
type AuthContext struct {
	APIKey    string
	Secret    string
	ListenKey string

	// Binance-specific metadata
	ListenKeyMetadata *ListenKeyMetadata
	SessionState      SessionState
	LastRenewal       time.Time
	RenewalInterval   Duration
	ExpiresAt         time.Time
}

// ListenKeyMetadata contains Binance-specific listen key information
type ListenKeyMetadata struct {
	Key           string
	CreatedAt     time.Time
	LastKeepAlive time.Time
	ExpiresAt     time.Time
	RenewalCount  int
	FailureCount  int
	IsActive      bool
}

// SessionState represents the state of a private session
type SessionState string

const (
	SessionStateInactive   SessionState = "inactive"
	SessionStateActive     SessionState = "active"
	SessionStateRenewing   SessionState = "renewing"
	SessionStateExpired    SessionState = "expired"
	SessionStateFailed     SessionState = "failed"
	SessionStateTerminated SessionState = "terminated"
)

// SessionConfig contains configuration for private session management
type SessionConfig struct {
	KeepAliveInterval  Duration
	RenewalThreshold   Duration
	MaxRenewalFailures int
	AutoRenew          bool
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

	// EnablePrivate enables private stream subscriptions (account, orders).
	EnablePrivate bool

	// RESTRequests contains REST API calls to be executed through the pipeline.
	RESTRequests []InteractionRequest

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
