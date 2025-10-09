package connection

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/handler"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

const (
	defaultDialTimeout   = 10 * time.Second
	defaultMetricsWindow = 30 * time.Second
)

// Config configures the Engine behavior.
type Config struct {
	Endpoint          string
	DialTimeout       time.Duration
	Pool              PoolOptions
	Handshake         AuthConfig
	ReadBufferSize    int
	WriteBufferSize   int
	EnableCompression bool
	MetricsWindow     time.Duration
	Heartbeat         HeartbeatConfig
	Backpressure      BackpressureConfig
}

// HeartbeatConfig determines ping cadence and liveness enforcement.
type HeartbeatConfig struct {
	Interval time.Duration
	Liveness time.Duration
}

// BackpressureConfig controls inbound buffering and throttling.
type BackpressureConfig struct {
	MaxPending int
	Throttle   time.Duration
}

type engineConfig struct {
	endpoint          string
	dialTimeout       time.Duration
	poolOpts          PoolOptions
	readBufferSize    int
	writeBufferSize   int
	enableCompression bool
	metricsWindow     time.Duration
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	maxPending        int
	throttleDuration  time.Duration
}

// Engine implements stream.Engine for WebSocket sessions.
type Engine struct {
	cfg       engineConfig
	pool      *PoolHandle
	auth      AuthModule
	dialer    websocket.Dialer
	metrics   *metricsReporter
	telemetry *telemetry.Emitter
	reg       *handler.Registry
	sessions  sync.Map
	closed    atomic.Bool
	baseCtx   context.Context
	cancelCtx context.CancelFunc
}

// NewEngine constructs a streaming engine with pooled resources.
func NewEngine(cfg Config) (*Engine, error) {
	parsed, err := normalizeConfig(cfg)
	if err != nil {
		return nil, err
	}
	handle, err := NewPoolHandle(parsed.poolOpts)
	if err != nil {
		return nil, err
	}
	baseCtx, cancel := context.WithCancel(context.Background())
	engine := &Engine{
		cfg:       parsed,
		pool:      handle,
		auth:      NewAuthModule(cfg.Handshake),
		metrics:   newMetricsReporter(parsed.metricsWindow),
		telemetry: telemetry.NewEmitter(),
		reg:       handler.NewRegistry(),
		baseCtx:   baseCtx,
		cancelCtx: cancel,
	}
	engine.dialer = websocket.Dialer{
		HandshakeTimeout:  parsed.dialTimeout,
		ReadBufferSize:    parsed.readBufferSize,
		WriteBufferSize:   parsed.writeBufferSize,
		EnableCompression: parsed.enableCompression,
	}
	return engine, nil
}

// RegisterHandler stores handler registrations for later dispatch.
func (e *Engine) RegisterHandler(reg stream.HandlerRegistration) error {
	_, err := e.reg.Register(reg)
	return err
}

