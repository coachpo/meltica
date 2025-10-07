package filter_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/marketdata/filter"
)


func TestCoordinatorBookStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bookCh := make(chan corestreams.BookEvent, 1)
	bookCh <- corestreams.BookEvent{Symbol: "BTC-USDT"}
	close(bookCh)
	errCh := make(chan error)
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Books: true},
		bookSources: []filter.BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   filter.FeedSelection{Books: true},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	select {
	case envelope, ok := <-stream.Events:
		if !ok {
			t.Fatalf("expected event from stream")
		}
		if envelope.Kind != filter.EventKindBook {
			t.Fatalf("expected book event, got %s", envelope.Kind)
		}
		if envelope.Symbol != "BTC-USDT" {
			t.Fatalf("expected symbol BTC-USDT, got %s", envelope.Symbol)
		}
		if envelope.Timestamp.IsZero() {
			t.Fatal("expected normalized timestamp")
		}
		if envelope.Book == nil {
			t.Fatal("expected book payload")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for book event")
	}
}

func TestCoordinatorPropagatesErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bookCh := make(chan corestreams.BookEvent)
	close(bookCh)
	errCh := make(chan error, 1)
	errCh <- errors.New("boom")
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Books: true},
		bookSources: []filter.BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   filter.FeedSelection{Books: true},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	select {
	case err, ok := <-stream.Errors:
		if !ok {
			t.Fatalf("expected error from stream")
		}
		if err == nil {
			t.Fatal("expected non-nil error")
		}
		if err.Error() != "filter pipeline: boom" {
			t.Fatalf("unexpected error message: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for filter error")
	}
}

func TestAggregationTrimsBookDepthAndSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bookCh := make(chan corestreams.BookEvent, 1)
	bookCh <- corestreams.BookEvent{
		Symbol: "BTC-USDT",
		Bids: []core.BookDepthLevel{
			{Qty: big.NewRat(1, 1)},
			{Qty: big.NewRat(2, 1)},
		},
		Asks: []core.BookDepthLevel{
			{Qty: big.NewRat(3, 1)},
			{Qty: big.NewRat(4, 1)},
		},
	}
	close(bookCh)
	errCh := make(chan error)
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Books: true},
		bookSources: []filter.BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols:         []string{"BTC-USDT"},
		Feeds:           filter.FeedSelection{Books: true},
		BookDepth:       1,
		EnableSnapshots: true,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	select {
	case evt := <-stream.Events:
		if evt.Book == nil {
			t.Fatal("expected book event")
		}
		if len(evt.Book.Bids) != 1 {
			t.Fatalf("expected 1 bid level, got %d", len(evt.Book.Bids))
		}
		if len(evt.Book.Asks) != 1 {
			t.Fatalf("expected 1 ask level, got %d", len(evt.Book.Asks))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for aggregated book")
	}

	if snapshot, ok := stream.Snapshot(filter.EventKindBook, "BTC-USDT"); !ok || snapshot.Book == nil {
		t.Fatalf("expected snapshot entry")
	}
}

func TestThrottleSkipsRapidEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeCh := make(chan corestreams.TradeEvent, 2)
	tradeCh <- corestreams.TradeEvent{Symbol: "ETH-USDT", Time: time.Unix(100, 0)}
	tradeCh <- corestreams.TradeEvent{Symbol: "ETH-USDT", Time: time.Unix(100, int64(500*time.Millisecond))}
	close(tradeCh)
	errCh := make(chan error)
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Trades: true},
		tradeSources: []filter.TradeSource{
			{Symbol: "ETH-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols:         []string{"ETH-USDT"},
		Feeds:           filter.FeedSelection{Trades: true},
		MinEmitInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	// First event should be emitted
	select {
	case <-stream.Events:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for throttled event")
	}

	// Second event should be throttled (not emitted within the interval)
	select {
	case evt, ok := <-stream.Events:
		if ok {
			t.Fatalf("expected second event to be throttled, got %+v", evt)
		}
		// Channel closed - this is acceptable with multi-source stage
	case <-time.After(100 * time.Millisecond):
		// No event received - throttling worked correctly
	}
}

type countingObserver struct {
	events int
	errors int
}

func (o *countingObserver) OnEvent(filter.EventEnvelope) { o.events++ }
func (o *countingObserver) OnError(error)                { o.errors++ }

func TestObserverReceivesCallbacks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeCh := make(chan corestreams.TradeEvent, 1)
	tradeCh <- corestreams.TradeEvent{Symbol: "BTC-USDT", Time: time.Unix(200, 0)}
	close(tradeCh)
	errCh := make(chan error, 1)
	errCh <- errors.New("boom")
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Trades: true},
		tradeSources: []filter.TradeSource{
			{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	observer := &countingObserver{}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols:  []string{"BTC-USDT"},
		Feeds:    filter.FeedSelection{Trades: true},
		Observer: observer,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	// With multi-source stage, channels don't close immediately
	// So we'll use a timeout to consume events
	timeout := time.NewTimer(2 * time.Second)
	defer timeout.Stop()

	for {
		select {
		case _, ok := <-stream.Events:
			if !ok {
				stream.Events = nil
			}
		case err, ok := <-stream.Errors:
			if ok && err != nil {
				// consume
			} else if !ok {
				stream.Errors = nil
			}
		case <-timeout.C:
			// Timeout reached, stop waiting
			stream.Events = nil
			stream.Errors = nil
		}
		if stream.Events == nil && stream.Errors == nil {
			break
		}
	}

	if observer.events == 0 {
		t.Fatal("expected observer to receive events")
	}
	if observer.errors == 0 {
		t.Fatal("expected observer to receive errors")
	}
}

func TestVWAPStageEmitsAnalytics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeCh := make(chan corestreams.TradeEvent, 2)
	tradeCh <- corestreams.TradeEvent{Symbol: "BTC-USDT", Price: big.NewRat(100, 1), Quantity: big.NewRat(2, 1), Time: time.Unix(300, 0)}
	tradeCh <- corestreams.TradeEvent{Symbol: "BTC-USDT", Price: big.NewRat(200, 1), Quantity: big.NewRat(1, 1), Time: time.Unix(301, 0)}
	close(tradeCh)
	errCh := make(chan error)
	close(errCh)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{Trades: true},
		tradeSources: []filter.TradeSource{
			{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols:    []string{"BTC-USDT"},
		Feeds:      filter.FeedSelection{Trades: true},
		EnableVWAP: true,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	var lastStats *filter.AnalyticsEvent

	for evt := range stream.Events {
		if evt.Kind == filter.EventKindVWAP {
			if evt.Stats == nil {
				t.Fatalf("missing analytics payload")
			}
			lastStats = evt.Stats
		}
	}
	for err := range stream.Errors {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if lastStats == nil {
		t.Fatalf("expected VWAP analytics event")
	}

	expected := big.NewRat(400, 3)
	if lastStats.VWAP == nil || lastStats.VWAP.Cmp(expected) != 0 {
		t.Fatalf("unexpected vwap: got %v want %v", lastStats.VWAP, expected)
	}
	if lastStats.TradeCount != 2 {
		t.Fatalf("unexpected trade count: %d", lastStats.TradeCount)
	}
}

func TestCoordinatorPassThroughTradeTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeCh := make(chan corestreams.TradeEvent, 1)
	tradeCh <- corestreams.TradeEvent{Symbol: "ETH-USDT", Time: time.Unix(1700000000, 0)}
	close(tradeCh)
	tradeErr := make(chan error)
	close(tradeErr)

	tickerCh := make(chan corestreams.TickerEvent, 1)
	tickerCh <- corestreams.TickerEvent{Symbol: "BNB-USDT", Time: time.Unix(1700000500, 0)}
	close(tickerCh)
	tickerErr := make(chan error)
	close(tickerErr)

	adapter := &stubAdapter{
		capabilities: filter.Capabilities{
			Trades:  true,
			Tickers: true,
		},
		tradeSources: []filter.TradeSource{
			{Symbol: "ETH-USDT", Events: tradeCh, Errors: tradeErr},
		},
		tickerSources: []filter.TickerSource{
			{Symbol: "BNB-USDT", Events: tickerCh, Errors: tickerErr},
		},
	}

	coordinator := filter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, filter.FilterRequest{
		Symbols: []string{"ETH-USDT", "BNB-USDT"},
		Feeds: filter.FeedSelection{
			Trades:  true,
			Tickers: true,
		},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	var receivedTrades, receivedTickers bool
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

	for receivedTrades == false || receivedTickers == false {
		select {
		case envelope, ok := <-stream.Events:
			if !ok {
				t.Fatalf("expected events before close")
			}
			switch envelope.Kind {
			case filter.EventKindTrade:
				if envelope.Trade == nil {
					t.Fatalf("expected trade payload")
				}
				receivedTrades = true
			case filter.EventKindTicker:
				if envelope.Ticker == nil {
					t.Fatalf("expected ticker payload")
				}
				receivedTickers = true
			default:
				t.Fatalf("unexpected event kind %s", envelope.Kind)
			}
		case <-timeout.C:
			t.Fatal("timed out waiting for trade/ticker events")
		}
	}
}
