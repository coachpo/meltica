package router

import (
	"context"
	"fmt"
	"sync"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

const globalSymbolKey = "*"

// Options configures router state and concurrency behavior.
type Options struct {
	Registry   *handler.Registry
	Logger     telemetry.Logger
	BufferSize int
}

// Router dispatches events while honoring ordering guarantees.
type Router struct {
	registry    *handler.Registry
	logger      telemetry.Logger
	bufferSize  int
	mu          sync.RWMutex
	workers     map[string]*symbolWorker
	closed      bool
	requestPool sync.Pool
}

// New constructs a Router instance.
func New(opts Options) (*Router, error) {
	if opts.Registry == nil {
		return nil, invalidErr("registry required")
	}
	if opts.BufferSize < 0 {
		return nil, invalidErr("buffer size cannot be negative")
	}
	logger := opts.Logger
	if logger == nil {
		logger = telemetry.NewNoop()
	}
	r := &Router{
		registry:   opts.Registry,
		logger:     logger,
		bufferSize: opts.BufferSize,
		workers:    make(map[string]*symbolWorker),
	}
	r.requestPool.New = func() any {
		return &dispatchRequest{done: make(chan struct{}, 1)}
	}
	return r, nil
}

// Dispatch routes the provided event through registered handlers.
func (r *Router) Dispatch(ctx context.Context, event internal.Event) error {
	if ctx == nil {
		return invalidErr("context required")
	}
	req := r.acquireRequest(ctx, event)
	worker, err := r.workerFor(event.Symbol)
	if err != nil {
		r.releaseRequest(req)
		return err
	}
	if err := worker.push(ctx, req); err != nil {
		r.releaseRequest(req)
		return err
	}
	req.wait()
	result := req.err
	r.releaseRequest(req)
	return result
}

// Close terminates all routing workers.
func (r *Router) Close() {
	workers := r.shutdown()
	for _, worker := range workers {
		worker.stop()
	}
}

func (r *Router) shutdown() []*symbolWorker {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil
	}
	r.closed = true
	workers := make([]*symbolWorker, 0, len(r.workers))
	for _, worker := range r.workers {
		workers = append(workers, worker)
	}
	r.workers = make(map[string]*symbolWorker)
	return workers
}

func (r *Router) workerFor(symbol string) (*symbolWorker, error) {
	key := symbol
	if key == "" {
		key = globalSymbolKey
	}
	r.mu.RLock()
	if r.closed {
		r.mu.RUnlock()
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("router closed"))
	}
	if worker, ok := r.workers[key]; ok {
		r.mu.RUnlock()
		return worker, nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("router closed"))
	}
	if worker, ok := r.workers[key]; ok {
		return worker, nil
	}
	worker := newSymbolWorker(r, key, r.bufferSize)
	r.workers[key] = worker
	return worker, nil
}

func (r *Router) acquireRequest(ctx context.Context, event internal.Event) *dispatchRequest {
	raw := r.requestPool.Get()
	req := raw.(*dispatchRequest)
	req.prepare(ctx, event)
	return req
}

func (r *Router) releaseRequest(req *dispatchRequest) {
	req.reset()
	r.requestPool.Put(req)
}

func (r *Router) invokeHandlers(symbol string, req *dispatchRequest) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("handler panic: %v", rec)
			r.logger.Error(req.ctx, "handler panic", telemetry.Field{Key: "symbol", Value: symbol}, telemetry.Field{Key: "error", Value: err})
		}
	}()
	handlers := r.registry.HandlersSnapshot(req.event.Type)
	for _, h := range handlers {
		if h == nil {
			continue
		}
		if err = h(req.ctx, req.event); err != nil {
			r.logger.Error(req.ctx, "handler failure", telemetry.Field{Key: "symbol", Value: symbol}, telemetry.Field{Key: "error", Value: err})
			return err
		}
	}
	return nil
}

type dispatchRequest struct {
	ctx   context.Context
	event internal.Event
	err   error
	done  chan struct{}
}

func (req *dispatchRequest) prepare(ctx context.Context, event internal.Event) {
	req.ctx = ctx
	req.event = event
	req.err = nil
	select {
	case <-req.done:
	default:
	}
}

func (req *dispatchRequest) signal(err error) {
	req.err = err
	select {
	case req.done <- struct{}{}:
	default:
	}
}

func (req *dispatchRequest) wait() {
	<-req.done
}

func (req *dispatchRequest) reset() {
	req.ctx = nil
	req.event = internal.Event{}
	req.err = nil
	select {
	case <-req.done:
	default:
	}
}

type symbolWorker struct {
	router *Router
	symbol string
	queue  chan *dispatchRequest
	done   chan struct{}
	once   sync.Once
	inline bool
	permit chan struct{}
}

func newSymbolWorker(router *Router, symbol string, buffer int) *symbolWorker {
	worker := &symbolWorker{
		router: router,
		symbol: symbol,
		done:   make(chan struct{}),
	}
	if buffer == 0 {
		worker.inline = true
		worker.permit = make(chan struct{}, 1)
		worker.permit <- struct{}{}
		close(worker.done)
		return worker
	}
	worker.queue = make(chan *dispatchRequest, buffer)
	go worker.run()
	return worker
}

func (w *symbolWorker) run() {
	defer close(w.done)
	for req := range w.queue {
		err := w.router.invokeHandlers(w.symbol, req)
		req.signal(err)
	}
}

func (w *symbolWorker) push(ctx context.Context, req *dispatchRequest) (err error) {
	if w.inline {
		return w.pushInline(ctx, req)
	}
	defer func() {
		if rec := recover(); rec != nil {
			err = errs.New("", errs.CodeInvalid, errs.WithMessage("router closed"))
		}
	}()
	select {
	case w.queue <- req:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *symbolWorker) pushInline(ctx context.Context, req *dispatchRequest) error {
	if done := ctx.Done(); done != nil {
		select {
		case <-done:
			return ctx.Err()
		case <-w.permit:
		}
	} else {
		<-w.permit
	}
	err := w.router.invokeHandlers(w.symbol, req)
	req.signal(err)
	w.permit <- struct{}{}
	return nil
}

func (w *symbolWorker) stop() {
	w.once.Do(func() {
		if w.inline {
			return
		}
		close(w.queue)
	})
	<-w.done
}

func invalidErr(message string) error {
	return errs.New("", errs.CodeInvalid, errs.WithMessage(message))
}
