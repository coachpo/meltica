package binance

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

func TestSessionKeepsListenKeyAliveAndCloses(t *testing.T) {
	restore := routing.SetPrivateKeepAliveInterval(10 * time.Millisecond)
	defer restore()

	deps := &perfDeps{
		keepAliveSignal: make(chan struct{}, 4),
		closeSignal:     make(chan struct{}, 1),
	}

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(context.Context, ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		_, sub := newPerfSubscription(64)
		return sub, nil
	}

	router := routing.NewWSRouter(client, deps)
	t.Cleanup(func() { require.NoError(t, router.Close()) })

	svc := newWSService(router)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	subscription, err := svc.SubscribePrivate(ctx)
	require.NoError(t, err)
	require.NotNil(t, subscription)

	select {
	case <-deps.keepAliveSignal:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected keep alive signal but timed out")
	}

	require.GreaterOrEqual(t, atomic.LoadInt32(&deps.keepAliveCount), int32(1))

	require.NoError(t, subscription.Close())

	select {
	case <-deps.closeSignal:
	case <-time.After(time.Second):
		t.Fatal("expected listen key close invocation")
	}

	require.Equal(t, int32(1), atomic.LoadInt32(&deps.closeCount))

}
