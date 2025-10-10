package connection

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/handler"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

func newTestEngine(t *testing.T) *Engine {
	t.Helper()
	pool, err := NewPoolHandle(PoolOptions{MaxMessageBytes: 1024})
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}
	return &Engine{
		pool:      pool,
		telemetry: telemetry.NewEmitter(),
		reg:       handler.NewRegistry(),
	}
}

func TestSessionRuntimeDecodePayloadSuccess(t *testing.T) {
	engine := newTestEngine(t)
	runtime := &sessionRuntime{engine: engine}
	payload := []byte(`{"channel":"ticker","value":42}`)
	env, err := runtime.decodePayload(payload)
	if err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if !env.Validated() {
		t.Fatalf("expected envelope marked as validated")
	}
	if string(env.Raw()) != string(payload) {
		t.Fatalf("expected raw payload to match input")
	}
	decoded, ok := env.Decoded().(map[string]any)
	if !ok {
		t.Fatalf("expected decoded payload to be a map")
	}
	if decoded["channel"] != "ticker" {
		t.Fatalf("expected channel field to be preserved")
	}
	engine.pool.ReleaseEnvelope(env)
}

func TestSessionRuntimeDecodePayloadInvalidJSON(t *testing.T) {
	engine := newTestEngine(t)
	runtime := &sessionRuntime{engine: engine}
	env, err := runtime.decodePayload([]byte("{"))
	if err == nil {
		t.Fatalf("expected decode error")
	}
	if env != nil {
		t.Fatalf("expected no envelope on error")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected errs.E, got %v", err)
	}
	if e.Code != errs.CodeInvalid {
		t.Fatalf("expected invalid code, got %s", e.Code)
	}
}

func TestSessionRuntimeHandleFrameBackpressure(t *testing.T) {
	session := &framework.ConnectionSession{SessionID: "sess", Throughput: &framework.MetricsSnapshot{Session: "sess"}}
	incoming := make(chan []byte, 1)
	incoming <- []byte("full")
	errCh := make(chan error, 1)
	runtime := &sessionRuntime{
		ctx:      context.Background(),
		session:  session,
		incoming: incoming,
		throttle: 0,
		errs:     errCh,
		metrics:  telemetry.NewMetricsAggregator(time.Second, session.Throughput),
		emitter:  telemetry.NewEmitter(),
	}
	runtime.handleFrame([]byte(`{"k":1}`))
	if session.LastHeartbeat().IsZero() {
		t.Fatalf("expected heartbeat timestamp to be updated")
	}
	select {
	case err := <-errCh:
		var e *errs.E
		if !errors.As(err, &e) {
			t.Fatalf("expected errs.E, got %v", err)
		}
		if e.Code != errs.CodeRateLimited {
			t.Fatalf("expected rate limited code, got %s", e.Code)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected throttling error to be emitted")
	}
	snapshot := runtime.metrics.Snapshot()
	if snapshot.ErrorsTotal != 1 {
		t.Fatalf("expected a single recorded failure, got %d", snapshot.ErrorsTotal)
	}
}

func TestSessionRuntimeProcessLoopSuccess(t *testing.T) {
	engine := newTestEngine(t)
	session := &framework.ConnectionSession{SessionID: "session-1", Throughput: &framework.MetricsSnapshot{Session: "session-1"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	session.SetErrorChannel(errCh)
	handled := make(chan struct{}, 1)
	if err := engine.RegisterHandler(stream.HandlerRegistration{
		Name:     "echo",
		Version:  "v1",
		Channels: []string{"ticker"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				handled <- struct{}{}
				return framework.HandlerOutcome{OutcomeType: stream.OutcomeAck, LatencyDur: 5 * time.Millisecond}, nil
			})
		},
	}); err != nil {
		t.Fatalf("register handler: %v", err)
	}
	dispatch, err := newDispatcher(engine, session, DialOptions{})
	if err != nil {
		t.Fatalf("new dispatcher: %v", err)
	}
	runtime := &sessionRuntime{
		engine:     engine,
		session:    session,
		ctx:        ctx,
		cancel:     cancel,
		incoming:   make(chan []byte, 1),
		metrics:    telemetry.NewMetricsAggregator(10*time.Millisecond, session.Throughput),
		invalids:   newInvalidTracker(time.Second, 3),
		emitter:    engine.telemetry,
		errs:       errCh,
		maxPending: 1,
	}
	runtime.invalids.recordFailure()
	session.SetInvalidCount(1)
	dispatch.setErrorSink(runtime.sendError)
	dispatch.setEmitter(engine.telemetry)
	runtime.dispatch = dispatch
	done := make(chan struct{})
	go func() {
		runtime.processLoop()
		close(done)
	}()
	payload := []byte(`{"channel":"ticker","value":42}`)
	runtime.incoming <- payload
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatalf("handler was not invoked")
	}
	close(runtime.incoming)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("process loop did not exit")
	}
	snapshot := runtime.metrics.Snapshot()
	if snapshot.MessagesTotal != 1 {
		t.Fatalf("expected metrics to record message, got %d", snapshot.MessagesTotal)
	}
	if snapshot.ErrorsTotal != 0 {
		t.Fatalf("expected zero errors, got %d", snapshot.ErrorsTotal)
	}
	if snapshot.Allocated != uint64(len(payload)) {
		t.Fatalf("expected allocation %d, got %d", len(payload), snapshot.Allocated)
	}
	if session.InvalidCount != 0 {
		t.Fatalf("expected invalid count reset, got %d", session.InvalidCount)
	}
	select {
	case err := <-errCh:
		t.Fatalf("unexpected error propagated: %v", err)
	default:
	}
}

func TestSessionRuntimeProcessLoopDecodeFailure(t *testing.T) {
	engine := newTestEngine(t)
	session := &framework.ConnectionSession{SessionID: "session-fail", Throughput: &framework.MetricsSnapshot{Session: "session-fail"}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	runtime := &sessionRuntime{
		engine:   engine,
		session:  session,
		ctx:      ctx,
		cancel:   cancel,
		incoming: make(chan []byte, 1),
		metrics:  telemetry.NewMetricsAggregator(10*time.Millisecond, session.Throughput),
		invalids: newInvalidTracker(time.Second, 2),
		emitter:  engine.telemetry,
		errs:     errCh,
	}
	done := make(chan struct{})
	go func() {
		runtime.processLoop()
		close(done)
	}()
	runtime.incoming <- []byte("{")
	var recv error
	select {
	case recv = <-errCh:
	case <-time.After(time.Second):
		t.Fatalf("expected decode error to be emitted")
	}
	close(runtime.incoming)
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("process loop did not exit on failure")
	}
	if session.InvalidCount == 0 {
		t.Fatalf("expected invalid count to be tracked")
	}
	snapshot := runtime.metrics.Snapshot()
	if snapshot.ErrorsTotal == 0 {
		t.Fatalf("expected metrics failure to be recorded")
	}
	if snapshot.MessagesTotal != 0 {
		t.Fatalf("expected no successful messages, got %d", snapshot.MessagesTotal)
	}
	var e *errs.E
	if !errors.As(recv, &e) {
		t.Fatalf("expected errs.E, got %v", recv)
	}
	if e.Code != errs.CodeInvalid {
		t.Fatalf("expected invalid error code, got %s", e.Code)
	}
}
