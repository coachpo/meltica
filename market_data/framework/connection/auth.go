package connection

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/coachpo/meltica/errs"
)

const defaultAuthTimeout = 5 * time.Second

// Authenticator verifies clients during the handshake phase.
type Authenticator interface {
	Authenticate(ctx context.Context, req HandshakeRequest) (*HandshakeDecision, error)
}

// AuthenticatorFunc adapts a function to the Authenticator interface.
type AuthenticatorFunc func(context.Context, HandshakeRequest) (*HandshakeDecision, error)

// Authenticate executes the wrapped function.
func (f AuthenticatorFunc) Authenticate(ctx context.Context, req HandshakeRequest) (*HandshakeDecision, error) {
	if f == nil {
		return nil, nil
	}
	return f(ctx, req)
}

// AuthConfig controls handshake authentication behavior.
type AuthConfig struct {
	Hook    Authenticator
	Timeout time.Duration
}

// HandshakeRequest supplies connection details to the authenticator.
type HandshakeRequest struct {
	Endpoint  string
	Protocols []string
	AuthToken string
	Headers   map[string]string
	Metadata  map[string]string
}

// HandshakeDecision returns context and metadata to associate with the session.
type HandshakeDecision struct {
	Context  context.Context
	Metadata map[string]string
}

// AuthModule evaluates handshake hooks when configured.
type AuthModule struct {
	cfg AuthConfig
}

// NewAuthModule prepares an authentication module with sensible defaults.
func NewAuthModule(cfg AuthConfig) AuthModule {
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultAuthTimeout
	}
	return AuthModule{cfg: cfg}
}

// Enabled reports whether an authenticator is configured.
func (m AuthModule) Enabled() bool {
	return m.cfg.Hook != nil
}

// Evaluate executes the authenticator and normalizes its result.
func (m AuthModule) Evaluate(ctx context.Context, req HandshakeRequest) (HandshakeDecision, error) {
	if m.cfg.Hook == nil {
		return HandshakeDecision{Context: ctx, Metadata: sanitizeMetadata(nil)}, nil
	}
	req.Metadata = sanitizeMetadata(req.Metadata)
	ctxWithTimeout := ctx
	var cancel context.CancelFunc
	if m.cfg.Timeout > 0 {
		ctxWithTimeout, cancel = context.WithTimeout(ctx, m.cfg.Timeout)
		defer cancel()
	}
	decision, err := m.cfg.Hook.Authenticate(ctxWithTimeout, req)
	if err != nil {
		return HandshakeDecision{}, normalizeAuthError(err)
	}
	if decision == nil {
		return HandshakeDecision{Context: ctx, Metadata: nil}, nil
	}
	if decision.Context == nil {
		decision.Context = ctx
	}
	decision.Metadata = sanitizeMetadata(decision.Metadata)
	return *decision, nil
}

func sanitizeMetadata(meta map[string]string) map[string]string {
	if len(meta) == 0 {
		return nil
	}
	clean := make(map[string]string, len(meta))
	for k, v := range meta {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		clean[key] = strings.TrimSpace(v)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func normalizeAuthError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return errs.New("", errs.CodeAuth, errs.WithMessage("authentication canceled"), errs.WithCause(err))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.New("", errs.CodeAuth, errs.WithMessage("authentication timeout"), errs.WithCause(err))
	}
	var e *errs.E
	if errors.As(err, &e) {
		return err
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		msg = "authentication rejected"
	}
	return errs.New("", errs.CodeAuth, errs.WithMessage(msg), errs.WithCause(err))
}
