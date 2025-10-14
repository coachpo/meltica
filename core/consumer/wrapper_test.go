package consumer

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/coachpo/meltica/core/events"
)

type recyclerStub struct {
	mu    sync.Mutex
	count int
}

func (r *recyclerStub) RecycleEvent(ev *events.Event) {
	if ev == nil {
		return
	}
	r.mu.Lock()
	r.count++
	r.mu.Unlock()
}

func (r *recyclerStub) RecycleMergedEvent(*events.MergedEvent)  {}
func (r *recyclerStub) RecycleExecReport(*events.ExecReport)    {}
func (r *recyclerStub) RecycleMany([]*events.Event)             {}
func (r *recyclerStub) EnableDebugMode()                        {}
func (r *recyclerStub) DisableDebugMode()                       {}
func (r *recyclerStub) CheckoutEvent(*events.Event)             {}
func (r *recyclerStub) CheckoutMergedEvent(*events.MergedEvent) {}
func (r *recyclerStub) CheckoutExecReport(*events.ExecReport)   {}

func (r *recyclerStub) recycled() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count
}

func TestWrapperInvokeRecordsMetricsAndRecycles(t *testing.T) {
	metrics := NewConsumerMetrics(prometheus.NewRegistry())
	recycler := &recyclerStub{}
	wrapper := NewWrapper("consumer", recycler, metrics)
	ev := &events.Event{Kind: events.KindMarketData, RoutingVersion: 10}

	if err := wrapper.Invoke(context.Background(), ev, func(ctx context.Context, event *events.Event) error {
		time.Sleep(1 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatalf("invoke returned error: %v", err)
	}

	if recycler.recycled() != 1 {
		t.Fatalf("expected event recycled once, got %d", recycler.recycled())
	}
	if got := testutil.ToFloat64(metrics.invocations.WithLabelValues("consumer")); got != 1 {
		t.Fatalf("expected invocation counter to be 1, got %f", got)
	}
}

func TestWrapperFiltersStaleMarketData(t *testing.T) {
	metrics := NewConsumerMetrics(prometheus.NewRegistry())
	recycler := &recyclerStub{}
	wrapper := NewWrapper("consumer", recycler, metrics)
	wrapper.UpdateMinVersion(100)

	called := atomic.Bool{}
	ev := &events.Event{Kind: events.KindMarketData, RoutingVersion: 50}
	if err := wrapper.Invoke(context.Background(), ev, func(ctx context.Context, event *events.Event) error {
		called.Store(true)
		return nil
	}); err != nil {
		t.Fatalf("invoke returned error: %v", err)
	}
	if called.Load() {
		t.Fatalf("expected lambda not to be called for stale market data")
	}
	if got := testutil.ToFloat64(metrics.filtered.WithLabelValues("consumer")); got != 1 {
		t.Fatalf("expected filtered counter to increment, got %f", got)
	}
}

func TestWrapperConvertsPanicToError(t *testing.T) {
	metrics := NewConsumerMetrics(prometheus.NewRegistry())
	recycler := &recyclerStub{}
	wrapper := NewWrapper("consumer", recycler, metrics)
	event := &events.Event{Kind: events.KindMarketData}

	panicErr := wrapper.Invoke(context.Background(), event, func(ctx context.Context, ev *events.Event) error {
		panic("boom")
	})
	if panicErr == nil || panicErr.Error() == "" {
		t.Fatalf("expected panic to be converted into error")
	}
	if recycler.recycled() != 1 {
		t.Fatalf("expected event recycled after panic")
	}
	if got := testutil.ToFloat64(metrics.panics.WithLabelValues("consumer")); got != 1 {
		t.Fatalf("expected panic counter increment, got %f", got)
	}
}

func TestWrapperShouldProcessCriticalEvents(t *testing.T) {
	wrapper := NewWrapper("consumer", &recyclerStub{}, nil)
	wrapper.UpdateMinVersion(100)
	critical := &events.Event{Kind: events.KindExecReport, RoutingVersion: 1}
	if !wrapper.ShouldProcess(critical) {
		t.Fatalf("expected critical events to bypass filter")
	}
}

func TestRegistryRegistersAndInvokes(t *testing.T) {
	metrics := NewConsumerMetrics(prometheus.NewRegistry())
	recycler := &recyclerStub{}
	wrapper := NewWrapper("consumer", recycler, metrics)
	reg := NewRegistry()
	reg.Register(wrapper)

	calls := atomic.Int64{}
	ev := &events.Event{Kind: events.KindMarketData}
	if err := reg.Invoke(context.Background(), "consumer", ev, func(ctx context.Context, event *events.Event) error {
		calls.Add(1)
		return nil
	}); err != nil {
		t.Fatalf("registry invoke error: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("expected consumer lambda invoked once, got %d", calls.Load())
	}

	reg.UpdateMinVersion("consumer", 10)
	if wrapper.minAcceptVersion.Load() != 10 {
		t.Fatalf("expected min version updated via registry")
	}
}

func TestNilMetricsNoop(t *testing.T) {
	var metrics *ConsumerMetrics
	metrics.ObserveInvocation("c")
	metrics.ObserveDuration("c", time.Second)
	metrics.ObservePanic("c")
	metrics.ObserveFiltered("c")
}
