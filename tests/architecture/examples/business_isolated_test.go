package examples

import (
	"context"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/layers"
	archmocks "github.com/coachpo/meltica/tests/architecture/mocks"
)

type sampleBusiness struct {
	routing layers.Routing
	state   layers.BusinessState
}

func newSampleBusiness(r layers.Routing) *sampleBusiness {
	b := &sampleBusiness{
		routing: r,
		state: layers.BusinessState{
			Status:     "initial",
			Metrics:    map[string]any{},
			LastUpdate: time.Now().UTC(),
		},
	}
	r.OnMessage(b.consume)
	return b
}

func (b *sampleBusiness) consume(msg layers.NormalizedMessage) {
	b.state.Metrics["last_symbol"] = msg.Symbol
	b.state.LastUpdate = time.Now().UTC()
}

func (b *sampleBusiness) Bootstrap(ctx context.Context, req layers.SubscriptionRequest) error {
	return b.routing.Subscribe(ctx, req)
}

func (b *sampleBusiness) Process(ctx context.Context, msg layers.NormalizedMessage) (layers.BusinessResult, error) {
	_ = ctx
	b.state.Status = "processed"
	b.state.Metrics["last_payload"] = msg.Data
	return layers.BusinessResult{Success: true, Data: msg.Data}, nil
}

func (b *sampleBusiness) Validate(context.Context, layers.BusinessRequest) error {
	return nil
}

func (b *sampleBusiness) GetState() layers.BusinessState {
	return b.state
}

func TestBusinessIsolatedWithMockRouting(t *testing.T) {
	ctx := context.Background()
	routing := archmocks.NewMockRouting()
	business := newSampleBusiness(routing)

	req := layers.SubscriptionRequest{Symbol: "ETHUSDT", Type: "trade"}
	if err := business.Bootstrap(ctx, req); err != nil {
		t.Fatalf("bootstrap failed: %v", err)
	}
	if len(routing.Subscriptions()) != 1 {
		t.Fatalf("expected a single subscription, got %d", len(routing.Subscriptions()))
	}

	routing.Emit(layers.NormalizedMessage{Symbol: "ETHUSDT", Type: "trade"})
	state := business.GetState()
	if state.Metrics["last_symbol"] != "ETHUSDT" {
		t.Fatalf("expected last_symbol metric to be set, got %#v", state.Metrics["last_symbol"])
	}

	result, err := business.Process(ctx, layers.NormalizedMessage{Symbol: "ETHUSDT", Data: map[string]any{"qty": 1}})
	if err != nil {
		t.Fatalf("process failed: %v", err)
	}
	if !result.Success {
		t.Fatal("expected successful business result")
	}
	if business.GetState().Status != "processed" {
		t.Fatalf("expected business status to be processed, got %s", business.GetState().Status)
	}
}
