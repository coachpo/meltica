package router

import (
	"context"
	"testing"
	"time"
)

func BenchmarkRoutingPipeline(b *testing.B) {
	rt, err := InitializeRouter()
	if err != nil {
		b.Fatalf("initialize router: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher := NewRouterDispatcher(ctx, rt.metrics)
	defer dispatcher.Shutdown()

	tradeReg := rt.Lookup("trade")
	if tradeReg == nil {
		b.Fatalf("trade registration missing")
	}
	orderReg := rt.Lookup("orderbook")
	if orderReg == nil {
		b.Fatalf("orderbook registration missing")
	}

	tradeInbox := dispatcher.Bind("trade", tradeReg)
	orderInbox := dispatcher.Bind("orderbook", orderReg)

	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	go func() {
		for {
			select {
			case <-workerCtx.Done():
				return
			case raw := <-tradeInbox:
				if _, err := tradeReg.Processor.Process(ctx, raw); err != nil {
					panic(err)
				}
			}
		}
	}()

	go func() {
		for {
			select {
			case <-workerCtx.Done():
				return
			case raw := <-orderInbox:
				if _, err := orderReg.Processor.Process(ctx, raw); err != nil {
					panic(err)
				}
			}
		}
	}()

	messages := [][]byte{
		[]byte(`{"type":"trade","price":"101.25","quantity":"0.5","side":"buy","taker":true,"trade_id":"bench-1"}`),
		[]byte(`{"type":"book","snapshot":false,"bids":[{"price":"201.10","quantity":"1"}],"asks":[{"price":"202.20","quantity":"1"}]}`),
		[]byte(`{"type":"trade","price":"99.75","quantity":"0.7","side":"sell","taker":false,"trade_id":"bench-2"}`),
		[]byte(`{"type":"book","snapshot":false,"bids":[{"price":"205.50","quantity":"2"}],"asks":[{"price":"206.40","quantity":"3"}]}`),
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.SetBytes(int64(len(messages[0])))

	for i := 0; i < b.N; i++ {
		msg := messages[i%len(messages)]
		messageType, err := rt.Detect(msg)
		if err != nil {
			b.Fatalf("detect message: %v", err)
		}
		if reg := rt.Lookup(messageType); reg == nil {
			b.Fatalf("lookup missing for %s", messageType)
		}
		if err := dispatcher.Dispatch(messageType, msg); err != nil {
			b.Fatalf("dispatch message: %v", err)
		}
	}

	b.StopTimer()
	workerCancel()
	time.Sleep(10 * time.Millisecond)
}
