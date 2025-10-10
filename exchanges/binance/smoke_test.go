package binance

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/routing"
)

func TestBinanceSmokeSuite(t *testing.T) {
	restore := routing.SetPrivateKeepAliveInterval(10 * time.Millisecond)
	defer restore()

	deps := &perfDeps{
		keepAliveSignal: make(chan struct{}, 2),
		closeSignal:     make(chan struct{}, 2),
	}

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		ps, sub := newPerfSubscription(32)
		if len(topics) > 0 && topics[0].Scope == coretransport.StreamScopePublic {
			go func() {
				ps.messages <- coretransport.RawMessage{Data: tradePayload("BTC-USDT")}
			}()
		} else {
			go func() {
				ps.messages <- coretransport.RawMessage{Data: balancePayload("balanceUpdate")}
			}()
		}
		return sub, nil
	}

	router := routing.NewWSRouter(client, deps)
	t.Cleanup(func() { require.NoError(t, router.Close()) })

	svc := newWSService(router)
	pubSub, err := svc.SubscribePublic(context.Background(), routing.Trade("BTC-USDT"))
	require.NoError(t, err)
	privSub, err := svc.SubscribePrivate(context.Background())
	require.NoError(t, err)

	select {
	case msg := <-pubSub.C():
		require.Equal(t, corestreams.RouteTradeUpdate, msg.Event)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for public message")
	}

	select {
	case msg := <-privSub.C():
		require.Equal(t, corestreams.RouteBalanceSnapshot, msg.Event)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for private message")
	}

	select {
	case <-deps.keepAliveSignal:
	case <-time.After(time.Second):
		t.Fatal("expected keep-alive signal")
	}

	require.NoError(t, pubSub.Close())
	require.NoError(t, privSub.Close())

	select {
	case <-deps.closeSignal:
	case <-time.After(time.Second):
		t.Fatal("expected listen key closure")
	}
}

func balancePayload(event string) []byte {
	payload := map[string]any{
		"e": event,
		"a": "USDT",
		"d": "1.23",
	}
	b, _ := json.Marshal(payload)
	return b
}
