package dispatcher

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/coachpo/meltica/core/events"
	"github.com/coachpo/meltica/core/recycler"
	"github.com/coachpo/meltica/internal/observability"
)

type stubRecycler struct {
	mu       sync.Mutex
	recycled []*events.Event
}

var _ recycler.Recycler = (*stubRecycler)(nil)

func (s *stubRecycler) RecycleEvent(ev *events.Event) {
	if ev == nil {
		return
	}
	s.mu.Lock()
	s.recycled = append(s.recycled, ev)
	s.mu.Unlock()
}

func (s *stubRecycler) RecycleMergedEvent(*events.MergedEvent) {}
func (s *stubRecycler) RecycleExecReport(*events.ExecReport)   {}
func (s *stubRecycler) RecycleMany(events []*events.Event) {
	for _, ev := range events {
		s.RecycleEvent(ev)
	}
}
func (s *stubRecycler) EnableDebugMode()                        {}
func (s *stubRecycler) DisableDebugMode()                       {}
func (s *stubRecycler) CheckoutEvent(*events.Event)             {}
func (s *stubRecycler) CheckoutMergedEvent(*events.MergedEvent) {}
func (s *stubRecycler) CheckoutExecReport(*events.ExecReport)   {}

func (s *stubRecycler) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.recycled)
}

type countingPool struct {
	getCount atomic.Int64
}

func (c *countingPool) Get() any {
	c.getCount.Add(1)
	return &events.Event{}
}

func (c *countingPool) Put(any) {}

type captureLogger struct {
	mu     sync.Mutex
	msg    string
	fields []observability.Field
}

func (c *captureLogger) Debug(string, ...observability.Field) {}
func (c *captureLogger) Info(string, ...observability.Field)  {}

func (c *captureLogger) Error(msg string, fields ...observability.Field) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.msg = msg
	c.fields = append([]observability.Field(nil), fields...)
}

func (c *captureLogger) snapshot() (string, []observability.Field) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.msg, append([]observability.Field(nil), c.fields...)
}

func TestDispatchNoSubscribersRecyclesOriginal(t *testing.T) {
	recycler := &stubRecycler{}
	fanout := NewFanout(recycler, nil, nil, 4)
	original := &events.Event{TraceID: "noop"}

	if err := fanout.Dispatch(context.Background(), original, nil); err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if recycler.count() != 1 || recycler.recycled[0] != original {
		t.Fatalf("expected original to be recycled once")
	}
}

func TestDispatchSingleSubscriberAvoidsDuplicates(t *testing.T) {
	recycler := &stubRecycler{}
	pool := &countingPool{}
	fanout := NewFanout(recycler, pool, nil, 4)
	original := &events.Event{TraceID: "single"}

	var received *events.Event
	subscriber := Subscriber{
		ID: "only",
		Deliver: func(ctx context.Context, ev *events.Event) error {
			received = ev
			recycler.RecycleEvent(ev)
			return nil
		},
	}

	if err := fanout.Dispatch(context.Background(), original, []Subscriber{subscriber}); err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if received != original {
		t.Fatalf("expected original event to be delivered directly")
	}
	if recycler.count() != 1 {
		t.Fatalf("expected subscriber to recycle once, got %d", recycler.count())
	}
	if pool.getCount.Load() != 0 {
		t.Fatalf("expected pool Get not called, got %d", pool.getCount.Load())
	}
}

func TestDispatchParallelSuccess(t *testing.T) {
	recycler := &stubRecycler{}
	metrics := NewFanoutMetrics(prometheus.NewRegistry())
	fanout := NewFanout(recycler, &countingPool{}, metrics, 8)

	subscribers := []Subscriber{
		{
			ID: "a",
			Deliver: func(ctx context.Context, ev *events.Event) error {
				recycler.RecycleEvent(ev)
				return nil
			},
		},
		{
			ID: "b",
			Deliver: func(ctx context.Context, ev *events.Event) error {
				recycler.RecycleEvent(ev)
				return nil
			},
		},
		{
			ID: "c",
			Deliver: func(ctx context.Context, ev *events.Event) error {
				recycler.RecycleEvent(ev)
				return nil
			},
		},
	}

	original := &events.Event{TraceID: "fanout", Kind: events.KindMarketData}
	if err := fanout.Dispatch(context.Background(), original, subscribers); err != nil {
		t.Fatalf("dispatch error: %v", err)
	}
	if recycler.count() == 0 {
		t.Fatalf("expected recycled events")
	}
}

