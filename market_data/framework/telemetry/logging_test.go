package telemetry

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
)

func TestEmitterBroadcastsEvents(t *testing.T) {
	emitter := NewEmitter()
	var mu sync.Mutex
	received := make([]Event, 0)
	unsub := emitter.Subscribe(func(evt Event) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})
	event := Event{Kind: EventMessageProcessed, SessionID: "sess", Outcome: stream.OutcomeAck}
	emitter.Emit(event)
	unsub()
	emitter.Emit(Event{Kind: EventMessageProcessed})
	mu.Lock()
	defer mu.Unlock()
	require.Len(t, received, 1)
	require.Equal(t, "sess", received[0].SessionID)
	require.False(t, received[0].Timestamp.IsZero())
}

func TestEmitterClonesMetadata(t *testing.T) {
	emitter := NewEmitter()
	metadata := map[string]string{"channel": "BTC-USDT"}
	receiver := make(chan Event, 1)
	emitter.Subscribe(func(evt Event) { receiver <- evt })
	emitter.Emit(Event{Kind: EventHandlerError, Metadata: metadata})
	evt := <-receiver
	require.Equal(t, "BTC-USDT", evt.Metadata["channel"])
	metadata["channel"] = "ETH-USD"
	require.Equal(t, "BTC-USDT", evt.Metadata["channel"])
}

func TestEmitterNilSubscriber(t *testing.T) {
	emitter := NewEmitter()
	emitter.Subscribe(nil)
	emitter.Emit(Event{Timestamp: time.Now()})
}
