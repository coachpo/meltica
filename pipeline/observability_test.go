package pipeline

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

type instrumentedTestAdapter struct {
	hooks AdapterHooks
}

func (a *instrumentedTestAdapter) Capabilities() Capabilities {
	return Capabilities{Trades: true, Tickers: true}
}

func (a *instrumentedTestAdapter) BookSources(context.Context, []string) ([]BookSource, error) {
	return nil, nil
}

func (a *instrumentedTestAdapter) TradeSources(ctx context.Context, symbols []string) ([]TradeSource, error) {
	events := make(chan corestreams.TradeEvent, len(symbols))
	errs := make(chan error, 1)
	for _, sym := range symbols {
		if a.hooks.OnSubscribe != nil {
			a.hooks.OnSubscribe(ctx, AdapterEvent{Exchange: a.ExchangeName(), Feed: "trades", Symbol: sym})
		}
		events <- corestreams.TradeEvent{Symbol: sym, Time: time.Now()}
	}
	close(events)
	close(errs)
	sources := make([]TradeSource, 0, len(symbols))
	for _, sym := range symbols {
		sources = append(sources, TradeSource{Symbol: sym, Events: events, Errors: errs})
	}
	return sources, nil
}

func (a *instrumentedTestAdapter) TickerSources(ctx context.Context, symbols []string) ([]TickerSource, error) {
	sources := make([]TickerSource, 0, len(symbols))
	for _, sym := range symbols {
		if a.hooks.OnSubscribe != nil {
			a.hooks.OnSubscribe(ctx, AdapterEvent{Exchange: a.ExchangeName(), Feed: "tickers", Symbol: sym})
		}
		events := make(chan corestreams.TickerEvent)
		errs := make(chan error)
		close(events)
		close(errs)
		sources = append(sources, TickerSource{Symbol: sym, Events: events, Errors: errs})
	}
	return sources, nil
}

func (a *instrumentedTestAdapter) PrivateSources(context.Context, *AuthContext) ([]PrivateSource, error) {
	return nil, nil
}

func (a *instrumentedTestAdapter) ExecuteREST(context.Context, InteractionRequest) (<-chan Event, <-chan error, error) {
	return nil, nil, nil
}

func (a *instrumentedTestAdapter) InitPrivateSession(context.Context, *AuthContext) error { return nil }

func (a *instrumentedTestAdapter) Close() {}

func (a *instrumentedTestAdapter) ExchangeName() core.ExchangeName {
	return core.ExchangeName("test-exchange")
}

func (a *instrumentedTestAdapter) SetAdapterHooks(h AdapterHooks) { a.hooks = h }

func TestObservabilityHooksPropagation(t *testing.T) {
	adapter := &instrumentedTestAdapter{}
	facade := NewInteractionFacade(adapter, nil)

	var (
		startCalled    bool
		subscribeCount int
		closeDur       time.Duration
	)
	closeCh := make(chan struct{}, 1)

	hooks := ObservabilityHooks{
		OnStreamStart: func(context.Context, PipelineRequest) {
			startCalled = true
		},
		OnStreamClose: func(_ context.Context, _ PipelineRequest, dur time.Duration) {
			closeDur = dur
			closeCh <- struct{}{}
		},
		Adapter: AdapterHooks{
			OnSubscribe: func(context.Context, AdapterEvent) {
				subscribeCount++
			},
		},
	}

	facade.SetObservability(hooks)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := facade.SubscribePublic(ctx, []string{"BTC-USDT"})
	if err != nil {
		t.Fatalf("SubscribePublic returned error: %v", err)
	}

	// Drain a single event to ensure pipeline spins up.
	select {
	case <-stream.Events:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for event")
	}

	stream.Close()

	select {
	case <-closeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("observability close hook not invoked")
	}

	if !startCalled {
		t.Fatal("expected OnStreamStart to be called")
	}
	if subscribeCount == 0 {
		t.Fatal("expected adapter OnSubscribe hook to fire")
	}
	if closeDur <= 0 {
		t.Fatalf("expected positive close duration, got %s", closeDur)
	}
}

type failingAdapter struct{}

func (f *failingAdapter) Capabilities() Capabilities { return Capabilities{} }
func (f *failingAdapter) BookSources(context.Context, []string) ([]BookSource, error) {
	return nil, nil
}
func (f *failingAdapter) TradeSources(context.Context, []string) ([]TradeSource, error) {
	return nil, nil
}
func (f *failingAdapter) TickerSources(context.Context, []string) ([]TickerSource, error) {
	return nil, nil
}
func (f *failingAdapter) PrivateSources(context.Context, *AuthContext) ([]PrivateSource, error) {
	return nil, nil
}
func (f *failingAdapter) ExecuteREST(context.Context, InteractionRequest) (<-chan Event, <-chan error, error) {
	return nil, nil, nil
}
func (f *failingAdapter) InitPrivateSession(context.Context, *AuthContext) error { return nil }
func (f *failingAdapter) Close()                                                 {}
func (f *failingAdapter) ExchangeName() core.ExchangeName                        { return core.ExchangeName("fail") }

func TestObservabilityStreamError(t *testing.T) {
	adapter := &failingAdapter{}
	facade := NewInteractionFacade(adapter, nil)

	var (
		once        sync.Once
		observedErr error
		wg          sync.WaitGroup
	)

	hooks := ObservabilityHooks{
		OnStreamError: func(_ context.Context, _ PipelineRequest, err error) {
			once.Do(func() {
				observedErr = err
				wg.Done()
			})
		},
	}

	facade.SetObservability(hooks)

	wg.Add(1)
	_, err := facade.SubscribePublic(context.Background(), []string{"BTC-USDT"})
	if err == nil {
		t.Fatal("expected capability validation error")
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OnStreamError")
	}

	if observedErr == nil {
		t.Fatal("expected observed error to be recorded")
	}
}
