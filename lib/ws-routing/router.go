package wsrouting

import (
	"context"

	"github.com/coachpo/meltica/errs"
)

// Router coordinates inbound routing lifecycle for a framework session.
type Router struct {
	session *FrameworkSession
}

func newRouter(session *FrameworkSession) *Router {
	return &Router{session: session}
}

// HandleRaw consumes a raw payload, applying parsing, middleware, and publication.
func (r *Router) HandleRaw(ctx context.Context, raw []byte) error {
	if r == nil || r.session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("router not configured"))
	}
	msg, err := r.session.parse(ctx, raw)
	if err != nil {
		return err
	}
	processed, err := r.session.applyMiddleware(ctx, msg)
	if err != nil {
		return err
	}
	return r.session.publishMessage(ctx, processed)
}

// Publish bypasses parsing but still executes middleware and publication.
func (r *Router) Publish(ctx context.Context, msg *Message) error {
	if r == nil || r.session == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("router not configured"))
	}
	processed, err := r.session.applyMiddleware(ctx, msg)
	if err != nil {
		return err
	}
	return r.session.publishMessage(ctx, processed)
}
