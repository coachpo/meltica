package connection

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/handler"
	"github.com/coachpo/meltica/market_data/framework/telemetry"
)

func TestDispatcherDispatchesToMatchingHandler(t *testing.T) {
	engine := &Engine{reg: handler.NewRegistry()}
	called := 0
	_, err := engine.reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				called++
				return framework.HandlerOutcome{OutcomeType: stream.OutcomeAck}, nil
			})
		},
	})
	require.NoError(t, err)
	disp, err := newDispatcher(engine, &framework.ConnectionSession{}, DialOptions{
		Bindings: []handler.Binding{{Name: "quotes", Channels: []string{"BTC-USDT"}}},
	})
	require.NoError(t, err)
	env := &framework.MessageEnvelope{}
	env.SetDecoded(map[string]any{"channel": "btc-usdt"})
	summary := disp.Dispatch(context.Background(), env)
	require.Equal(t, 1, called)
	require.Equal(t, 1, summary.Invocations)
	require.Zero(t, summary.Errors)
}

func TestDispatcherOutcomeDropStopsPropagation(t *testing.T) {
	engine := &Engine{reg: handler.NewRegistry()}
	stopCalled := 0
	_, err := engine.reg.Register(stream.HandlerRegistration{
		Name:     "primary",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				stopCalled++
				return framework.HandlerOutcome{OutcomeType: stream.OutcomeDrop}, nil
			})
		},
	})
	require.NoError(t, err)
	secondaryCalled := 0
	_, err = engine.reg.Register(stream.HandlerRegistration{
		Name:     "secondary",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				secondaryCalled++
				return framework.HandlerOutcome{OutcomeType: stream.OutcomeAck}, nil
			})
		},
	})
	require.NoError(t, err)
	disp, err := newDispatcher(engine, &framework.ConnectionSession{}, DialOptions{
		Bindings: []handler.Binding{
			{Name: "primary", Channels: []string{"BTC-USDT"}},
			{Name: "secondary", Channels: []string{"BTC-USDT"}},
		},
	})
	require.NoError(t, err)
	env := &framework.MessageEnvelope{}
	env.SetDecoded(map[string]any{"channel": "BTC-USDT"})
	summary := disp.Dispatch(context.Background(), env)
	require.Equal(t, 1, stopCalled)
	require.Zero(t, secondaryCalled)
	require.True(t, summary.Dropped)
	require.Equal(t, 1, summary.Invocations)
}

func TestDispatcherTransformUpdatesEnvelope(t *testing.T) {
	engine := &Engine{reg: handler.NewRegistry()}
	_, err := engine.reg.Register(stream.HandlerRegistration{
		Name:     "transformer",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				return framework.HandlerOutcome{
					OutcomeType: stream.OutcomeTransform,
					PayloadData: map[string]any{"channel": "BTC-USDT", "price": 42},
				}, nil
			})
		},
	})
	require.NoError(t, err)
	seen := false
	_, err = engine.reg.Register(stream.HandlerRegistration{
		Name:     "consumer",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				payload, ok := env.Decoded().(map[string]any)
				require.True(t, ok)
				value, ok := payload["price"].(int)
				require.True(t, ok)
				require.Equal(t, 42, value)
				seen = true
				return framework.HandlerOutcome{OutcomeType: stream.OutcomeAck}, nil
			})
		},
	})
	require.NoError(t, err)
	disp, err := newDispatcher(engine, &framework.ConnectionSession{}, DialOptions{
		Bindings: []handler.Binding{
			{Name: "transformer", Channels: []string{"BTC-USDT"}},
			{Name: "consumer", Channels: []string{"BTC-USDT"}},
		},
	})
	require.NoError(t, err)
	env := &framework.MessageEnvelope{}
	env.SetDecoded(map[string]any{"channel": "BTC-USDT"})
	summary := disp.Dispatch(context.Background(), env)
	require.True(t, seen)
	require.Equal(t, 2, summary.Invocations)
	require.Zero(t, summary.Errors)
}

func TestDispatcherEmitsErrors(t *testing.T) {
	engine := &Engine{reg: handler.NewRegistry()}
	first := true
	_, err := engine.reg.Register(stream.HandlerRegistration{
		Name:     "failing",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				if first {
					first = false
					return nil, errFailure
				}
				return framework.HandlerOutcome{
					OutcomeType: stream.OutcomeError,
					Err:         errFailure,
				}, nil
			})
		},
	})
	require.NoError(t, err)
	disp, err := newDispatcher(engine, &framework.ConnectionSession{}, DialOptions{
		Bindings: []handler.Binding{{Name: "failing", Channels: []string{"BTC-USDT"}}},
	})
	require.NoError(t, err)
	received := make(chan error, 2)
	disp.setErrorSink(func(err error) { received <- err })
	env := &framework.MessageEnvelope{}
	env.SetDecoded(map[string]any{"channel": "BTC-USDT"})
	summary1 := disp.Dispatch(context.Background(), env)
	summary2 := disp.Dispatch(context.Background(), env)
	require.Equal(t, errFailure, <-received)
	require.Equal(t, errFailure, <-received)
	require.Equal(t, 1, summary1.Errors)
	require.Equal(t, 1, summary2.Errors)
}

func TestDispatcherEmitsTelemetryEvents(t *testing.T) {
	engine, err := NewEngine(Config{})
	require.NoError(t, err)
	count := 0
	_, err = engine.reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"BTC-USDT"},
		Factory: func() stream.Handler {
			return handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
				count++
				return nil, nil
			})
		},
	})
	require.NoError(t, err)
	disp, err := newDispatcher(engine, &framework.ConnectionSession{SessionID: "sess"}, DialOptions{
		Bindings: []handler.Binding{{Name: "quotes", Channels: []string{"BTC-USDT"}}},
	})
	require.NoError(t, err)
	disp.setEmitter(engine.Telemetry())
	events := make(chan telemetry.Event, 1)
	unsub := engine.Telemetry().Subscribe(func(evt telemetry.Event) {
		events <- evt
	})
	defer unsub()
	env := &framework.MessageEnvelope{}
	env.SetDecoded(map[string]any{"channel": "BTC-USDT"})
	summary := disp.Dispatch(context.Background(), env)
	require.Equal(t, 1, count)
	require.Equal(t, 1, summary.Invocations)
	select {
	case evt := <-events:
		require.Equal(t, telemetry.EventMessageProcessed, evt.Kind)
		require.Equal(t, stream.OutcomeAck, evt.Outcome)
		require.Equal(t, "sess", evt.SessionID)
	default:
		t.Fatal("expected telemetry event")
	}
}

var errFailure = streamError("handler failure")

type streamError string

func (e streamError) Error() string { return string(e) }
