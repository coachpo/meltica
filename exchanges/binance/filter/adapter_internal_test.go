package filter

import (
	"context"
	"errors"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/bridge"
	"github.com/coachpo/meltica/pipeline"
)

func TestUniqueSymbols(t *testing.T) {
	input := []string{" btc-usd ", "", "BTC-USD", "eth-usd", "ETH-USD "}
	got := uniqueSymbols(input)
	require.Equal(t, []string{"BTC-USD", "ETH-USD"}, got)
}

func TestNormalize(t *testing.T) {
	require.Equal(t, "BTC-USD", normalize(" btc-usd "))
	require.Equal(t, "", normalize("   "))
}

func TestConvertPrivateMessageOrder(t *testing.T) {
	avg := big.NewRat(105, 1)
	filled := big.NewRat(2, 1)
	msg := core.Message{
		Parsed: &corestreams.OrderEvent{
			Symbol:      "BTC-USD",
			OrderID:     "42",
			Status:      core.OrderFilled,
			FilledQty:   filled,
			AvgPrice:    avg,
			Quantity:    nil,
			Price:       nil,
			Side:        core.SideBuy,
			Type:        core.TypeLimit,
			TimeInForce: core.GTC,
		},
	}
	event, err := convertPrivateMessage(msg)
	require.NoError(t, err)
	payload, ok := event.Payload.(pipeline.OrderPayload)
	require.True(t, ok)
	require.NotNil(t, payload.Order)
	require.Equal(t, "BTC-USD", payload.Order.Symbol)
	require.Equal(t, "42", payload.Order.OrderID)
	require.Equal(t, "buy", payload.Order.Side)
	require.Equal(t, avg.RatString(), payload.Order.Price.RatString())
	require.Equal(t, filled.RatString(), payload.Order.Quantity.RatString())
}

func TestConvertPrivateMessageBalance(t *testing.T) {
	available := big.NewRat(10, 1)
	msg := core.Message{
		Parsed: &corestreams.BalanceEvent{
			Balances: []core.Balance{{
				Asset:     "usd",
				Available: available,
			}},
		},
	}
	event, err := convertPrivateMessage(msg)
	require.NoError(t, err)
	payload, ok := event.Payload.(pipeline.AccountPayload)
	require.True(t, ok)
	require.NotNil(t, payload.Account)
	require.Equal(t, "usd", payload.Account.Symbol)
	require.Equal(t, available.RatString(), payload.Account.Balance.RatString())
}

func TestConvertPrivateMessageUnknownAndErrors(t *testing.T) {
	unknownPayload := struct{ Field string }{Field: "value"}
	event, err := convertPrivateMessage(core.Message{Parsed: unknownPayload})
	require.NoError(t, err)
	_, ok := event.Payload.(pipeline.UnknownPayload)
	require.True(t, ok)

	_, err = convertPrivateMessage(core.Message{})
	require.Error(t, err)

	var order *corestreams.OrderEvent
	_, err = convertPrivateMessage(core.Message{Parsed: order})
	require.Error(t, err)

	_, err = convertPrivateMessage(core.Message{Parsed: &corestreams.BalanceEvent{}})
	require.Error(t, err)
}

func TestMatchesPrivateStream(t *testing.T) {
	orderPayload := pipeline.OrderPayload{Order: &pipeline.OrderEvent{}}
	accountPayload := pipeline.AccountPayload{Account: &pipeline.AccountEvent{}}
	require.True(t, matchesPrivateStream("orders", orderPayload))
	require.False(t, matchesPrivateStream("orders", accountPayload))
	require.True(t, matchesPrivateStream("account", accountPayload))
	require.True(t, matchesPrivateStream("", pipeline.UnknownPayload{}))
}

func TestBuildPrivateErrorEvent(t *testing.T) {
	err := errors.New("boom")
	raw := []byte("payload")
	event := buildPrivateErrorEvent(err, raw)
	require.Equal(t, pipeline.TransportPrivateWS, event.Transport)
	p, ok := event.Payload.(pipeline.UnknownPayload)
	require.True(t, ok)
	require.Contains(t, p.Detail, "error")
	require.Equal(t, "payload", event.Metadata["raw"])
}

func TestCloneRat(t *testing.T) {
	require.Nil(t, cloneRat(nil))
	src := big.NewRat(5, 2)
	clone := cloneRat(src)
	require.Equal(t, src.RatString(), clone.RatString())
	require.False(t, clone == src)
}

