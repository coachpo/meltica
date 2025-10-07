package pipeline

import (
	"context"
	"testing"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
)

type stubAdapter struct {
	capabilities  Capabilities
	bookSources   []BookSource
	tradeSources  []TradeSource
	tickerSources []TickerSource
}

func (s *stubAdapter) Capabilities() Capabilities {
	return s.capabilities
}

func (s *stubAdapter) BookSources(ctx context.Context, symbols []string) ([]BookSource, error) {
	return s.bookSources, nil
}

func (s *stubAdapter) TradeSources(ctx context.Context, symbols []string) ([]TradeSource, error) {
	return s.tradeSources, nil
}

func (s *stubAdapter) TickerSources(ctx context.Context, symbols []string) ([]TickerSource, error) {
	return s.tickerSources, nil
}

func (s *stubAdapter) PrivateSources(ctx context.Context, auth *AuthContext) ([]PrivateSource, error) {
	return nil, nil
}

func (s *stubAdapter) ExecuteREST(ctx context.Context, req InteractionRequest) (<-chan Event, <-chan error, error) {
	events := make(chan Event)
	close(events)
	errors := make(chan error)
	close(errors)
	return events, errors, nil
}

func (s *stubAdapter) InitPrivateSession(ctx context.Context, auth *AuthContext) error {
	return nil
}

func (s *stubAdapter) Close() {}

func BenchmarkPipeline(b *testing.B) {
	run := func(name string, req PipelineRequest) {
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
					capabilities: Capabilities{Trades: true},
					tradeSources: []TradeSource{
						{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
					},
				}

				coord := NewCoordinator(adapter, nil)
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

	run("baseline", PipelineRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   FeedSelection{Trades: true},
	})
	run("with_snapshots", PipelineRequest{
		Symbols:         []string{"BTC-USDT"},
		Feeds:           FeedSelection{Trades: true},
		EnableSnapshots: true,
	})
}
