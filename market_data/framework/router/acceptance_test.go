package router

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShimRoutesMessagesViaFramework(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{}
	parser := &stubParser{}
	publisher := &stubPublisher{}

	options := Options{
		SessionID: "shim-acceptance",
		Dialer:    dialer,
		Parser:    parser,
		Publish:   publisher.Publish,
		Backoff: BackoffConfig{
			Initial:    100 * time.Millisecond,
			Max:        time.Second,
			Multiplier: big.NewRat(3, 2),
			Jitter:     10,
		},
	}

	session, err := Init(ctx, options)
	require.NoError(t, err)
	require.Equal(t, StateInitialized, session.State())

	spec := SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"BTC-USDT"}}
	require.NoError(t, Subscribe(ctx, session, spec))

	recorder := &middlewareRecorder{}
	require.NoError(t, UseMiddleware(session, recorder.Handler))

	require.NoError(t, Start(ctx, session))
	require.Equal(t, 1, dialer.Calls())
	require.Equal(t, StateStreaming, session.State())

	subs := dialer.conn.Subscriptions()
	require.Len(t, subs, 1)
	require.Equal(t, spec, subs[0])

	manual := &Message{Type: "manual", Metadata: map[string]string{}}
	require.NoError(t, Publish(ctx, session, manual))

	require.NoError(t, RouteRaw(ctx, session, []byte(`{"kind":"auto"}`)))

	require.Equal(t, 2, publisher.Invocations())
	msgs := publisher.Messages()
	require.Len(t, msgs, 2)
	require.Equal(t, "manual", msgs[0].Type)
	require.Equal(t, "stub", msgs[1].Type)
	require.Equal(t, "seen", msgs[1].Metadata["middleware"])
	require.Equal(t, 2, recorder.calls)
}
