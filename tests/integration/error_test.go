package integration

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/coachpo/meltica/core/dispatcher"
	"github.com/coachpo/meltica/core/events"
	"github.com/coachpo/meltica/core/recycler"
)

func TestFanoutAggregatedErrorIncludesMetadata(t *testing.T) {

	eventPool := &sync.Pool{New: func() any { return &events.Event{} }}
	execPool := &sync.Pool{New: func() any { return &events.ExecReport{} }}
	recyclerMetrics := recycler.NewRecyclerMetrics(prometheus.NewRegistry())
	rec := recycler.NewRecycler(eventPool, execPool, recyclerMetrics)
	fanout := dispatcher.NewFanout(rec, eventPool, nil, 4)

	subscribers := []dispatcher.Subscriber{
		{ID: "ok", Deliver: func(ctx context.Context, ev *events.Event) error {
			if ev != nil {
				rec.RecycleEvent(ev)
			}
			return nil
		}},
		{ID: "fail", Deliver: func(ctx context.Context, ev *events.Event) error {
			if ev != nil {
				rec.RecycleEvent(ev)
			}
			return context.Canceled
		}},
	}

	original := &events.Event{Kind: events.KindExecReport, TraceID: "trace-integration", RoutingVersion: 42}
	err := fanout.Dispatch(context.Background(), original, subscribers)
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation error wrapped, got %v", err)
	}
	var aggErr *dispatcher.FanoutError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected fanout error, got %T", err)
	}
	if aggErr.TraceID != original.TraceID {
		t.Fatalf("expected trace id %q, got %q", original.TraceID, aggErr.TraceID)
	}
	if aggErr.EventKind != original.Kind {
		t.Fatalf("expected event kind %v, got %v", original.Kind, aggErr.EventKind)
	}
	if aggErr.RoutingVersion != original.RoutingVersion {
		t.Fatalf("expected routing version %d, got %d", original.RoutingVersion, aggErr.RoutingVersion)
	}
	if aggErr.SubscriberCount != len(subscribers) {
		t.Fatalf("expected subscriber count %d, got %d", len(subscribers), aggErr.SubscriberCount)
	}
	foundFailed := false
	for _, id := range aggErr.FailedSubscribers {
		if id == "fail" {
			foundFailed = true
			break
		}
	}
	if !foundFailed {
		t.Fatalf("expected failed subscriber list to include fail, got %v", aggErr.FailedSubscribers)
	}
}
