package router

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
)

func TestShimQueuesSubscriptionsBeforeStart(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{}
	publisher := &stubPublisher{}

	session, err := Init(ctx, Options{
		SessionID: "shim-integration-queue",
		Dialer:    dialer,
		Parser:    &stubParser{},
		Publish:   publisher.Publish,
		Backoff: BackoffConfig{
			Initial:    50 * time.Millisecond,
			Max:        500 * time.Millisecond,
			Multiplier: big.NewRat(5, 4),
			Jitter:     5,
		},
	})
	require.NoError(t, err)

	specA := SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"ETH-USDT"}}
	specB := SubscriptionSpec{Exchange: "binance", Channel: "book", Symbols: []string{"BTC-USDT"}}
	require.NoError(t, Subscribe(ctx, session, specA))
	require.NoError(t, Subscribe(ctx, session, specB))

	require.NoError(t, Start(ctx, session))
	require.Equal(t, 1, dialer.Calls())

	subs := dialer.conn.Subscriptions()
	require.Equal(t, []SubscriptionSpec{specA, specB}, subs)
}

func TestShimEnforcesStreamingState(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{}
	publisher := &stubPublisher{}
	session, err := Init(ctx, Options{
		SessionID: "shim-integration-state",
		Dialer:    dialer,
		Parser:    &stubParser{},
		Publish:   publisher.Publish,
		Backoff:   BackoffConfig{Initial: time.Millisecond, Max: time.Millisecond, Multiplier: big.NewRat(1, 1)},
	})
	require.NoError(t, err)

	msg := &Message{Type: "premature"}
	err = Publish(ctx, session, msg)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)

	require.NoError(t, Start(ctx, session))
	require.NoError(t, Publish(ctx, session, msg))

	require.NotNil(t, dialer.conn)
	require.NoError(t, RouteRaw(ctx, session, []byte(`{"kind":"demo"}`)))
	require.Equal(t, 2, publisher.Invocations())
}
