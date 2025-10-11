package session

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/lib/ws-routing/connection"
	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
	"github.com/coachpo/meltica/lib/ws-routing/router"
	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

// Options configures a Session instance.
type Options struct {
	ID         string
	Transport  connection.AbstractTransport
	Router     *router.Router
	Registry   *handler.Registry
	Middleware *handler.Chain
	Logger     telemetry.Logger
}

// Session orchestrates routing using an abstract transport.
type Session struct {
	mu            sync.RWMutex
	id            string
	startedAt     time.Time
	status        internal.Status
	transport     connection.AbstractTransport
	router        *router.Router
	registry      *handler.Registry
	middleware    *handler.Chain
	logger        telemetry.Logger
	cancel        context.CancelFunc
	receiverDone  chan struct{}
	subscriptions map[string]struct{}
}

// New creates a new Session instance.
func New(opts Options) (*Session, error) {
	id := strings.TrimSpace(opts.ID)
	if id == "" {
		return nil, invalidErr("session id required")
	}
	if opts.Transport == nil {
		return nil, invalidErr("transport required")
	}
	if opts.Router == nil {
		return nil, invalidErr("router required")
	}
	registry := opts.Registry
	if registry == nil {
		registry = handler.NewRegistry()
	}
	middleware := opts.Middleware
	if middleware == nil {
		middleware = handler.NewChain()
	}
	logger := opts.Logger
	if logger == nil {
		logger = telemetry.NewNoop()
	}
	return &Session{
		id:            id,
		status:        internal.StatusStopped,
		transport:     opts.Transport,
		router:        opts.Router,
		registry:      registry,
		middleware:    middleware,
		logger:        logger,
		subscriptions: make(map[string]struct{}),
	}, nil
}

// ID returns the session identifier.
func (s *Session) ID() string {
	if s == nil {
		return ""
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.id
}

// Status returns the current lifecycle status.
func (s *Session) Status() internal.Status {
	if s == nil {
		return internal.StatusStopped
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status
}

// StartedAt reports the timestamp when the session entered the running state.
func (s *Session) StartedAt() time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.startedAt
}

// Registry returns the session handler registry.
func (s *Session) Registry() *handler.Registry {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry
}

// Middleware returns the middleware chain.
func (s *Session) Middleware() *handler.Chain {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.middleware
}

// Subscriptions returns a snapshot of registered subscription topics.
func (s *Session) Subscriptions() []string {
	return s.subscriptionSnapshot()
}

// Start begins consuming events from the underlying transport.
func (s *Session) Start(ctx context.Context) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	transport, cancelPrev, donePrev, err := s.prepareStart()
	if err != nil {
		return err
	}
	if cancelPrev != nil {
		cancelPrev()
	}
	if donePrev != nil {
		<-donePrev
	}
	events := transport.Receive()
	if events == nil {
		s.finishStartFailure()
		return errs.New("", errs.CodeInvalid, errs.WithMessage("transport receive channel unavailable"))
	}
	runCtx, cancel := context.WithCancel(ctx)
	receiverDone := make(chan struct{})
	s.establishRunState(cancel, receiverDone)
	s.logger.Info(ctx, "session starting", telemetry.Field{Key: "session_id", Value: s.id})
	go s.consume(runCtx, events, receiverDone)
	s.markRunning()
	if err := s.replaySubscriptions(ctx, transport); err != nil {
		s.logger.Error(ctx, "subscription replay failed", telemetry.Field{Key: "error", Value: err})
		return err
	}
	s.logger.Info(ctx, "session started", telemetry.Field{Key: "session_id", Value: s.id})
	return nil
}

// Publish routes the supplied event through the session router.
func (s *Session) Publish(ctx context.Context, event internal.Event) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	if !s.isRunning() {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session not running"))
	}
	router := s.currentRouter()
	if router == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("router unavailable"))
	}
	processed, err := s.applyMiddleware(ctx, event)
	if err != nil {
		return err
	}
	return router.Dispatch(ctx, processed)
}

// Subscribe registers interest in a topic and forwards the request to the transport when running.
func (s *Session) Subscribe(ctx context.Context, topic string) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	key := strings.TrimSpace(topic)
	if key == "" {
		return invalidErr("subscription topic required")
	}
	s.mu.Lock()
	if s.subscriptions == nil {
		s.subscriptions = make(map[string]struct{})
	}
	if _, exists := s.subscriptions[key]; exists {
		s.mu.Unlock()
		return nil
	}
	s.subscriptions[key] = struct{}{}
	running := s.status == internal.StatusRunning
	transport := s.transport
	s.mu.Unlock()
	s.logger.Info(ctx, "subscription registered", telemetry.Field{Key: "session_id", Value: s.id}, telemetry.Field{Key: "topic", Value: key})
	if running {
		return s.sendSubscription(ctx, transport, key, true)
	}
	return nil
}

// Unsubscribe removes interest in a topic and forwards the request when running.
func (s *Session) Unsubscribe(ctx context.Context, topic string) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	key := strings.TrimSpace(topic)
	if key == "" {
		return invalidErr("subscription topic required")
	}
	s.mu.Lock()
	if s.subscriptions == nil {
		s.mu.Unlock()
		return errs.New("", errs.CodeInvalid, errs.WithMessage("subscription not found"))
	}
	if _, exists := s.subscriptions[key]; !exists {
		s.mu.Unlock()
		return errs.New("", errs.CodeInvalid, errs.WithMessage("subscription not found"))
	}
	delete(s.subscriptions, key)
	running := s.status == internal.StatusRunning
	transport := s.transport
	s.mu.Unlock()
	s.logger.Info(ctx, "subscription removed", telemetry.Field{Key: "session_id", Value: s.id}, telemetry.Field{Key: "topic", Value: key})
	if running {
		return s.sendSubscription(ctx, transport, key, false)
	}
	return nil
}

