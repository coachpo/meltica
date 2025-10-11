package wsrouting

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/internal/observability"
	"github.com/coachpo/meltica/lib/ws-routing/middleware"
)

// FrameworkSession represents a reusable WebSocket routing session.
type FrameworkSession struct {
	mu            sync.RWMutex
	id            string
	state         State
	backoff       BackoffConfig
	dialer        Dialer
	parser        Parser
	publish       PublishFunc
	logger        observability.Logger
	connection    Connection
	middleware    *middleware.Chain
	subscriptions map[string]SubscriptionSpec
	order         []string
	router        *Router
}

func newSession(opts Options) *FrameworkSession {
	session := &FrameworkSession{
		id:            opts.SessionID,
		state:         StateInitialized,
		backoff:       opts.Backoff,
		dialer:        opts.Dialer,
		parser:        opts.Parser,
		publish:       opts.Publish,
		logger:        opts.Logger,
		middleware:    middleware.NewChain(),
		subscriptions: make(map[string]SubscriptionSpec),
		order:         make([]string, 0),
	}
	session.router = newRouter(session)
	return session
}

// ID returns the session identifier.
func (s *FrameworkSession) ID() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// State returns the current lifecycle state of the session.
func (s *FrameworkSession) State() State {
	if s == nil {
		return StateUnknown
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Backoff returns a copy of the configured backoff policy.
func (s *FrameworkSession) Backoff() BackoffConfig {
	if s == nil {
		return BackoffConfig{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backoff
}

func (s *FrameworkSession) setState(state State) {
	s.mu.Lock()
	s.state = state
	s.mu.Unlock()
}

func (s *FrameworkSession) setConnection(conn Connection) {
	s.mu.Lock()
	s.connection = conn
	s.mu.Unlock()
}

func (s *FrameworkSession) getConnection() Connection {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connection
}

func (s *FrameworkSession) getDialer() Dialer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dialer
}

func (s *FrameworkSession) getLogger() observability.Logger {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.logger
}

func (s *FrameworkSession) publishMessage(ctx context.Context, msg *Message) error {
	s.mu.RLock()
	fn := s.publish
	s.mu.RUnlock()
	if fn == nil {
		return errs.New("", errs.CodeExchange, errs.WithMessage("publish function not configured"))
	}
	if err := fn(ctx, msg); err != nil {
		if _, ok := err.(*errs.E); ok {
			return err
		}
		return errs.New("", errs.CodeExchange, errs.WithMessage("publish failed"), errs.WithCause(err))
	}
	return nil
}

func (s *FrameworkSession) registerSubscription(spec SubscriptionSpec) {
	key := spec.Key()
	s.mu.Lock()
	if _, exists := s.subscriptions[key]; !exists {
		s.order = append(s.order, key)
	}
	s.subscriptions[key] = spec
	s.mu.Unlock()
}

func (s *FrameworkSession) subscriptionsSnapshot() []SubscriptionSpec {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.order) == 0 {
		return nil
	}
	result := make([]SubscriptionSpec, 0, len(s.order))
	for _, key := range s.order {
		if spec, ok := s.subscriptions[key]; ok {
			result = append(result, spec)
		}
	}
	return result
}

func (s *FrameworkSession) parse(ctx context.Context, raw []byte) (*Message, error) {
	s.mu.RLock()
	parser := s.parser
	s.mu.RUnlock()
	if parser == nil {
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("parser not configured"))
	}
	msg, err := parser.Parse(ctx, raw)
	if err != nil {
		if _, ok := err.(*errs.E); ok {
			return nil, err
		}
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("failed to parse message"), errs.WithCause(err))
	}
	if msg == nil {
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("parser returned nil message"))
	}
	return msg, nil
}

func (s *FrameworkSession) applyMiddleware(ctx context.Context, msg *Message) (*Message, error) {
	if msg == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("message required"))
	}
	result, err := s.middleware.Execute(ctx, msg)
	if err != nil {
		if _, ok := err.(*errs.E); ok {
			return nil, err
		}
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("middleware execution failed"), errs.WithCause(err))
	}
	cast, ok := result.(*Message)
	if !ok || cast == nil {
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("middleware returned invalid message"))
	}
	return cast, nil
}

func (s *FrameworkSession) nextBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return s.backoff.Initial
	}
	base := s.backoff.Initial
	multiplier := s.backoff.multiplier()
	current := base
	for i := 0; i < attempt; i++ {
		current = multiplyDuration(current, multiplier)
		if current >= s.backoff.Max && s.backoff.Max > 0 {
			return s.backoff.Max
		}
	}
	if s.backoff.Max > 0 && current > s.backoff.Max {
		return s.backoff.Max
	}
	if current < base {
		return base
	}
	return current
}

func multiplyDuration(d time.Duration, factor rational) time.Duration {
	if factor.den == 0 {
		return d
	}
	if factor.num <= 0 {
		return d
	}
	num := d * time.Duration(factor.num)
	result := num / time.Duration(factor.den)
	if num%time.Duration(factor.den) != 0 {
		result++
	}
	if result <= 0 {
		return d
	}
	return result
}