// Dial establishes a WebSocket session using pooled resources.
func (e *Engine) Dial(ctx context.Context) (stream.Session, error) {
	if ctx == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("context is required"))
	}
	if e.closed.Load() {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("engine is closed"))
	}
	opts := dialOptionsFromContext(ctx)
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = e.cfg.endpoint
	}
	if endpoint == "" {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("endpoint is required"))
	}
	handshakeReq := HandshakeRequest{
		Endpoint:  endpoint,
		Protocols: opts.Protocols,
		AuthToken: opts.AuthToken,
		Headers:   opts.Headers,
		Metadata:  opts.Metadata,
	}
	decision, err := e.auth.Evaluate(ctx, handshakeReq)
	if err != nil {
		return nil, err
	}
	requestCtx := ctx
	if e.cfg.dialTimeout > 0 {
		var cancel context.CancelFunc
		requestCtx, cancel = context.WithTimeout(ctx, e.cfg.dialTimeout)
		defer cancel()
	}
	headers := make(http.Header)
	for k, v := range opts.Headers {
		headers.Add(k, v)
	}
	dialer := e.dialer
	dialer.Subprotocols = opts.Protocols
	conn, resp, err := dialer.DialContext(requestCtx, endpoint, headers)
	if err != nil {
		if resp != nil && resp.Body != nil {
			resp.Body.Close()
		}
		return nil, wrapDialError(err, resp)
	}
	if resp != nil && resp.Body != nil {
		resp.Body.Close()
	}
	sessionID := newSessionID()
	negotiated := resolvedProtocols(opts.Protocols, conn)
	session := &framework.ConnectionSession{
		SessionID:    sessionID,
		EndpointAddr: endpoint,
		Protocols:    negotiated,
		ConnectedAt:  time.Now().UTC(),
		LastBeat:     time.Now().UTC(),
		State:        stream.SessionActive,
		Throughput: &framework.MetricsSnapshot{
			Session:      sessionID,
			WindowLength: e.cfg.metricsWindow,
		},
	}
	if decision.Context != nil {
		session.SetContext(decision.Context)
	} else {
		session.SetContext(ctx)
	}
	session.SetMetadata(mergeMetadata(decision.Metadata, opts.Metadata))
	errCh := make(chan error, 1)
	session.SetErrorChannel(errCh)
	runCtx, cancelRun := context.WithCancel(session.Context())
	runtime, err := newSessionRuntime(e, session, conn, runCtx, cancelRun, errCh, opts)
	if err != nil {
		cancelRun()
		_ = conn.Close()
		return nil, err
	}
	session.SetCloseFunc(runtime.close)
	e.sessions.Store(sessionID, runtime)
	e.metrics.Track(sessionID, session.Throughput)
	runtime.start()
	return session, nil
}

// Metrics exposes the metrics reporter.
func (e *Engine) Metrics() stream.MetricsReporter {
	return e.metrics
}

// Telemetry exposes the shared event emitter for observers.
func (e *Engine) Telemetry() *telemetry.Emitter {
	return e.telemetry
}

// Close shuts down the engine and all sessions.
func (e *Engine) Close(ctx context.Context) error {
	if !e.closed.CompareAndSwap(false, true) {
		return nil
	}
	e.cancelCtx()
	done := make(chan struct{})
	go func() {
		e.sessions.Range(func(key, value any) bool {
			runtime := value.(*sessionRuntime)
			runtime.close(context.Background())
			return true
		})
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (e *Engine) removeSession(id string) {
	e.sessions.Delete(id)
	e.metrics.Remove(id)
}

func normalizeConfig(cfg Config) (engineConfig, error) {
	parsed := engineConfig{
		endpoint:          strings.TrimSpace(cfg.Endpoint),
		dialTimeout:       cfg.DialTimeout,
		poolOpts:          cfg.Pool,
		readBufferSize:    cfg.ReadBufferSize,
		writeBufferSize:   cfg.WriteBufferSize,
		enableCompression: cfg.EnableCompression,
		metricsWindow:     cfg.MetricsWindow,
		heartbeatInterval: cfg.Heartbeat.Interval,
		heartbeatTimeout:  cfg.Heartbeat.Liveness,
		maxPending:        cfg.Backpressure.MaxPending,
		throttleDuration:  cfg.Backpressure.Throttle,
	}
	if parsed.dialTimeout <= 0 {
		parsed.dialTimeout = defaultDialTimeout
	}
	if parsed.metricsWindow <= 0 {
		parsed.metricsWindow = defaultMetricsWindow
	}
	if parsed.poolOpts.MaxMessageBytes == 0 {
		parsed.poolOpts.MaxMessageBytes = defaultMaxMessageBytes
	}
	if parsed.heartbeatInterval <= 0 {
		parsed.heartbeatInterval = 15 * time.Second
	}
	if parsed.heartbeatTimeout <= 0 {
		parsed.heartbeatTimeout = parsed.heartbeatInterval * 3
	}
	if parsed.maxPending < 0 {
		parsed.maxPending = 0
	}
	if parsed.maxPending == 0 {
		parsed.maxPending = 128
	}
	if parsed.throttleDuration <= 0 {
		parsed.throttleDuration = 50 * time.Millisecond
	}
	return parsed, nil
}

func resolvedProtocols(requested []string, conn *websocket.Conn) []string {
	if conn == nil {
		return requested
	}
	proto := strings.TrimSpace(conn.Subprotocol())
	if proto != "" {
		return []string{proto}
	}
	return requested
}

func newSessionID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func wrapDialError(err error, resp *http.Response) error {
	if err == nil {
		return nil
	}
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return errs.New("", errs.CodeAuth, errs.WithMessage("authentication failed"), errs.WithCause(err))
		case http.StatusTooManyRequests, http.StatusConflict:
			return errs.New("", errs.CodeRateLimited, errs.WithMessage("capacity reached"), errs.WithCause(err))
		case http.StatusBadRequest:
			return errs.New("", errs.CodeInvalid, errs.WithMessage("invalid session request"), errs.WithCause(err))
		}
	}
	if errors.Is(err, context.Canceled) {
		return errs.New("", errs.CodeNetwork, errs.WithMessage("dial canceled"), errs.WithCause(err))
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errs.New("", errs.CodeNetwork, errs.WithMessage("dial timeout"), errs.WithCause(err))
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return errs.New("", errs.CodeNetwork, errs.WithMessage("dial timeout"), errs.WithCause(err))
		}
		return errs.New("", errs.CodeNetwork, errs.WithMessage("network error"), errs.WithCause(err))
	}
	return errs.New("", errs.CodeNetwork, errs.WithMessage("dial failed"), errs.WithCause(err))
}

