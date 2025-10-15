package recycler

import (
    "context"
    "fmt"
    "log"
    "time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

const (
	acquireWarnDelay = 250 * time.Millisecond
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
        return nil, fmt.Errorf("recycler: pool manager unavailable")
    }
    requestCtx := ctx
    if requestCtx == nil {
        requestCtx = context.Background()
    }
    start := time.Now()
    obj, err := m.pools.Get(requestCtx, "CanonicalEvent")
    if err != nil {
        return nil, fmt.Errorf("recycler: acquire canonical event: %w", err)
    }
    if waited := time.Since(start); waited >= acquireWarnDelay {
        log.Printf("recycler: waited %s for CanonicalEvent pool", waited)
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
    if m == nil || evt == nil {
        return
    }
    if m.pools == nil {
        return
    }
    evt.Reset()
    m.pools.Put("CanonicalEvent", evt)
}

// RecycleMany returns a batch of events to the pool.
func (m *Manager) RecycleMany(events []*schema.Event) {
	for _, evt := range events {
		m.RecycleEvent(evt)
	}
}
