package wsrouting

import (
	"context"

	"github.com/coachpo/meltica/errs"
)

// Init initializes a framework session.
func Init(ctx context.Context, opts Options) (*FrameworkSession, error) {
	if ctx == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("context required"))
	}
	validated, err := opts.validate()
	if err != nil {
		return nil, err
	}
	return newSession(validated), nil
}

// Start transitions a session into the streaming state.
func Start(ctx context.Context, session *FrameworkSession) error {
	if ctx == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("context required"))
	}
	if session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session required"))
	}
	if session.State() != StateInitialized {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session must be initialized"))
	}
	session.setState(StateStarting)
	dialer := session.getDialer()
	logger := session.getLogger()
	conn, err := dialer.Dial(ctx, DialOptions{
		SessionID: session.ID(),
		Backoff:   session.Backoff(),
		Logger:    logger,
	})
	if err != nil {
		session.setState(StateInitialized)
		if _, ok := err.(*errs.E); ok {
			return err
		}
		return errs.New("", errs.CodeNetwork, errs.WithMessage("failed to dial connection"), errs.WithCause(err))
	}
	session.setConnection(conn)
	session.setState(StateStreaming)
	for _, spec := range session.subscriptionsSnapshot() {
		if err := conn.Subscribe(ctx, spec); err != nil {
			if _, ok := err.(*errs.E); ok {
				return err
			}
			return errs.New("", errs.CodeNetwork, errs.WithMessage("subscription failed"), errs.WithCause(err))
		}
	}
	return nil
}

// Subscribe registers a subscription with the session.
func Subscribe(ctx context.Context, session *FrameworkSession, spec SubscriptionSpec) error {
	if ctx == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("context required"))
	}
	if session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session required"))
	}
	if err := spec.Validate(); err != nil {
		return err
	}
	session.registerSubscription(spec)
	switch session.State() {
	case StateInitialized, StateStarting:
		return nil
	case StateStreaming:
		if conn := session.getConnection(); conn != nil {
			if err := conn.Subscribe(ctx, spec); err != nil {
				if _, ok := err.(*errs.E); ok {
					return err
				}
				return errs.New("", errs.CodeNetwork, errs.WithMessage("subscription failed"), errs.WithCause(err))
			}
		}
		return nil
	default:
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session not active"))
	}
	return nil
}

// Publish emits a routed message to downstream handlers.
func Publish(ctx context.Context, session *FrameworkSession, msg *Message) error {
	if ctx == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("context required"))
	}
	if session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session required"))
	}
	if session.State() != StateStreaming {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session must be streaming to publish"))
	}
	return session.router.Publish(ctx, msg)
}

// UseMiddleware registers middleware with the session.
func UseMiddleware(session *FrameworkSession, handler Middleware) error {
	if session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session required"))
	}
	if handler == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("middleware handler required"))
	}
	return session.middleware.Use(func(ctx context.Context, input any) (any, error) {
		msg, ok := input.(*Message)
		if !ok {
			return nil, errs.New("", errs.CodeExchange, errs.WithMessage("middleware payload must be *Message"))
		}
		return handler(ctx, msg)
	})
}

// RouteRaw processes a raw payload through the routing pipeline.
func RouteRaw(ctx context.Context, session *FrameworkSession, raw []byte) error {
	if ctx == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("context required"))
	}
	if session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session required"))
	}
	if session.State() != StateStreaming {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session must be streaming"))
	}
	return session.router.HandleRaw(ctx, raw)
}