type dialOptionsKey struct{}

// DialOptions carries per-dial overrides.
type DialOptions struct {
	Endpoint         string
	Protocols        []string
	Headers          map[string]string
	AuthToken        string
	Metadata         map[string]string
	InvalidThreshold uint32
	Bindings         []handler.Binding
}

// WithDialOptions annotates the context with dial options.
func WithDialOptions(ctx context.Context, opts DialOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, dialOptionsKey{}, opts)
}

func dialOptionsFromContext(ctx context.Context) DialOptions {
	if ctx == nil {
		return DialOptions{}
	}
	opts, _ := ctx.Value(dialOptionsKey{}).(DialOptions)
	return opts
}

func mergeMetadata(values ...map[string]string) map[string]string {
	merged := make(map[string]string)
	for _, src := range values {
		for k, v := range src {
			if k == "" {
				continue
			}
			merged[k] = v
		}
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

type metricsReporter struct {
	mu        sync.RWMutex
	window    time.Duration
	snapshots map[string]*framework.MetricsSnapshot
}

func newMetricsReporter(window time.Duration) *metricsReporter {
	if window <= 0 {
		window = defaultMetricsWindow
	}
	return &metricsReporter{
		window:    window,
		snapshots: make(map[string]*framework.MetricsSnapshot),
	}
}

func (m *metricsReporter) Track(id string, snapshot *framework.MetricsSnapshot) {
	if snapshot == nil {
		snapshot = &framework.MetricsSnapshot{Session: id, WindowLength: m.window}
	}
	m.mu.Lock()
	if snapshot.WindowLength <= 0 {
		snapshot.WindowLength = m.window
	}
	m.snapshots[id] = snapshot
	m.mu.Unlock()
}

func (m *metricsReporter) Remove(id string) {
	m.mu.Lock()
	delete(m.snapshots, id)
	m.mu.Unlock()
}

func (m *metricsReporter) Snapshot(id string) (stream.MetricsSample, bool) {
	m.mu.RLock()
	snap, ok := m.snapshots[id]
	m.mu.RUnlock()
	if !ok {
		return nil, false
	}
	copy := *snap
	return &copy, true
}

func (m *metricsReporter) All() []stream.MetricsSample {
	m.mu.RLock()
	defer m.mu.RUnlock()
	results := make([]stream.MetricsSample, 0, len(m.snapshots))
	for _, snap := range m.snapshots {
		copy := *snap
		results = append(results, &copy)
	}
	return results
}

var _ stream.Engine = (*Engine)(nil)
var _ stream.MetricsReporter = (*metricsReporter)(nil)
