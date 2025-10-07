package main

import (
	"context"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	binanceplugin "github.com/coachpo/meltica/exchanges/binance/plugin"
	"github.com/coachpo/meltica/internal/numeric"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

type fakeExchange struct {
	tradeSub  core.Subscription
	tickerSub core.Subscription
	bookEvent corestreams.BookEvent
}

func (f *fakeExchange) Name() string                            { return string(binanceplugin.Name) }
func (f *fakeExchange) Capabilities() core.ExchangeCapabilities { return 0 }
func (f *fakeExchange) SupportedProtocolVersion() string        { return "1" }
func (f *fakeExchange) Close() error                            { return nil }

func (f *fakeExchange) WS() core.WS { return f }
func (f *fakeExchange) BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	events := make(chan corestreams.BookEvent, 1)
	events <- f.bookEvent
	close(events)
	errs := make(chan error)
	close(errs)
	return events, errs, nil
}

func (f *fakeExchange) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, nil
	}
	switch topics[0] {
	case core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT"):
		return f.tradeSub, nil
	case core.MustCanonicalTopic(core.TopicTicker, "BTC-USDT"):
		return f.tickerSub, nil
	default:
		return f.tradeSub, nil
	}
}

func (f *fakeExchange) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	return nil, core.ErrNotSupported
}

type fakeSubscription struct {
	events chan core.Message
	errs   chan error
}

func (s *fakeSubscription) C() <-chan core.Message { return s.events }
func (s *fakeSubscription) Err() <-chan error      { return s.errs }
func (s *fakeSubscription) Close() error           { return nil }

func TestStartMarketFilter(t *testing.T) {
	tradeQty, ok := numeric.Parse("1")
	if !ok {
		t.Fatal("failed to parse qty")
	}
	tradePrice, ok := numeric.Parse("100")
	if !ok {
		t.Fatal("failed to parse price")
	}
	tradeMsgs := make(chan core.Message, 1)
	tradeMsgs <- core.Message{Parsed: &corestreams.TradeEvent{Symbol: "BTC-USDT", Quantity: tradeQty, Price: tradePrice, Time: time.Unix(1, 0)}}
	close(tradeMsgs)
	tradeErrs := make(chan error)
	close(tradeErrs)

	bid, ok := numeric.Parse("99")
	if !ok {
		t.Fatal("failed to parse bid")
	}
	ask, ok := numeric.Parse("101")
	if !ok {
		t.Fatal("failed to parse ask")
	}
	tickerMsgs := make(chan core.Message, 1)
	tickerMsgs <- core.Message{Parsed: &corestreams.TickerEvent{Symbol: "BTC-USDT", Bid: bid, Ask: ask, Time: time.Unix(1, 0)}}
	close(tickerMsgs)
	tickerErrs := make(chan error)
	close(tickerErrs)

	book := corestreams.BookEvent{
		Symbol: "BTC-USDT",
		Bids:   make([]core.BookDepthLevel, 0, 10),
		Asks:   make([]core.BookDepthLevel, 0, 10),
		Time:   time.Unix(1, 0),
	}
	for i := 0; i < 10; i++ {
		qty, ok := numeric.Parse("1")
		if !ok {
			t.Fatal("failed to parse qty")
		}
		priceBid, ok := numeric.Parse("100")
		if !ok {
			t.Fatal("failed to parse price")
		}
		priceAsk, ok := numeric.Parse("101")
		if !ok {
			t.Fatal("failed to parse price")
		}
		book.Bids = append(book.Bids, core.BookDepthLevel{Qty: qty, Price: priceBid})
		book.Asks = append(book.Asks, core.BookDepthLevel{Qty: qty, Price: priceAsk})
	}

	exch := &fakeExchange{
		tradeSub:  &fakeSubscription{events: tradeMsgs, errs: tradeErrs},
		tickerSub: &fakeSubscription{events: tickerMsgs, errs: tickerErrs},
		bookEvent: book,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	coord, stream, err := startMarketPipeline(ctx, exch, []string{"BTC-USDT"}, defaultBookDepth, 0, true, false)
	if err != nil {
		t.Fatalf("startMarketPipeline: %v", err)
	}
	defer coord.Close()
	defer stream.Close()

	receivedBooks := 0
	receivedTrades := 0
	receivedTickers := 0

	// Wait for a short time to receive events
	eventCtx, eventCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer eventCancel()

loop:
	for {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				stream.Events = nil
				continue
			}
			switch payload := evt.Payload.(type) {
			case mdfilter.TradePayload:
				receivedTrades++
			case mdfilter.TickerPayload:
				receivedTickers++
			case mdfilter.BookPayload:
				receivedBooks++
				if payload.Book == nil {
					t.Fatal("expected book payload")
				}
				expectedDepth := 10
				if len(payload.Book.Bids) != expectedDepth {
					t.Fatalf("expected book with %d levels, got %d", expectedDepth, len(payload.Book.Bids))
				}
			}
		case err, ok := <-stream.Errors:
			if !ok {
				stream.Errors = nil
				continue
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-eventCtx.Done():
			// Timeout reached, break out of the loop
			break loop
		}

		if (receivedTrades >= 1 && receivedTickers >= 1 && receivedBooks >= 1) ||
			(stream.Events == nil && stream.Errors == nil) {
			break loop
		}
	}

	if receivedTrades != 1 || receivedTickers != 1 || receivedBooks != 1 {
		t.Fatalf("unexpected counts trades=%d tickers=%d books=%d", receivedTrades, receivedTickers, receivedBooks)
	}

	if _, ok := stream.LastTradeEvent("BTC-USDT"); !ok {
		t.Fatal("expected cached trade entry")
	}
}
