package router

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBackpressureUnderLoad(t *testing.T) {
	metrics := NewRoutingMetrics()
	dispatcher := NewRouterDispatcher(context.Background(), metrics)
	defer dispatcher.Shutdown()

	proc := newMockProcessor("trade")
	reg := &ProcessorRegistration{
		MessageTypeID: "trade",
		Processor:     proc,
		Status:        ProcessorStatusAvailable,
	}
	tradeInbox := dispatcher.Bind("trade", reg)

	const totalMessages = 10
	done := make(chan struct{})
	go func() {
		for i := 0; i < totalMessages; i++ {
			select {
			case raw := <-tradeInbox:
				time.Sleep(15 * time.Millisecond)
				if _, err := reg.Processor.Process(context.Background(), raw); err != nil {
					panic(err)
				}
			}
		}
		close(done)
	}()

	start := time.Now()
	for i := 0; i < totalMessages; i++ {
		payload := []byte(fmt.Sprintf("payload-%d", i))
		if err := dispatcher.Dispatch("trade", payload); err != nil {
			t.Fatalf("dispatch failed: %v", err)
		}
	}
	elapsed := time.Since(start)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("processor did not drain messages in time")
	}

	require.Equal(t, totalMessages, proc.Calls())
	snapshot := metrics.Snapshot()
	require.GreaterOrEqual(t, snapshot.BackpressureEvents, uint64(totalMessages-1))
	require.Equal(t, uint64(0), snapshot.ChannelDepth["trade"])
	minExpected := time.Duration(totalMessages-1) * 15 * time.Millisecond
	require.GreaterOrEqual(t, elapsed, minExpected)
}