func TestAdapterIsRetryableError(t *testing.T) {
	a := &Adapter{}
	policy := &pipeline.RetryPolicy{RetryableErrors: []string{"temporarily"}}
	require.True(t, a.isRetryableError(errors.New("temporarily unavailable"), policy))
	require.True(t, a.isRetryableError(errors.New("timeout while dialing"), policy))
	require.False(t, a.isRetryableError(errors.New("permanent failure"), policy))
}

func TestAdapterCreateResponseEvent(t *testing.T) {
	a := &Adapter{}
	req := pipeline.InteractionRequest{Method: "GET", Path: "/foo", Symbol: "BTC", CorrelationID: "abc"}
	event := a.createResponseEvent(req, map[string]string{"ok": "1"}, nil)
	p, ok := event.Payload.(pipeline.RestResponsePayload)
	require.True(t, ok)
	require.Equal(t, 200, p.Response.StatusCode)
	require.Nil(t, p.Response.Error)

	err := errors.New("failed")
	event = a.createResponseEvent(req, map[string]string{"ok": "1"}, err)
	p = event.Payload.(pipeline.RestResponsePayload)
	require.Equal(t, 500, p.Response.StatusCode)
	require.Equal(t, err, p.Response.Error)
}

func TestAdapterExecuteWithRetry(t *testing.T) {
	dispatcher := &retryDispatcher{responses: []error{errors.New("timeout"), nil}}
	a := &Adapter{restRouter: dispatcher}
	req := pipeline.InteractionRequest{}
	policy := &pipeline.RetryPolicy{
		MaxAttempts:       3,
		BaseDelay:         pipeline.Duration(time.Millisecond),
		MaxDelay:          pipeline.Duration(2 * time.Millisecond),
		BackoffMultiplier: 1,
		RetryableErrors:   []string{"timeout"},
	}
	req.RetryPolicy = policy
	require.NoError(t, a.executeWithRetry(context.Background(), req, nil))
	require.Equal(t, 2, dispatcher.Calls())

	dispatcher = &retryDispatcher{responses: []error{errors.New("fatal")}}
	a = &Adapter{restRouter: dispatcher}
	req.RetryPolicy = policy
	require.Error(t, a.executeWithRetry(context.Background(), req, nil))
	require.Equal(t, 1, dispatcher.Calls())
}

func TestAdapterInitAndCloseSessionManager(t *testing.T) {
	dispatcher := &stubDispatcher{createKeys: []string{"key"}}
	sm := NewSessionManager(dispatcher, pipeline.SessionConfig{})
	a := &Adapter{sessionMgr: sm}
	require.NoError(t, a.InitPrivateSession(context.Background(), &pipeline.AuthContext{}))
	require.Equal(t, 1, dispatcher.CreateCalls())

	a.Close()
	require.Equal(t, 1, dispatcher.CloseCalls())
}

func TestAdapterInitPrivateSessionWithoutManager(t *testing.T) {
	a := &Adapter{}
	require.Error(t, a.InitPrivateSession(context.Background(), &pipeline.AuthContext{}))
}

func TestNewAdapterValidation(t *testing.T) {
	_, err := NewAdapter(nil, nil)
	require.Error(t, err)

	adapter, err := NewAdapter(&stubBookSubscriber{}, nil)
	require.NoError(t, err)
	require.NotNil(t, adapter)
}

func TestNewAdapterWithPrivateValidation(t *testing.T) {
	_, err := NewAdapterWithPrivate(nil, nil, nil, nil)
	require.Error(t, err)

	adapter, err := NewAdapterWithPrivate(nil, nil, &stubPrivateSubscriber{}, nil)
	require.NoError(t, err)
	require.NotNil(t, adapter)
}

func TestNewAdapterWithRESTSessionManager(t *testing.T) {
	adapter, err := NewAdapterWithREST(nil, nil, &stubPrivateSubscriber{}, &retryDispatcher{})
	require.NoError(t, err)
	require.NotNil(t, adapter.sessionMgr)
}

func TestAdapterCapabilities(t *testing.T) {
	a := &Adapter{books: &stubBookSubscriber{}, ws: &stubPublicSubscriber{}, privateWS: &stubPrivateSubscriber{}, restRouter: &retryDispatcher{}}
	caps := a.Capabilities()
	require.True(t, caps.Books)
	require.True(t, caps.Trades)
	require.True(t, caps.Tickers)
	require.True(t, caps.PrivateStreams)
	require.True(t, caps.RESTEndpoints)
}

