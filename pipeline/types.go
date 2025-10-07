package pipeline

import (
	"encoding/json"
	"time"
)

// ChannelType classifies the communication channel for interactions exposed to clients.
type ChannelType string

const (
	ChannelPublicWS  ChannelType = "public_ws"
	ChannelPrivateWS ChannelType = "private_ws"
	ChannelREST      ChannelType = "rest"
	ChannelHybrid    ChannelType = "hybrid"
)

// TransportPayload aliases the canonical payload interface for client consumption.
type TransportPayload = Payload

// ClientEvent is emitted by the Level-4 façade to callers.
type ClientEvent struct {
	Channel       ChannelType
	Symbol        string
	At            time.Time
	Payload       TransportPayload
	CorrelationID string
	Metadata      map[string]any
}

// Observer receives callbacks for events and errors flowing through the filter pipeline.
type Observer interface {
	OnEvent(ClientEvent)
	OnError(error)
}

// InteractionRequest represents a single interaction with an exchange.
type InteractionRequest struct {
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

// PipelineRequest declares the desired feeds and pipeline configuration for a session.
type PipelineRequest struct {
	Symbols []string
	Feeds   FeedSelection

	// BookDepth limits the number of order book levels emitted. Values <= 0 keep full depth.
	BookDepth int

	// SamplingInterval throttles event emission when set to a non-zero duration.
	SamplingInterval time.Duration

	// MinEmitInterval enforces a minimum time between events per symbol/type.
	MinEmitInterval time.Duration

	// EnableSnapshots controls whether the coordinator stores last-known events per feed.
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

// PipelineStream is the multiplexed output of a pipeline.
type PipelineStream struct {
	Events <-chan ClientEvent
	Errors <-chan error
	cancel func()
	cache  *snapshotCache
}

// Close cancels the underlying pipeline.
func (s PipelineStream) Close() {
	if s.cancel != nil {
		s.cancel()
	}
}

// LastBookEvent returns the latest cached book event for the given symbol.
func (s PipelineStream) LastBookEvent(symbol string) (ClientEvent, bool) {
	return s.lastEventByType("book", symbol)
}

// LastTradeEvent returns the latest cached trade event for the given symbol.
func (s PipelineStream) LastTradeEvent(symbol string) (ClientEvent, bool) {
	return s.lastEventByType("trade", symbol)
}

// LastTickerEvent returns the latest cached ticker event for the given symbol.
func (s PipelineStream) LastTickerEvent(symbol string) (ClientEvent, bool) {
	return s.lastEventByType("ticker", symbol)
}

// LastAccountEvent returns the latest cached account event for the given symbol.
func (s PipelineStream) LastAccountEvent(symbol string) (ClientEvent, bool) {
	return s.lastEventByType("account", symbol)
}

// LastOrderEvent returns the latest cached order event for the given symbol.
func (s PipelineStream) LastOrderEvent(symbol string) (ClientEvent, bool) {
	return s.lastEventByType("order", symbol)
}

func (s PipelineStream) lastEventByType(eventType string, symbol string) (ClientEvent, bool) {
	if s.cache == nil {
		return ClientEvent{}, false
	}
	return s.cache.Get(eventType, symbol)
}
