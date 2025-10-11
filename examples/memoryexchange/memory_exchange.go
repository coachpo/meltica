package memoryexchange

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
)

type MemoryConnection struct {
	mu            sync.Mutex
	connected     bool
	readDeadline  time.Time
	writeDeadline time.Time
}

func NewMemoryConnection() *MemoryConnection {
	return &MemoryConnection{}
}

func (c *MemoryConnection) Connect(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = true
	return nil
}

func (c *MemoryConnection) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	return nil
}

func (c *MemoryConnection) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected
}

func (c *MemoryConnection) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadline = t
	return nil
}

func (c *MemoryConnection) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadline = t
	return nil
}

type MemoryRouting struct {
	mu        sync.Mutex
	handler   layers.MessageHandler
	subscribe map[string]layers.SubscriptionRequest
}

func NewMemoryRouting() *MemoryRouting {
	return &MemoryRouting{
		subscribe: make(map[string]layers.SubscriptionRequest),
	}
}

func (r *MemoryRouting) Subscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	key := subscriptionKey(req)
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribe[key] = req
	return nil
}

func (r *MemoryRouting) Unsubscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	key := subscriptionKey(req)
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.subscribe, key)
	return nil
}

func (r *MemoryRouting) OnMessage(handler layers.MessageHandler) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.handler = handler
}

func (r *MemoryRouting) ParseMessage(raw []byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{
		Type:      "memory",
		Symbol:    "MEMORY",
		Timestamp: time.Now().UTC(),
		Data:      string(raw),
	}, nil
}

func (r *MemoryRouting) Emit(msg layers.NormalizedMessage) {
	r.mu.Lock()
	h := r.handler
	r.mu.Unlock()
	if h != nil {
		h(msg)
	}
}

type MemoryBusiness struct {
	mu      sync.Mutex
	count   int
	last    time.Time
	status  string
	metrics map[string]any
}

func NewMemoryBusiness() *MemoryBusiness {
	return &MemoryBusiness{
		status:  "ready",
		metrics: make(map[string]any),
	}
}

func (b *MemoryBusiness) Process(ctx context.Context, msg layers.NormalizedMessage) (layers.BusinessResult, error) {
	select {
	case <-ctx.Done():
		return layers.BusinessResult{}, ctx.Err()
	default:
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.count++
	b.last = time.Now().UTC()
	b.metrics["last_type"] = msg.Type
	return layers.BusinessResult{Success: true, Data: msg}, nil
}

func (b *MemoryBusiness) Validate(ctx context.Context, req layers.BusinessRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

func (b *MemoryBusiness) GetState() layers.BusinessState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return layers.BusinessState{
		Status:     b.status,
		Metrics:    map[string]any{"processed": b.count, "last_type": b.metrics["last_type"]},
		LastUpdate: b.last,
	}
}

type PassthroughFilter struct {
	name      string
	mu        sync.Mutex
	done      chan struct{}
	closeOnce sync.Once
}

func NewPassthroughFilter(name string) *PassthroughFilter {
	return &PassthroughFilter{name: name, done: make(chan struct{})}
}

func (f *PassthroughFilter) Apply(ctx context.Context, events <-chan layers.Event) (<-chan layers.Event, error) {
	out := make(chan layers.Event)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case <-f.done:
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				select {
				case out <- evt:
				case <-ctx.Done():
					return
				case <-f.done:
					return
				}
			}
		}
	}()
	return out, nil
}

func (f *PassthroughFilter) Name() string {
	return f.name
}

func (f *PassthroughFilter) Close() error {
	f.closeOnce.Do(func() {
		close(f.done)
	})
	return nil
}

func subscriptionKey(req layers.SubscriptionRequest) string {
	return req.Symbol + "|" + req.Type + "|" + req.Category + "|" + boolKey(req.IsPrivate)
}

func boolKey(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

var (
	_ layers.Connection = (*MemoryConnection)(nil)
	_ layers.Routing    = (*MemoryRouting)(nil)
	_ layers.Business   = (*MemoryBusiness)(nil)
	_ layers.Filter     = (*PassthroughFilter)(nil)
)
