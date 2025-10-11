package apiwsrouting

import (
	"context"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/internal/observability"
	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
	"github.com/coachpo/meltica/market_data/framework/connection"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

// SessionConfig captures the normalized session parameters required to establish
// a market data streaming session.
type SessionConfig struct {
	Endpoint         string
	Protocols        []string
	AuthToken        string
	InvalidThreshold uint32
	Metadata         map[string]string
	Bindings         []handler.Binding
}

// SessionResult exposes the created framework and stream sessions.
type SessionResult struct {
	Framework *wsrouting.FrameworkSession
	Stream    stream.Session
}

// Manager orchestrates session lifecycle using the shared ws-routing framework.
type Manager struct {
	engine stream.Engine
	logger observability.Logger
}

// NewManager constructs a Manager bound to the provided engine and logger.
func NewManager(engine stream.Engine, logger observability.Logger) *Manager {
	if engine == nil {
		return nil
	}
	if logger == nil {
		logger = observability.Log()
	}
	return &Manager{engine: engine, logger: logger}
}

// CreateSession provisions a framework session, replays requested subscriptions,
// and returns both the framework and underlying stream sessions.
func (m *Manager) CreateSession(ctx context.Context, cfg SessionConfig) (SessionResult, error) {
	if m == nil || m.engine == nil {
		return SessionResult{}, errs.New("", errs.CodeInvalid, errs.WithMessage("session manager not configured"))
	}

	dialer := &engineDialer{engine: m.engine, config: cfg}
	options := wsrouting.Options{
		SessionID: newFrameworkSessionID(),
		Dialer:    dialer,
		Parser:    noopParser{},
		Publish:   noopPublisher,
		Backoff:   defaultBackoffConfig(),
		Logger:    m.logger,
	}

	frameworkSession, err := wsrouting.Init(ctx, options)
	if err != nil {
		return SessionResult{}, err
	}

	subscriptions := buildSubscriptions(cfg)
	for _, spec := range subscriptions {
		if subErr := wsrouting.Subscribe(ctx, frameworkSession, spec); subErr != nil {
			return SessionResult{}, subErr
		}
	}

	if err := wsrouting.Start(ctx, frameworkSession); err != nil {
		return SessionResult{}, err
	}

	streamSession := dialer.Session()
	if streamSession == nil {
		return SessionResult{}, errs.New("", errs.CodeInvalid, errs.WithMessage("stream session unavailable"))
	}

	return SessionResult{Framework: frameworkSession, Stream: streamSession}, nil
}

type engineDialer struct {
	engine  stream.Engine
	config  SessionConfig
	mu      sync.RWMutex
	session stream.Session
}

func (d *engineDialer) Dial(ctx context.Context, opts wsrouting.DialOptions) (wsrouting.Connection, error) {
	if d == nil || d.engine == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("engine dialer not configured"))
	}
	options := connection.DialOptions{
		Endpoint:         d.config.Endpoint,
		Protocols:        d.config.Protocols,
		AuthToken:        d.config.AuthToken,
		InvalidThreshold: d.config.InvalidThreshold,
		Metadata:         d.config.Metadata,
		Bindings:         d.config.Bindings,
	}
	ctx = connection.WithDialOptions(ctx, options)
	sess, err := d.engine.Dial(ctx)
	if err != nil {
		return nil, err
	}
	d.mu.Lock()
	d.session = sess
	d.mu.Unlock()
	return &engineConnection{session: sess}, nil
}

func (d *engineDialer) Session() stream.Session {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.session
}

type engineConnection struct {
	session stream.Session
}

func (c *engineConnection) Subscribe(context.Context, wsrouting.SubscriptionSpec) error {
	return nil
}

func (c *engineConnection) Close(ctx context.Context) error {
	if c == nil || c.session == nil {
		return nil
	}
	return c.session.Close(ctx)
}

type noopParser struct{}

func (noopParser) Parse(context.Context, []byte) (*wsrouting.Message, error) {
	return &wsrouting.Message{Type: "noop", Payload: map[string]any{}, Metadata: map[string]string{}}, nil
}

func noopPublisher(context.Context, *wsrouting.Message) error { return nil }

func defaultBackoffConfig() wsrouting.BackoffConfig {
	return wsrouting.BackoffConfig{
		Initial:    500 * time.Millisecond,
		Max:        5 * time.Second,
		Multiplier: big.NewRat(3, 2),
		Jitter:     15,
	}
}

func buildSubscriptions(cfg SessionConfig) []wsrouting.SubscriptionSpec {
	if len(cfg.Bindings) == 0 {
		return nil
	}
	exchange := deriveExchange(cfg.Endpoint)
	seen := make(map[string]struct{})
	var specs []wsrouting.SubscriptionSpec
	for _, binding := range cfg.Bindings {
		if len(binding.Channels) == 0 {
			continue
		}
		channels := make([]string, 0, len(binding.Channels))
		for _, channel := range binding.Channels {
			trimmed := strings.TrimSpace(channel)
			if trimmed == "" {
				continue
			}
			channels = append(channels, trimmed)
		}
		if len(channels) == 0 {
			continue
		}
		for _, channel := range binding.Channels {
			key := fmt.Sprintf("%s|%s", binding.Name, channel)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			specs = append(specs, wsrouting.SubscriptionSpec{
				Exchange: exchange,
				Channel:  binding.Name,
				Symbols:  channels,
				QoS:      wsrouting.QoSRealtime,
			})
		}
	}
	return specs
}

func deriveExchange(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "market_data"
	}
	host := strings.TrimSpace(parsed.Host)
	if host == "" {
		return "market_data"
	}
	return host
}

func newFrameworkSessionID() string {
	return fmt.Sprintf("ws-routing-%d", time.Now().UnixNano())
}
