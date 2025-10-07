package pipeline

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
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
		capabilities: Capabilities{Books: true},
		bookSources: []BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   FeedSelection{Books: true},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	select {
	case evt, ok := <-stream.Events:
		if !ok {
			t.Fatalf("expected event from stream")
		}
		payload, ok := evt.Payload.(BookPayload)
		if !ok || payload.Book == nil {
			t.Fatalf("expected book payload, got %T", evt.Payload)
		}
		if evt.Symbol != "BTC-USDT" {
			t.Fatalf("expected symbol BTC-USDT, got %s", evt.Symbol)
		}
		if evt.At.IsZero() {
			t.Fatal("expected normalized timestamp")
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
		capabilities: Capabilities{Books: true},
		bookSources: []BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols: []string{"BTC-USDT"},
		Feeds:   FeedSelection{Books: true},
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
		capabilities: Capabilities{Books: true},
		bookSources: []BookSource{
			{Symbol: "BTC-USDT", Events: bookCh, Errors: errCh},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols:         []string{"BTC-USDT"},
		Feeds:           FeedSelection{Books: true},
		BookDepth:       1,
		EnableSnapshots: true,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	select {
	case evt := <-stream.Events:
		payload, ok := evt.Payload.(BookPayload)
		if !ok || payload.Book == nil {
			t.Fatal("expected book event")
		}
		if len(payload.Book.Bids) != 1 {
			t.Fatalf("expected 1 bid level, got %d", len(payload.Book.Bids))
		}
		if len(payload.Book.Asks) != 1 {
			t.Fatalf("expected 1 ask level, got %d", len(payload.Book.Asks))
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for aggregated book")
	}

	if last, ok := stream.LastBookEvent("BTC-USDT"); !ok {
		t.Fatalf("expected cached book entry")
	} else if payload, ok := last.Payload.(BookPayload); !ok || payload.Book == nil {
		t.Fatalf("expected book payload, got %T", last.Payload)
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
		capabilities: Capabilities{Trades: true},
		tradeSources: []TradeSource{
			{Symbol: "ETH-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols:         []string{"ETH-USDT"},
		Feeds:           FeedSelection{Trades: true},
		MinEmitInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	// First event should be emitted
	select {
	case evt := <-stream.Events:
		if _, ok := evt.Payload.(TradePayload); !ok {
			t.Fatalf("expected trade payload, got %T", evt.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for throttled event")
	}

	// Second event should be throttled (not emitted within the interval)
	select {
	case evt, ok := <-stream.Events:
		if ok {
			t.Fatalf("expected second event to be throttled, got %+v", evt)
		}
	case <-time.After(100 * time.Millisecond):
		// No event received - throttling worked correctly
	}
}

type countingObserver struct {
	events int
	errors int
}

func (o *countingObserver) OnEvent(ClientEvent) { o.events++ }
func (o *countingObserver) OnError(error)       { o.errors++ }

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
		capabilities: Capabilities{Trades: true},
		tradeSources: []TradeSource{
			{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	observer := &countingObserver{}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols:  []string{"BTC-USDT"},
		Feeds:    FeedSelection{Trades: true},
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

	trades := []corestreams.TradeEvent{
		{Symbol: "BTC-USDT", Price: big.NewRat(100, 1), Quantity: big.NewRat(2, 1), Time: time.Unix(300, 0)},
		{Symbol: "BTC-USDT", Price: big.NewRat(200, 1), Quantity: big.NewRat(1, 1), Time: time.Unix(301, 0)},
	}

	tradeCh := make(chan corestreams.TradeEvent, len(trades))
	for _, trade := range trades {
		tradeCh <- trade
	}
	close(tradeCh)
	errCh := make(chan error)
	close(errCh)

	adapter := &stubAdapter{
		capabilities: Capabilities{Trades: true},
		tradeSources: []TradeSource{
			{Symbol: "BTC-USDT", Events: tradeCh, Errors: errCh},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols:    []string{"BTC-USDT"},
		Feeds:      FeedSelection{Trades: true},
		EnableVWAP: true,
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	events := stream.Events
	errors := stream.Errors
	targetTrades := int64(len(trades))
	timeout := time.NewTimer(time.Second)
	defer timeout.Stop()

outer:
	for {
		select {
		case evt, ok := <-events:
			if !ok {
				events = nil
				continue
			}
			payload, ok := evt.Payload.(AnalyticsPayload)
			if !ok || payload.Analytics == nil {
				continue
			}
			stats := payload.Analytics
			if stats.TradeCount < targetTrades {
				continue
			}
			if stats.TradeCount > targetTrades {
				t.Fatalf("unexpected trade count: %d", stats.TradeCount)
			}

			expected := big.NewRat(400, 3)
			if stats.VWAP == nil || stats.VWAP.Cmp(expected) != 0 {
				t.Fatalf("unexpected vwap: got %v want %v", stats.VWAP, expected)
			}
			break outer
		case err, ok := <-errors:
			if !ok {
				errors = nil
				continue
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-timeout.C:
			t.Fatal("timed out waiting for VWAP analytics")
		}
		if events == nil && errors == nil {
			t.Fatalf("stream closed before analytics event was received")
		}
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
		capabilities: Capabilities{
			Trades:  true,
			Tickers: true,
		},
		tradeSources: []TradeSource{
			{Symbol: "ETH-USDT", Events: tradeCh, Errors: tradeErr},
		},
		tickerSources: []TickerSource{
			{Symbol: "BNB-USDT", Events: tickerCh, Errors: tickerErr},
		},
	}

	coordinator := NewCoordinator(adapter, nil)
	defer coordinator.Close()

	stream, err := coordinator.Stream(ctx, PipelineRequest{
		Symbols: []string{"ETH-USDT", "BNB-USDT"},
		Feeds: FeedSelection{
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

	for !receivedTrades || !receivedTickers {
		select {
		case envelope, ok := <-stream.Events:
			if !ok {
				t.Fatalf("expected events before close")
			}
			switch payload := envelope.Payload.(type) {
			case TradePayload:
				if payload.Trade == nil {
					t.Fatalf("expected trade payload")
				}
				receivedTrades = true
			case TickerPayload:
				if payload.Ticker == nil {
					t.Fatalf("expected ticker payload")
				}
				receivedTickers = true
			default:
				t.Fatalf("unexpected event payload %T", payload)
			}
		case <-timeout.C:
			t.Fatal("timed out waiting for trade/ticker events")
		}
	}
}
