package filter_test

import (
	"context"
	"testing"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/marketdata/filter"
)

func BenchmarkPipeline(b *testing.B) {
	run := func(name string, req filter.FilterRequest) {
		b.Run(name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithCancel(context.Background())

				tradeCh := make(chan corestreams.TradeEvent, 1024)
				errCh := make(chan error)
				for j := 0; j < 1024; j++ {
					tradeCh <- corestreams.TradeEvent{
						Symbol: "BTC-USDT",
						Time:   time.Unix(int64(j), 0),
					}
				}
				close(tradeCh)
				close(errCh)

				adapter := &stubAdapter{
					capabilities: filter.Capabilities{Trades: true},
					tradeSources: []filter.TradeSource{
						{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
					},
				}

				coord := filter.NewCoordinator(adapter)
				stream, err := coord.Stream(ctx, req)
				if err != nil {
					b.Fatalf("stream: %v", err)
				}
				b.ResetTimer()
				count := 0
				for range stream.Events {
					count++
				}
				b.StopTimer()
				stream.Close()
				coord.Close()
				cancel()
			}
		})
	}

	run("baseline", filter.FilterRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   filter.FeedSelection{Trades: true},
	})
	run("with_snapshots", filter.FilterRequest{
		Symbols:         []string{"BTC-USDT"},
		Feeds:           filter.FeedSelection{Trades: true},
		EnableSnapshots: true,
	})
}
