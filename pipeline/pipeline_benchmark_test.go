package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
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

func (s *stubAdapter) ExchangeName() core.ExchangeName { return "" }

const benchmarkEvents = 512

func BenchmarkPipeline(b *testing.B) {
	cases := []struct {
		name string
		req  PipelineRequest
	}{
		{
			name: "baseline",
			req: PipelineRequest{
				Symbols: []string{"BTC-USDT"},
				Feeds:   FeedSelection{Trades: true},
			},
		},
		{
			name: "with_snapshots",
			req: PipelineRequest{
				Symbols:         []string{"BTC-USDT"},
				Feeds:           FeedSelection{Trades: true},
				EnableSnapshots: true,
			},
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				ctx, cancel := context.WithCancel(context.Background())

				tradeCh := make(chan corestreams.TradeEvent, benchmarkEvents)
				for j := 0; j < benchmarkEvents; j++ {
					tradeCh <- corestreams.TradeEvent{
						Symbol: "BTC-USDT",
						Time:   time.Unix(int64(j), 0),
					}
				}
				close(tradeCh)
				errCh := make(chan error, 1)
				close(errCh)

				adapter := &stubAdapter{
					capabilities: Capabilities{Trades: true},
					tradeSources: []TradeSource{
						{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
					},
				}

				coord := NewCoordinator(adapter, nil)
				stream, err := coord.Stream(ctx, tc.req)
				if err != nil {
					b.Fatalf("stream: %v", err)
				}
				count := 0
				for range stream.Events {
					count++
				}
				if count == 0 {
					b.Fatalf("expected events for %s", tc.name)
				}
				stream.Close()
				coord.Close()
				cancel()
			}
		})
	}
}

var benchmarkClientEvent ClientEvent

func BenchmarkHotPathClientEventFromPipeline(b *testing.B) {
	trade := corestreams.TradeEvent{Symbol: "BTC-USDT"}
	evt := Event{
		Transport: TransportPublicWS,
		Symbol:    "BTC-USDT",
		Payload:   TradePayload{Trade: &trade},
		Metadata: map[string]any{
			metadataKeySourceFeed:     "trade",
			metadataKeySourceSymbol:   "BTC-USDT",
			metadataKeySourceExchange: "binance",
			metadataKeySourceSequence: uint64(42),
		},
	}

	b.ReportAllocs()
	if allocs := testing.AllocsPerRun(1, func() {
		benchmarkClientEvent = clientEventFromPipeline(evt)
	}); allocs != 0 {
		b.Fatalf("client event conversion allocated %.0f objects", allocs)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkClientEvent = clientEventFromPipeline(evt)
	}
}
