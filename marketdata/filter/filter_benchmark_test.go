package filter_test

import (
	"context"
	"testing"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/marketdata/filter"
)

type stubAdapter struct {
	capabilities  filter.Capabilities
	bookSources   []filter.BookSource
	tradeSources  []filter.TradeSource
	tickerSources []filter.TickerSource
}

func (s *stubAdapter) Capabilities() filter.Capabilities {
	return s.capabilities
}

func (s *stubAdapter) BookSources(ctx context.Context, symbols []string) ([]filter.BookSource, error) {
	return s.bookSources, nil
}

func (s *stubAdapter) TradeSources(ctx context.Context, symbols []string) ([]filter.TradeSource, error) {
	return s.tradeSources, nil
}

func (s *stubAdapter) TickerSources(ctx context.Context, symbols []string) ([]filter.TickerSource, error) {
	return s.tickerSources, nil
}

func (s *stubAdapter) PrivateSources(ctx context.Context, auth *filter.AuthContext) ([]filter.PrivateSource, error) {
	return nil, nil
}

func (s *stubAdapter) ExecuteREST(ctx context.Context, req filter.InteractionRequest) (<-chan filter.EventEnvelope, <-chan error, error) {
	events := make(chan filter.EventEnvelope)
	close(events)
	errors := make(chan error)
	close(errors)
	return events, errors, nil
}

func (s *stubAdapter) InitPrivateSession(ctx context.Context, auth *filter.AuthContext) error {
	return nil
}

func (s *stubAdapter) Close() {}

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

				coord := filter.NewCoordinator(adapter, nil)
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