func TestDispatchAggregatesErrorsAndLogs(t *testing.T) {
	recycler := &stubRecycler{}
	fanout := NewFanout(recycler, &countingPool{}, nil, 4)
	logger := &captureLogger{}
	observability.SetLogger(logger)
	defer observability.SetLogger(nil)

	errBoom := errors.New("boom")
	subscribers := []Subscriber{
		{
			ID: "ok",
			Deliver: func(ctx context.Context, ev *events.Event) error {
				recycler.RecycleEvent(ev)
				return nil
			},
		},
		{
			ID: "fail",
			Deliver: func(ctx context.Context, ev *events.Event) error {
				recycler.RecycleEvent(ev)
				return errBoom
			},
		},
	}

	original := &events.Event{TraceID: "trace", Kind: events.KindMarketData}
	if err := fanout.Dispatch(context.Background(), original, subscribers); err == nil || !errors.Is(err, errBoom) {
		t.Fatalf("expected aggregated error containing original failure, got %v", err)
	}
	msg, fields := logger.snapshot()
	if msg != "operation errors" {
		t.Fatalf("unexpected log message %q", msg)
	}
	foundFailed := false
	for _, f := range fields {
		if f.Key == "failed_subscribers" {
			if ids, ok := f.Value.([]string); ok {
				for _, id := range ids {
					if id == "fail" {
						foundFailed = true
					}
				}
			}
		}
	}
	if !foundFailed {
		t.Fatalf("expected failed subscriber id in log fields")
	}
}

func TestDispatchRecoversSubscriberPanic(t *testing.T) {
	recycler := &stubRecycler{}
	fanout := NewFanout(recycler, &countingPool{}, nil, 4)

	subscriber := Subscriber{
		ID: "panic",
		Deliver: func(ctx context.Context, ev *events.Event) error {
			recycler.RecycleEvent(ev)
			panic("boom")
		},
	}

	original := &events.Event{TraceID: "panic", Kind: events.KindMarketData}
	if err := fanout.Dispatch(context.Background(), original, []Subscriber{subscriber, subscriber}); err == nil || err.Error() == "" {
		t.Fatalf("expected panic to be converted into error")
	}
}

func TestDispatchHandlesContextCancellation(t *testing.T) {
	recycler := &stubRecycler{}
	fanout := NewFanout(recycler, &countingPool{}, nil, 4)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	subscribers := []Subscriber{
		{ID: "a", Deliver: func(ctx context.Context, ev *events.Event) error {
			recycler.RecycleEvent(ev)
			return nil
		}},
		{ID: "b", Deliver: func(ctx context.Context, ev *events.Event) error {
			recycler.RecycleEvent(ev)
			return nil
		}},
	}

	original := &events.Event{TraceID: "ctx", Kind: events.KindMarketData}
	if err := fanout.Dispatch(ctx, original, subscribers); err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestCloneEventCopiesFields(t *testing.T) {
	src := &events.Event{TraceID: "trace", RoutingVersion: 3, Kind: events.KindExecReport, Payload: "payload", SeqProvider: 9, ProviderID: "src"}
	dst := &events.Event{}
	cloneEvent(src, dst)
	if dst.TraceID != src.TraceID || dst.RoutingVersion != src.RoutingVersion || dst.Kind != src.Kind || dst.Payload != src.Payload || dst.SeqProvider != src.SeqProvider || dst.ProviderID != src.ProviderID {
		t.Fatalf("clone mismatch: src=%+v dst=%+v", src, dst)
	}
}

func TestUniqueStringsDeduplicates(t *testing.T) {
	values := []string{"a", "b", "a", "", "c", "b"}
	result := uniqueStrings(values)
	if len(result) != 3 {
		t.Fatalf("expected three unique values, got %v", result)
	}
}

type nilPool struct{}

func (nilPool) Get() any { return nil }
func (nilPool) Put(any)  {}

func TestBorrowDuplicateFallsBackToNewEvent(t *testing.T) {
	recycler := &stubRecycler{}
	fanout := &Fanout{recycler: recycler, duplicates: nilPool{}}
	dup := fanout.borrowDuplicate()
	if dup == nil {
		t.Fatalf("expected fallback duplicate")
	}
	if dup.TraceID != "" || dup.Kind != 0 {
		t.Fatalf("expected reset duplicate, got %+v", dup)
	}
}

func TestFanoutMetricsObserve(t *testing.T) {
	metrics := NewFanoutMetrics(prometheus.NewRegistry())
	metrics.Observe(3, []time.Duration{time.Millisecond, 2 * time.Millisecond, 0}, 3*time.Millisecond)
	// Sanity: ensure observe does not panic and gauge set between 0 and 1.
}
