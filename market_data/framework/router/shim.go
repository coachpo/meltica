package router

import (
	"context"

	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Options instead.
type Options = wsrouting.Options

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.BackoffConfig instead.
type BackoffConfig = wsrouting.BackoffConfig

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.DialOptions instead.
type DialOptions = wsrouting.DialOptions

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.FrameworkSession instead.
type FrameworkSession = wsrouting.FrameworkSession

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.State instead.
type State = wsrouting.State

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.StateInitialized instead.
const (
	StateUnknown     = wsrouting.StateUnknown
	StateInitialized = wsrouting.StateInitialized
	StateStarting    = wsrouting.StateStarting
	StateStreaming   = wsrouting.StateStreaming
	StateStopping    = wsrouting.StateStopping
	StateStopped     = wsrouting.StateStopped
)

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Connection instead.
type Connection = wsrouting.Connection

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Parser instead.
type Parser = wsrouting.Parser

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Dialer instead.
type Dialer = wsrouting.Dialer

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.PublishFunc instead.
type PublishFunc = wsrouting.PublishFunc

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Middleware instead.
type Middleware = wsrouting.Middleware

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Message instead.
type Message = wsrouting.Message

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.SubscriptionSpec instead.
type SubscriptionSpec = wsrouting.SubscriptionSpec

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.QoSLevel instead.
type QoSLevel = wsrouting.QoSLevel

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Init instead.
func Init(ctx context.Context, opts Options) (*FrameworkSession, error) {
	return wsrouting.Init(ctx, opts)
}

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Start instead.
func Start(ctx context.Context, session *FrameworkSession) error {
	return wsrouting.Start(ctx, session)
}

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Subscribe instead.
func Subscribe(ctx context.Context, session *FrameworkSession, spec SubscriptionSpec) error {
	return wsrouting.Subscribe(ctx, session, spec)
}

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.Publish instead.
func Publish(ctx context.Context, session *FrameworkSession, msg *Message) error {
	return wsrouting.Publish(ctx, session, msg)
}

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.UseMiddleware instead.
func UseMiddleware(session *FrameworkSession, handler Middleware) error {
	return wsrouting.UseMiddleware(session, handler)
}

// Deprecated: use github.com/coachpo/meltica/lib/ws-routing.RouteRaw instead.
func RouteRaw(ctx context.Context, session *FrameworkSession, raw []byte) error {
	return wsrouting.RouteRaw(ctx, session, raw)
}
