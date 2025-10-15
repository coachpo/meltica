package pool

import (
	"context"
	"fmt"

	"github.com/coachpo/meltica/internal/schema"
)

// BorrowCanonicalEvent acquires a CanonicalEvent from the pool manager.
func BorrowCanonicalEvent(ctx context.Context, pools *PoolManager) (*schema.Event, error) {
	if pools == nil {
		return nil, fmt.Errorf("canonical event pool unavailable")
	}
	obj, err := pools.Get(ctx, "CanonicalEvent")
	if err != nil {
		return nil, err
	}
	evt, ok := obj.(*schema.Event)
	if !ok {
		pools.Put("CanonicalEvent", obj)
		return nil, fmt.Errorf("canonical event pool: unexpected type %T", obj)
	}
	evt.Reset()
	return evt, nil
}

// RecycleCanonicalEvent returns the event to the pool manager.
func RecycleCanonicalEvent(pools *PoolManager, evt *schema.Event) {
	if pools == nil || evt == nil {
		return
	}
	pools.Put("CanonicalEvent", evt)
}

// RecycleCanonicalEvents recycles a slice of events.
func RecycleCanonicalEvents(pools *PoolManager, events []*schema.Event) {
	for _, evt := range events {
		RecycleCanonicalEvent(pools, evt)
	}
}