func TestBookSourcesUniqueAndHooks(t *testing.T) {
	ctx := context.Background()
	bookSub := &stubBookSubscriber{}
	adapter, err := NewAdapter(bookSub, nil)
	require.NoError(t, err)
	rec := &eventRecorder{}
	adapter.SetAdapterHooks(pipeline.AdapterHooks{
		OnSubscribe: rec.addSubscribe,
	})
	sources, err := adapter.BookSources(ctx, []string{"btc-usd", "BTC-USD", ""})
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Equal(t, []string{"BTC-USD"}, bookSub.Calls())
	require.Len(t, rec.Subscribed(), 1)
}

func TestBookSourcesErrorEmitsHook(t *testing.T) {
	ctx := context.Background()
	bookSub := &stubBookSubscriber{err: errors.New("failure")}
	adapter, err := NewAdapter(bookSub, nil)
	require.NoError(t, err)
	rec := &eventRecorder{}
	adapter.SetAdapterHooks(pipeline.AdapterHooks{
		OnError: rec.addError,
	})
	_, err = adapter.BookSources(ctx, []string{"BTC-USD"})
	require.Error(t, err)
	require.Len(t, rec.Errors(), 1)
}

func TestTradeSourcesForwarding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := newStubSubscription()
	pub := &stubPublicSubscriber{subscription: sub}
	a := &Adapter{ws: pub}
	rec := &eventRecorder{}
	a.SetAdapterHooks(pipeline.AdapterHooks{
		OnSubscribe: rec.addSubscribe,
		OnError:     rec.addError,
	})
	sources, err := a.TradeSources(ctx, []string{"btc-usd", "btc-usd"})
	require.NoError(t, err)
	require.Len(t, sources, 1)
	require.Len(t, rec.Subscribed(), 1)
	sub.events <- core.Message{Parsed: &corestreams.TradeEvent{Symbol: "ignored"}}
	trade := <-sources[0].Events
	require.Equal(t, "BTC-USD", trade.Symbol)
	close(sub.events)
	close(sub.errs)
	require.Eventually(t, func() bool { return sub.IsClosed() }, 100*time.Millisecond, 5*time.Millisecond)
}

func TestTradeSourcesSubscribeError(t *testing.T) {
	ctx := context.Background()
	pub := &stubPublicSubscriber{err: errors.New("subscribe failed")}
	a := &Adapter{ws: pub}
	rec := &eventRecorder{}
	a.SetAdapterHooks(pipeline.AdapterHooks{OnError: rec.addError})
	_, err := a.TradeSources(ctx, []string{"BTC-USD"})
	require.Error(t, err)
	require.Len(t, rec.Errors(), 1)
}

func TestTickerSourcesForwarding(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub := newStubSubscription()
	pub := &stubPublicSubscriber{subscription: sub}
	a := &Adapter{ws: pub}
	sources, err := a.TickerSources(ctx, []string{"eth-usd"})
	require.NoError(t, err)
	require.Len(t, sources, 1)
	sub.events <- core.Message{Parsed: &corestreams.TickerEvent{Symbol: "ignored"}}
	ticker := <-sources[0].Events
	require.Equal(t, "ETH-USD", ticker.Symbol)
	close(sub.events)
	close(sub.errs)
	require.Eventually(t, func() bool { return sub.IsClosed() }, 100*time.Millisecond, 5*time.Millisecond)
}

func TestExecuteRESTSuccess(t *testing.T) {
	dispatcher := &restDispatcherStub{}
	a := &Adapter{restRouter: dispatcher}
	rec := &eventRecorder{}
	a.SetAdapterHooks(pipeline.AdapterHooks{OnSubscribe: rec.addSubscribe, OnError: rec.addError})
	ctx := context.Background()
	events, errs, err := a.ExecuteREST(ctx, pipeline.InteractionRequest{Method: "GET", Path: "/api"})
	require.NoError(t, err)
	resp := <-events
	p := resp.Payload.(pipeline.RestResponsePayload)
	require.NotNil(t, p.Response)
	require.Equal(t, 1, dispatcher.Calls())
	_, more := <-errs
	require.False(t, more)
	require.Len(t, rec.Subscribed(), 1)
}

