package recycler

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

const (
	defaultAcquireTimeout = 100 * time.Millisecond
)

// Interface exposes the recycling operations required by dispatcher fan-out
// and consumer wrappers.
type Interface interface {
	BorrowEvent(ctx context.Context) (*schema.Event, error)
	CheckoutEvent(*schema.Event)
	RecycleEvent(*schema.Event)
	RecycleMany([]*schema.Event)
}

// Manager implements Interface backed by the shared pool manager.
type Manager struct {
	pools *pool.PoolManager
}

// NewManager constructs a recycler that returns objects through the pool manager.
func NewManager(pools *pool.PoolManager) *Manager {
	return &Manager{pools: pools}
}

// BorrowEvent acquires a CanonicalEvent from the pool manager with a bounded timeout.
func (m *Manager) BorrowEvent(ctx context.Context) (*schema.Event, error) {
	if m == nil || m.pools == nil {
		ev := new(schema.Event)
		return ev, nil
	}
	obj, err := m.acquire(ctx, "CanonicalEvent")
	if err != nil {
		return nil, err
	}
	event, ok := obj.(*schema.Event)
	if !ok {
		m.pools.Put("CanonicalEvent", obj)
		return nil, fmt.Errorf("recycler: unexpected object type %T", obj)
	}
	event.Reset()
	return event, nil
}

// CheckoutEvent marks the event as checked out from the pool for double-Put detection.
func (*Manager) CheckoutEvent(evt *schema.Event) {
	if evt == nil {
		return
	}
	evt.SetReturned(false)
}

// RecycleEvent resets and returns the event to the pool.
func (m *Manager) RecycleEvent(evt *schema.Event) {
	if evt == nil {
		return
	}
	evt.Reset()
	if m != nil && m.pools != nil {
		m.pools.Put("CanonicalEvent", evt)
	}
}

// RecycleMany returns a batch of events to the pool.
func (m *Manager) RecycleMany(events []*schema.Event) {
	for _, evt := range events {
		m.RecycleEvent(evt)
	}
}

func (m *Manager) acquire(ctx context.Context, poolName string) (pool.PooledObject, error) {
	if m == nil || m.pools == nil {
		return nil, fmt.Errorf("recycler: pool manager unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultAcquireTimeout)
		defer cancel()
	}
	obj, err := m.pools.Get(ctx, poolName)
	if err != nil {
		return nil, fmt.Errorf("recycler: acquire %s: %w", poolName, err)
	}
	return obj, nil
}
