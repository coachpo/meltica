package handler

import (
	"context"
	"strings"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework"
)

type contextKey string

const (
	contextKeySession   contextKey = "framework/session"
	contextKeyMetadata  contextKey = "framework/metadata"
	contextKeyBinding   contextKey = "framework/binding"
	contextKeyAuthToken contextKey = "framework/auth_token"
)

// Binding declares how a registered handler attaches to decoded channels.
type Binding struct {
	Name           string
	Channels       []string
	MaxConcurrency int
}

// Contains reports whether the binding subscribes to the provided channel identifier.
func (b Binding) Contains(channel string) bool {
	upper := strings.ToUpper(strings.TrimSpace(channel))
	for _, ch := range b.Channels {
		if ch == upper {
			return true
		}
	}
	return false
}

// HandlerFunc adapts a function to the stream.Handler interface.
type HandlerFunc func(context.Context, stream.Envelope) (stream.Outcome, error)

// Handle executes the wrapped function.
func (fn HandlerFunc) Handle(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
	if fn == nil {
		return nil, nil
	}
	return fn(ctx, env)
}

// Chain applies the given middleware in-order to produce a wrapped handler.
func Chain(base stream.Handler, middleware ...stream.Middleware) stream.Handler {
	if base == nil {
		return nil
	}
	wrapped := base
	for i := len(middleware) - 1; i >= 0; i-- {
		mw := middleware[i]
		if mw == nil {
			continue
		}
		wrapped = mw(wrapped)
	}
	return wrapped
}

// WithSession injects the connection session into handler contexts.
func WithSession(session *framework.ConnectionSession) stream.Middleware {
	return func(next stream.Handler) stream.Handler {
		if next == nil {
			return nil
		}
		return HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
			if session != nil {
				ctx = context.WithValue(ctx, contextKeySession, session)
			}
			return next.Handle(ctx, env)
		})
	}
}

// WithMetadata merges session metadata into the handler context.
func WithMetadata(meta map[string]string) stream.Middleware {
	return func(next stream.Handler) stream.Handler {
		if next == nil {
			return nil
		}
		return HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
			if len(meta) > 0 {
				clone := make(map[string]string, len(meta))
				for k, v := range meta {
					clone[k] = v
				}
				ctx = context.WithValue(ctx, contextKeyMetadata, clone)
			}
			return next.Handle(ctx, env)
		})
	}
}

// WithAuthToken attaches the dial authentication token to the context for downstream middleware.
func WithAuthToken(token string) stream.Middleware {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return func(h stream.Handler) stream.Handler { return h }
	}
	return func(next stream.Handler) stream.Handler {
		if next == nil {
			return nil
		}
		return HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
			ctx = context.WithValue(ctx, contextKeyAuthToken, trimmed)
			return next.Handle(ctx, env)
		})
	}
}

// WithBinding exposes the resolved binding to middleware and handlers.
func WithBinding(binding Binding) stream.Middleware {
	return func(next stream.Handler) stream.Handler {
		if next == nil {
			return nil
		}
		return HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
			ctx = context.WithValue(ctx, contextKeyBinding, binding)
			return next.Handle(ctx, env)
		})
	}
}

// SessionFromContext retrieves the current connection session if present.
func SessionFromContext(ctx context.Context) (*framework.ConnectionSession, bool) {
	if ctx == nil {
		return nil, false
	}
	session, ok := ctx.Value(contextKeySession).(*framework.ConnectionSession)
	return session, ok
}

// MetadataFromContext returns session metadata populated by WithMetadata.
func MetadataFromContext(ctx context.Context) map[string]string {
	if ctx == nil {
		return nil
	}
	meta, _ := ctx.Value(contextKeyMetadata).(map[string]string)
	return meta
}

// AuthTokenFromContext extracts the dial authentication token when present.
func AuthTokenFromContext(ctx context.Context) (string, bool) {
	if ctx == nil {
		return "", false
	}
	token, _ := ctx.Value(contextKeyAuthToken).(string)
	if token == "" {
		return "", false
	}
	return token, true
}

// BindingFromContext exposes the binding associated with the current handler invocation.
func BindingFromContext(ctx context.Context) (Binding, bool) {
	if ctx == nil {
		return Binding{}, false
	}
	binding, ok := ctx.Value(contextKeyBinding).(Binding)
	return binding, ok
}
