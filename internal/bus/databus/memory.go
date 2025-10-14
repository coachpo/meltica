package databus

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	concpool "github.com/sourcegraph/conc/pool"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/internal/recycler"
	"github.com/coachpo/meltica/internal/schema"
)

// MemoryBus is an in-memory implementation of the data bus.
type MemoryBus struct {
	cfg MemoryConfig

	ctx    context.Context
	cancel context.CancelFunc
	rec    recycler.Interface

	mu           sync.RWMutex
	subscribers  map[schema.EventType]map[SubscriptionID]*subscriber
	shutdownOnce sync.Once
	nextID       uint64
	duplicates   sync.Pool
	workers      int
}

type subscriber struct {
	ctx    context.Context
	cancel context.CancelFunc
	ch     chan *schema.Event
	once   sync.Once
}

// NewMemoryBus constructs a memory-backed data bus.
func NewMemoryBus(cfg MemoryConfig) *MemoryBus {
	cfg = cfg.normalize()
	ctx, cancel := context.WithCancel(context.Background())
	bus := new(MemoryBus)
	bus.cfg = cfg
	bus.ctx = ctx
	bus.cancel = cancel
	bus.rec = cfg.Recycler
	bus.subscribers = make(map[schema.EventType]map[SubscriptionID]*subscriber)
	bus.workers = cfg.FanoutWorkers
	bus.duplicates = sync.Pool{
		New: func() any {
			return &schema.Event{} //nolint:exhaustruct
		},
	}
	return bus
}

// Publish fan-outs the event to all subscribers of its type.
func (b *MemoryBus) Publish(ctx context.Context, evt *schema.Event) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if evt == nil {
		return nil
	}
	if evt.Type == "" {
		return errs.New("databus/publish", errs.CodeInvalid, errs.WithMessage("event type required"))
	}

	// Snapshot subscribers to avoid holding lock during delivery.
	b.mu.RLock()
	subscribers := make([]*subscriber, 0, len(b.subscribers[evt.Type]))
	for _, sub := range b.subscribers[evt.Type] {
		subscribers = append(subscribers, sub)
	}
	b.mu.RUnlock()

	if len(subscribers) == 0 {
		return nil
	}

	return b.dispatch(ctx, subscribers, evt)
}

// Subscribe registers for events of the given type and returns a subscription ID and channel.
func (b *MemoryBus) Subscribe(ctx context.Context, typ schema.EventType) (SubscriptionID, <-chan *schema.Event, error) {
	if typ == "" {
		return "", nil, errs.New("databus/subscribe", errs.CodeInvalid, errs.WithMessage("event type required"))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)

	sub := new(subscriber)
	sub.ctx = ctx
	sub.cancel = cancel
	sub.ch = make(chan *schema.Event, b.cfg.BufferSize)

	id := SubscriptionID(fmt.Sprintf("sub-%d", atomic.AddUint64(&b.nextID, 1)))

	b.mu.Lock()
	if _, ok := b.subscribers[typ]; !ok {
		b.subscribers[typ] = make(map[SubscriptionID]*subscriber)
	}
	b.subscribers[typ][id] = sub
	b.mu.Unlock()

	go b.observe(typ, id, sub)
	return id, sub.ch, nil
}

// Unsubscribe removes the subscription and closes the channel.
func (b *MemoryBus) Unsubscribe(id SubscriptionID) {
	if id == "" {
		return
	}
	b.mu.Lock()
	for typ, subs := range b.subscribers {
		if sub, ok := subs[id]; ok {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.subscribers, typ)
			}
			b.mu.Unlock()
			sub.close()
			return
		}
	}
	b.mu.Unlock()
}

// Close shuts down the bus and all subscriptions.
func (b *MemoryBus) Close() {
	b.shutdownOnce.Do(func() {
		b.cancel()
		b.mu.Lock()
		for typ, subs := range b.subscribers {
			for id, sub := range subs {
				if sub != nil {
					sub.close()
				}
				delete(subs, id)
			}
			delete(b.subscribers, typ)
		}
		b.mu.Unlock()
	})
}

func (b *MemoryBus) observe(typ schema.EventType, id SubscriptionID, sub *subscriber) {
	<-sub.ctx.Done()
	b.mu.Lock()
	subs := b.subscribers[typ]
	if subs != nil {
		if stored, ok := subs[id]; ok && stored == sub {
			delete(subs, id)
			if len(subs) == 0 {
				delete(b.subscribers, typ)
			}
		}
	}
	b.mu.Unlock()
	sub.close()
}

func (b *MemoryBus) deliver(ctx context.Context, sub *subscriber, evt *schema.Event) error {
	if err := sub.ctx.Err(); err != nil {
		return fmt.Errorf("subscriber context: %w", err)
	}
	dup := b.cloneEvent(ctx, evt)
	if dup == nil {
		return errs.New("databus/publish", errs.CodeUnavailable, errs.WithMessage("duplicate acquisition failed"))
	}
	select {
	case <-b.ctx.Done():
		b.recycle(dup)
		return errs.New("databus/publish", errs.CodeUnavailable, errs.WithMessage("bus closed"))
	case <-ctx.Done():
		b.recycle(dup)
		return fmt.Errorf("deliver context: %w", ctx.Err())
	case <-sub.ctx.Done():
		b.recycle(dup)
		return nil
	case sub.ch <- dup:
		return nil
	default:
		b.recycle(dup)
		return errs.New("databus/publish", errs.CodeUnavailable, errs.WithMessage("subscriber buffer full"))
	}
}

func (b *MemoryBus) dispatch(ctx context.Context, subs []*subscriber, evt *schema.Event) error {
	if len(subs) == 0 {
		return nil
	}
	workerLimit := b.workers
	if workerLimit <= 0 {
		workerLimit = 1
	}
	p := concpool.New().WithMaxGoroutines(workerLimit)
	errCh := make(chan error, len(subs))
	for _, sub := range subs {
		if sub == nil {
			continue
		}
		target := sub
		p.Go(func() {
			err := b.deliver(ctx, target, evt)
			if err != nil {
				errCh <- err
			}
		})
	}
	p.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *MemoryBus) cloneEvent(ctx context.Context, src *schema.Event) *schema.Event {
	if src == nil {
		return nil
	}
	if b.rec != nil {
		dup, err := b.rec.BorrowEvent(ctx)
		if err == nil && dup != nil {
			b.rec.CheckoutEvent(dup)
			copyEvent(dup, src)
			return dup
		}
	}
	value := b.duplicates.Get()
	dup, _ := value.(*schema.Event)
	if dup == nil {
		dup = &schema.Event{} //nolint:exhaustruct
	}
	copyEvent(dup, src)
	return dup
}

func (b *MemoryBus) recycle(evt *schema.Event) {
	if evt == nil {
		return
	}
	if b.rec != nil {
		b.rec.RecycleEvent(evt)
		return
	}
	evt.Reset()
	b.duplicates.Put(evt)
}

func copyEvent(dst, src *schema.Event) {
	if dst == nil || src == nil {
		return
	}
	clone := schema.CloneEvent(src)
	if clone == nil {
		return
	}
	*dst = *clone
}

func (s *subscriber) close() {
	s.once.Do(func() {
		s.cancel()
		close(s.ch)
	})
}