func TestExecuteRESTRouterUnavailable(t *testing.T) {
	a := &Adapter{}
	rec := &eventRecorder{}
	a.SetAdapterHooks(pipeline.AdapterHooks{OnError: rec.addError})
	_, _, err := a.ExecuteREST(context.Background(), pipeline.InteractionRequest{Method: "GET"})
	require.Error(t, err)
	require.Len(t, rec.Errors(), 1)
}

func TestExecuteRESTDispatchError(t *testing.T) {
	dispatcher := &restDispatcherStub{err: errors.New("boom")}
	a := &Adapter{restRouter: dispatcher}
	rec := &eventRecorder{}
	a.SetAdapterHooks(pipeline.AdapterHooks{OnError: rec.addError})
	ctx := context.Background()
	events, errs, err := a.ExecuteREST(ctx, pipeline.InteractionRequest{Method: "GET", Path: "/api"})
	require.NoError(t, err)
	select {
	case e := <-errs:
		require.EqualError(t, e, "boom")
	case <-time.After(time.Second):
		t.Fatal("expected error from executeREST")
	}
	_, more := <-events
	require.False(t, more)
	require.Len(t, rec.Errors(), 1)
}

type retryDispatcher struct {
	mu        sync.Mutex
	responses []error
	calls     int
}

func (r *retryDispatcher) Dispatch(ctx context.Context, req pipeline.InteractionRequest, result any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := r.calls
	r.calls++
	if idx < len(r.responses) {
		return r.responses[idx]
	}
	return nil
}

func (r *retryDispatcher) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type restDispatcherStub struct {
	mu    sync.Mutex
	err   error
	calls int
}

func (r *restDispatcherStub) Dispatch(ctx context.Context, req pipeline.InteractionRequest, result any) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	if ptr, ok := result.(*interface{}); ok {
		*ptr = map[string]string{"status": "ok"}
	}
	return r.err
}

func (r *restDispatcherStub) Calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

type stubBookSubscriber struct {
	mu    sync.Mutex
	err   error
	calls []string
}

func (s *stubBookSubscriber) BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	s.mu.Lock()
	s.calls = append(s.calls, symbol)
	err := s.err
	s.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	events := make(chan corestreams.BookEvent)
	errs := make(chan error)
	close(events)
	close(errs)
	return events, errs, nil
}

func (s *stubBookSubscriber) Calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.calls...)
}

type stubPublicSubscriber struct {
	mu           sync.Mutex
	err          error
	topics       []string
	subscription *stubSubscription
}

func (s *stubPublicSubscriber) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	s.mu.Lock()
	s.topics = append(s.topics, topics...)
	err := s.err
	sub := s.subscription
	s.mu.Unlock()
	if err != nil {
		return nil, err
	}
	if sub == nil {
		sub = newStubSubscription()
	}
	return sub, nil
}

type stubPrivateSubscriber struct{}

func (s *stubPrivateSubscriber) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	return newStubSubscription(), nil
}

type stubSubscription struct {
	events chan core.Message
	errs   chan error
	mu     sync.Mutex
	closed bool
}

func newStubSubscription() *stubSubscription {
	return &stubSubscription{
		events: make(chan core.Message, 4),
		errs:   make(chan error, 1),
	}
}

func (s *stubSubscription) C() <-chan core.Message { return s.events }

func (s *stubSubscription) Err() <-chan error { return s.errs }

func (s *stubSubscription) Close() error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
	return nil
}

func (s *stubSubscription) IsClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

type eventRecorder struct {
	mu         sync.Mutex
	subscribed []pipeline.AdapterEvent
	errors     []pipeline.AdapterEvent
}

func (r *eventRecorder) addSubscribe(ctx context.Context, evt pipeline.AdapterEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subscribed = append(r.subscribed, evt)
}

func (r *eventRecorder) addError(ctx context.Context, evt pipeline.AdapterEvent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, evt)
}

func (r *eventRecorder) Subscribed() []pipeline.AdapterEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]pipeline.AdapterEvent(nil), r.subscribed...)
}

func (r *eventRecorder) Errors() []pipeline.AdapterEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]pipeline.AdapterEvent(nil), r.errors...)
}

var _ bridge.Dispatcher = (*retryDispatcher)(nil)
