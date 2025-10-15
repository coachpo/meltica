package integration

import (
	"context"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/schema"
)

// Example integration test - tests multiple components together
func TestDatabusPublishSubscribeFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	bus := databus.NewMemoryBus(databus.MemoryConfig{
		BufferSize:    10,
		FanoutWorkers: 2,
	})
	defer bus.Close()

	// Subscribe
	subID, eventsCh, err := bus.Subscribe(ctx, schema.EventTypeTrade)
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	defer bus.Unsubscribe(subID)

	// Publish
	testEvent := &schema.Event{
		EventID:  "test-1",
		Provider: "test-provider",
		Type:     schema.EventTypeTrade,
		Symbol:   "BTC-USD",
	}

	if err := bus.Publish(ctx, testEvent); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	// Verify delivery
	select {
	case received := <-eventsCh:
		if received.EventID != testEvent.EventID {
			t.Errorf("expected EventID %s, got %s", testEvent.EventID, received.EventID)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}
