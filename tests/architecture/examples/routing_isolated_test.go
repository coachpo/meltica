package examples

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/core/layers"
	archmocks "github.com/coachpo/meltica/tests/architecture/mocks"
)

type sampleRouting struct {
	conn    layers.Connection
	handler layers.MessageHandler
}

func newSampleRouting(conn layers.Connection) *sampleRouting {
	return &sampleRouting{conn: conn}
}

func (r *sampleRouting) Subscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	if err := r.conn.Connect(ctx); err != nil {
		return err
	}
	_ = req // subscription management elided for brevity
	return nil
}

func (r *sampleRouting) Unsubscribe(ctx context.Context, req layers.SubscriptionRequest) error {
	_ = ctx
	_ = req
	return r.conn.Close()
}

func (r *sampleRouting) OnMessage(handler layers.MessageHandler) {
	r.handler = handler
}

func (r *sampleRouting) ParseMessage(raw []byte) (layers.NormalizedMessage, error) {
	return layers.NormalizedMessage{Type: "raw", Data: raw}, nil
}

func (r *sampleRouting) emit(msg layers.NormalizedMessage) {
	if r.handler != nil {
		r.handler(msg)
	}
}

func TestRoutingIsolatedWithMockConnection(t *testing.T) {
	ctx := context.Background()
	conn := archmocks.NewMockConnection()
	router := newSampleRouting(conn)

	req := layers.SubscriptionRequest{Symbol: "BTCUSDT", Type: "trade"}
	if err := router.Subscribe(ctx, req); err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	if conn.ConnectCalls() != 1 {
		t.Fatalf("expected single Connect call, got %d", conn.ConnectCalls())
	}

	received := make(chan layers.NormalizedMessage, 1)
	router.OnMessage(func(msg layers.NormalizedMessage) {
		received <- msg
	})
	router.emit(layers.NormalizedMessage{Symbol: "BTCUSDT", Type: "trade"})

	select {
	case msg := <-received:
		if msg.Symbol != "BTCUSDT" {
			t.Fatalf("unexpected symbol: %s", msg.Symbol)
		}
	case <-ctx.Done():
		t.Fatal("expected message from routing handler")
	}

	if err := router.Unsubscribe(ctx, req); err != nil {
		t.Fatalf("unsubscribe failed: %v", err)
	}
	if conn.CloseCalls() != 1 {
		t.Fatalf("expected single Close call, got %d", conn.CloseCalls())
	}
}
