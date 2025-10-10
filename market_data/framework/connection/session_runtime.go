package connection

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

type sessionRuntime struct {
	engine     *Engine
	session    *framework.ConnectionSession
	conn       *websocket.Conn
	ctx        context.Context
	cancel     context.CancelFunc
	errs       chan<- error
	opts       DialOptions
	done       chan struct{}
	closed     atomic.Bool
	incoming   chan []byte
	hbInterval time.Duration
	hbTimeout  time.Duration
	maxPending int
	throttle   time.Duration
	dispatch   *dispatcher
	metrics    *telemetry.MetricsAggregator
	emitter    *telemetry.Emitter
	invalids   *invalidTracker
}

func newSessionRuntime(engine *Engine, session *framework.ConnectionSession, conn *websocket.Conn, ctx context.Context, cancel context.CancelFunc, errs chan<- error, opts DialOptions) (*sessionRuntime, error) {
	if engine.cfg.poolOpts.MaxMessageBytes > 0 {
		conn.SetReadLimit(int64(engine.cfg.poolOpts.MaxMessageBytes))
	}
	maxPending := engine.cfg.maxPending
	if maxPending <= 0 {
		maxPending = 1
	}
	bufferSize := maxPending
	runtime := &sessionRuntime{
		engine:     engine,
		session:    session,
		conn:       conn,
		ctx:        ctx,
		cancel:     cancel,
		errs:       errs,
		opts:       opts,
		done:       make(chan struct{}),
		incoming:   make(chan []byte, bufferSize),
		hbInterval: engine.cfg.heartbeatInterval,
		hbTimeout:  engine.cfg.heartbeatTimeout,
		maxPending: maxPending,
		throttle:   engine.cfg.throttleDuration,
		invalids:   newInvalidTracker(engine.cfg.metricsWindow, opts.InvalidThreshold),
	}
	dispatch, err := newDispatcher(engine, session, opts)
	if err != nil {
		return nil, err
	}
	runtime.dispatch = dispatch
	runtime.dispatch.setErrorSink(runtime.sendError)
	runtime.dispatch.setEmitter(engine.telemetry)
	runtime.metrics = telemetry.NewMetricsAggregator(engine.cfg.metricsWindow, session.Throughput)
	runtime.emitter = engine.telemetry
	return runtime, nil
}

func (r *sessionRuntime) start() {
	go r.readLoop()
	go r.processLoop()
	r.startHeartbeat()
}

func (r *sessionRuntime) readLoop() {
	defer r.shutdown(stream.SessionClosed, false, nil)
	for {
		if r.hbTimeout > 0 {
			_ = r.conn.SetReadDeadline(time.Now().Add(r.hbTimeout))
		}
		select {
		case <-r.ctx.Done():
			return
		default:
		}
		typeCode, payload, err := r.conn.ReadMessage()
		if err != nil {
			if !isExpectedClose(err) {
				r.sendError(err)
			}
			return
		}
		if typeCode == websocket.PingMessage {
			_ = r.conn.WriteControl(websocket.PongMessage, payload, time.Now().Add(time.Second))
			continue
		}
		r.handleFrame(payload)
	}
}

func (r *sessionRuntime) handleFrame(payload []byte) {
	r.session.SetLastHeartbeat(time.Now().UTC())
	if len(payload) == 0 {
		return
	}
	if !r.enqueue(payload) {
		// Drop payload when throttled to prevent unbounded backpressure.
		err := errs.New("", errs.CodeRateLimited, errs.WithMessage("backpressure limit reached"))
		r.sendError(err)
		if r.metrics != nil {
			r.metrics.RecordFailure()
		}
		r.publish(telemetry.Event{Kind: telemetry.EventBackpressureNotice, Error: err})
	}
}

func (r *sessionRuntime) close(ctx context.Context) error {
	if !r.shutdown(stream.SessionClosed, true, nil) {
		return nil
	}
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (r *sessionRuntime) shutdown(status stream.SessionStatus, sendClose bool, cause error) bool {
	if !r.closed.CompareAndSwap(false, true) {
		return false
	}
	if cause != nil {
		r.sendError(cause)
	}
	r.cancel()
	r.session.SetStatus(status)
	if sendClose {
		msg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		_ = r.conn.WriteControl(websocket.CloseMessage, msg, time.Now().Add(time.Second))
	}
	_ = r.conn.Close()
	r.engine.removeSession(r.session.SessionID)
	close(r.incoming)
	close(r.done)
	return true
}

func (r *sessionRuntime) sendError(err error) {
	if err == nil {
		return
	}
	select {
	case r.errs <- err:
	default:
	}
}

func (r *sessionRuntime) publish(event telemetry.Event) {
	if r == nil || r.emitter == nil {
		return
	}
	if event.SessionID == "" && r.session != nil {
		event.SessionID = r.session.SessionID
	}
	r.emitter.Emit(event)
}

func isExpectedClose(err error) bool {
	return websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) || errors.Is(err, context.Canceled)
}

func (r *sessionRuntime) decodePayload(payload []byte) (*framework.MessageEnvelope, error) {
	if r == nil || r.engine == nil || r.engine.pool == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("connection resources unavailable"))
	}
	env := r.engine.pool.AcquireEnvelope()
	buffer := env.Raw()
	buffer = append(buffer[:0], payload...)
	env.SetRaw(buffer)
	env.SetReceivedAt(time.Now().UTC())
	lease, err := r.engine.pool.BorrowDecoder(buffer)
	if err != nil {
		r.engine.pool.ReleaseEnvelope(env)
		return nil, err
	}
	defer lease.Release()
	var decoded map[string]any
	if err := lease.Decode(&decoded); err != nil {
		r.engine.pool.ReleaseEnvelope(env)
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("json decode failed"), errs.WithCause(err))
	}
	env.SetDecoded(decoded)
	env.SetValidated(true)
	return env, nil
}