// Close stops the session and releases resources.
func (s *Session) Close(ctx context.Context) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	transport, cancel, receiverDone, alreadyStopped := s.prepareClose()
	if alreadyStopped {
		if cancel != nil {
			cancel()
		}
		if receiverDone != nil {
			select {
			case <-receiverDone:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}
	if cancel != nil {
		cancel()
	}
	var closeErr error
	if transport != nil {
		if err := transport.Close(); err != nil {
			closeErr = err
		}
	}
	if receiverDone != nil {
		select {
		case <-receiverDone:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	s.logger.Info(ctx, "session stopped", telemetry.Field{Key: "session_id", Value: s.id})
	return closeErr
}

func (s *Session) prepareStart() (connection.AbstractTransport, context.CancelFunc, <-chan struct{}, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == internal.StatusRunning || s.status == internal.StatusStarting {
		return nil, nil, nil, errs.New("", errs.CodeInvalid, errs.WithMessage("session already running"))
	}
	if s.transport == nil {
		return nil, nil, nil, errs.New("", errs.CodeInvalid, errs.WithMessage("transport unavailable"))
	}
	prevCancel := s.cancel
	prevDone := s.receiverDone
	s.cancel = nil
	s.receiverDone = nil
	s.status = internal.StatusStarting
	s.startedAt = time.Now().UTC()
	return s.transport, prevCancel, prevDone, nil
}

func (s *Session) establishRunState(cancel context.CancelFunc, done chan struct{}) {
	s.mu.Lock()
	s.cancel = cancel
	s.receiverDone = done
	s.mu.Unlock()
}

func (s *Session) markRunning() {
	s.mu.Lock()
	s.status = internal.StatusRunning
	s.mu.Unlock()
}

func (s *Session) finishStartFailure() {
	s.mu.Lock()
	s.status = internal.StatusStopped
	s.mu.Unlock()
}

func (s *Session) currentRouter() *router.Router {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.router
}

func (s *Session) consume(ctx context.Context, events <-chan internal.Event, done chan struct{}) {
	defer close(done)
	for {
		select {
		case <-ctx.Done():
			s.markStopped()
			return
		case event, ok := <-events:
			if !ok {
				s.markStopped()
				return
			}
			s.handleEvent(ctx, event)
		}
	}
}

func (s *Session) handleEvent(ctx context.Context, event internal.Event) {
	processed, err := s.applyMiddleware(ctx, event)
	if err != nil {
		s.logger.Warn(ctx, "middleware failure", telemetry.Field{Key: "error", Value: err})
		return
	}
	router := s.currentRouter()
	if router == nil {
		return
	}
	if err := router.Dispatch(ctx, processed); err != nil {
		s.logger.Error(ctx, "router dispatch failed", telemetry.Field{Key: "error", Value: err})
	}
}

func (s *Session) applyMiddleware(ctx context.Context, event internal.Event) (internal.Event, error) {
	middleware := s.Middleware()
	if middleware == nil {
		return event, nil
	}
	processed, err := middleware.Apply(ctx, event)
	if err != nil {
		return internal.Event{}, err
	}
	return processed, nil
}

func (s *Session) prepareClose() (connection.AbstractTransport, context.CancelFunc, <-chan struct{}, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.status == internal.StatusStopped {
		return s.transport, s.cancel, s.receiverDone, true
	}
	transport := s.transport
	cancel := s.cancel
	done := s.receiverDone
	s.cancel = nil
	s.receiverDone = nil
	s.status = internal.StatusStopped
	return transport, cancel, done, false
}

func (s *Session) markStopped() {
	s.mu.Lock()
	s.status = internal.StatusStopped
	s.mu.Unlock()
}

func (s *Session) isRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.status == internal.StatusRunning
}

func invalidErr(message string) error {
	return errs.New("", errs.CodeInvalid, errs.WithMessage(message))
}

func (s *Session) replaySubscriptions(ctx context.Context, transport connection.AbstractTransport) error {
	topics := s.subscriptionSnapshot()
	for _, topic := range topics {
		if err := s.sendSubscription(ctx, transport, topic, true); err != nil {
			return err
		}
	}
	return nil
}

func (s *Session) subscriptionSnapshot() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.subscriptions) == 0 {
		return nil
	}
	keys := make([]string, 0, len(s.subscriptions))
	for key := range s.subscriptions {
		keys = append(keys, key)
	}
	return keys
}

func (s *Session) sendSubscription(ctx context.Context, transport connection.AbstractTransport, topic string, add bool) error {
	if transport == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("transport unavailable"))
	}
	eventType := "routing.unsubscribe"
	if add {
		eventType = "routing.subscribe"
	}
	event := internal.Event{
		Symbol:    topic,
		Type:      eventType,
		Payload:   []byte(topic),
		Timestamp: time.Now().UTC(),
	}
	if err := transport.Send(event); err != nil {
		s.logger.Error(ctx, "subscription dispatch failed", telemetry.Field{Key: "topic", Value: topic}, telemetry.Field{Key: "error", Value: err})
		return err
	}
	return nil
}
