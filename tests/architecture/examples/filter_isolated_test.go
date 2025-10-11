package examples

import (
	"context"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/layers"
	archmocks "github.com/coachpo/meltica/tests/architecture/mocks"
)

type sampleFilter struct {
	business layers.Business
}

func newSampleFilter(b layers.Business) *sampleFilter {
	return &sampleFilter{business: b}
}

func (f *sampleFilter) Apply(ctx context.Context, events <-chan layers.Event) (<-chan layers.Event, error) {
	output := make(chan layers.Event)
	go func() {
		defer close(output)
		for {
			select {
			case <-ctx.Done():
				return
			case evt, ok := <-events:
				if !ok {
					return
				}
				_, _ = f.business.Process(ctx, layers.NormalizedMessage{Type: evt.Type, Data: evt.Payload})
				select {
				case output <- evt:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return output, nil
}

func (f *sampleFilter) Name() string { return "sample_filter" }

func (f *sampleFilter) Close() error { return nil }

func TestFilterIsolatedWithMockBusiness(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	business := archmocks.NewMockBusiness()
	filter := newSampleFilter(business)

	input := make(chan layers.Event, 1)
	input <- layers.Event{Type: "trade", Payload: map[string]any{"price": 100}}
	close(input)

	output, err := filter.Apply(ctx, input)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	select {
	case evt, ok := <-output:
		if !ok {
			t.Fatal("expected event from filter output")
		}
		if evt.Type != "trade" {
			t.Fatalf("unexpected event type: %s", evt.Type)
		}
	case <-time.After(50 * time.Millisecond):
		t.Fatal("expected filter to forward event promptly")
	}

	if len(business.ProcessCalls()) != 1 {
		t.Fatalf("expected business Process to be invoked once, got %d", len(business.ProcessCalls()))
	}

	if err := filter.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}
