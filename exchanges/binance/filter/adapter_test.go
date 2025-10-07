package filter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	binancefilter "github.com/coachpo/meltica/exchanges/binance/filter"
	"github.com/coachpo/meltica/internal/numeric"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

type bookFixture struct {
	Symbol string     `json:"symbol"`
	Time   string     `json:"time"`
	Bids   [][]string `json:"bids"`
	Asks   [][]string `json:"asks"`
}

type fixtureSubscriber struct {
	events map[string][]corestreams.BookEvent
}

func newFixtureSubscriber(fixtures []corestreams.BookEvent) *fixtureSubscriber {
	evts := make(map[string][]corestreams.BookEvent)
	for _, evt := range fixtures {
		evts[evt.Symbol] = append(evts[evt.Symbol], evt)
	}
	return &fixtureSubscriber{events: evts}
}

func (f *fixtureSubscriber) BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	symbol = normalize(symbol)
	events := make(chan corestreams.BookEvent, len(f.events[symbol]))
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)

		for _, evt := range f.events[symbol] {
			select {
			case <-ctx.Done():
				return
			case events <- evt:
			}
		}
	}()

	return events, errs, nil
}

func TestBinanceAdapterWithFixtures(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fixtures := loadBookFixtures(t)
	subscriber := newFixtureSubscriber(fixtures)

	adapter, err := binancefilter.NewAdapter(subscriber, nil)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}
	defer adapter.Close()

	coordinator := mdfilter.NewCoordinator(adapter, nil)
	defer coordinator.Close()

	symbols := []string{"BTC-USDT", "ETH-USDT"}
	stream, err := coordinator.Stream(ctx, mdfilter.PipelineRequest{
		Symbols: symbols,
		Feeds:   mdfilter.FeedSelection{Books: true},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	defer stream.Close()

	expected := len(fixtures)
	received := 0
	seen := make(map[string]int)

	for received < expected {
		select {
		case evt, ok := <-stream.Events:
			if !ok {
				t.Fatalf("stream closed before receiving all fixtures")
			}
			payload, ok := evt.Payload.(mdfilter.BookPayload)
			if !ok || payload.Book == nil {
				t.Fatalf("expected book payload, got %T", evt.Payload)
			}
			seen[evt.Symbol]++
			received++
		case err, ok := <-stream.Errors:
			if ok && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		case <-ctx.Done():
			t.Fatalf("context cancelled before receiving fixtures: %v", ctx.Err())
		}
	}

	for _, symbol := range symbols {
		if seen[symbol] == 0 {
			t.Fatalf("expected events for symbol %s", symbol)
		}
	}
}

func loadBookFixtures(t *testing.T) []corestreams.BookEvent {
	t.Helper()

	path := filepath.Join("testdata", "orderbook_snapshots.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixtures: %v", err)
	}

	var raw []bookFixture
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal fixtures: %v", err)
	}

	out := make([]corestreams.BookEvent, 0, len(raw))
	for _, rec := range raw {
		tm, err := time.Parse(time.RFC3339, rec.Time)
		if err != nil {
			t.Fatalf("parse time %s: %v", rec.Time, err)
		}
		out = append(out, corestreams.BookEvent{
			Symbol: rec.Symbol,
			Bids:   depthFromFixture(rec.Bids),
			Asks:   depthFromFixture(rec.Asks),
			Time:   tm,
		})
	}
	return out
}

func depthFromFixture(levels [][]string) []core.BookDepthLevel {
	out := make([]core.BookDepthLevel, 0, len(levels))
	for _, pair := range levels {
		if len(pair) < 2 {
			continue
		}
		price, _ := numeric.Parse(pair[0])
		qty, _ := numeric.Parse(pair[1])
		out = append(out, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return out
}

func normalize(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	symbol = strings.ToUpper(symbol)
	return symbol
}

type stubSubscription struct {
	msgs chan core.Message
	errs chan error
}

func newStubSubscription(messages []core.Message, errs []error) *stubSubscription {
	msgCh := make(chan core.Message, len(messages))
	for _, msg := range messages {
		msgCh <- msg
	}
	close(msgCh)

	errCh := make(chan error, len(errs))
	for _, err := range errs {
		errCh <- err
	}
	close(errCh)

	return &stubSubscription{msgs: msgCh, errs: errCh}
}

func (s *stubSubscription) C() <-chan core.Message { return s.msgs }
func (s *stubSubscription) Err() <-chan error      { return s.errs }
func (s *stubSubscription) Close() error           { return nil }

type stubWS struct {
	public map[string]*stubSubscription
}

func (s *stubWS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("no topic provided")
	}
	sub, ok := s.public[topics[0]]
	if !ok {
		return nil, fmt.Errorf("subscription for topic %s missing", topics[0])
	}
	return sub, nil
}

func TestAdapterTradeAndTickerSources(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	tradeTopic := core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT")
	tickerTopic := core.MustCanonicalTopic(core.TopicTicker, "ETH-USDT")

	ws := &stubWS{
		public: map[string]*stubSubscription{
			tradeTopic: newStubSubscription([]core.Message{
				{
					Parsed: &corestreams.TradeEvent{
						Symbol: "BTC-USDT",
						Time:   time.Unix(1700, 0),
					},
				},
			}, nil),
			tickerTopic: newStubSubscription([]core.Message{
				{
					Parsed: &corestreams.TickerEvent{
						Symbol: "ETH-USDT",
						Time:   time.Unix(1701, 0),
					},
				},
			}, nil),
		},
	}

	adapter, err := binancefilter.NewAdapter(nil, ws)
	if err != nil {
		t.Fatalf("new adapter: %v", err)
	}

	tradeSources, err := adapter.TradeSources(ctx, []string{"BTC-USDT"})
	if err != nil {
		t.Fatalf("trade sources: %v", err)
	}
	if len(tradeSources) != 1 {
		t.Fatalf("expected 1 trade source, got %d", len(tradeSources))
	}
	select {
	case evt := <-tradeSources[0].Events:
		if evt.Symbol != "BTC-USDT" {
			t.Fatalf("expected trade symbol BTC-USDT, got %s", evt.Symbol)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for trade event")
	}

	tickerSources, err := adapter.TickerSources(ctx, []string{"ETH-USDT"})
	if err != nil {
		t.Fatalf("ticker sources: %v", err)
	}
	if len(tickerSources) != 1 {
		t.Fatalf("expected 1 ticker source, got %d", len(tickerSources))
	}
	select {
	case evt := <-tickerSources[0].Events:
		if evt.Symbol != "ETH-USDT" {
			t.Fatalf("expected ticker symbol ETH-USDT, got %s", evt.Symbol)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ticker event")
	}
}
