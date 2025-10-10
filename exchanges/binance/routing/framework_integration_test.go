package routing

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
)

type integrationDeps struct {
	listenKey      string
	keepAliveCount int32
	closeCount     int32
}

func (d *integrationDeps) CanonicalSymbol(binanceSymbol string) (string, error) {
	upper := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if strings.Contains(upper, "-") || len(upper) < 6 {
		return upper, nil
	}
	return upper[:len(upper)/2] + "-" + upper[len(upper)/2:], nil
}

func (d *integrationDeps) NativeSymbol(canonical string) (string, error) {
	return strings.ReplaceAll(strings.ToUpper(canonical), "-", ""), nil
}

func (d *integrationDeps) NativeTopic(topic core.Topic) (string, error) {
	switch topic {
	case core.TopicTrade:
		return "trade", nil
	case core.TopicTicker:
		return "ticker", nil
	case core.TopicBookDelta:
		return "depth@100ms", nil
	case core.TopicUserOrder:
		return "order", nil
	case core.TopicUserBalance:
		return "balance", nil
	default:
		return "", core.ErrNotSupported
	}
}

func (d *integrationDeps) CreateListenKey(context.Context) (string, error) {
	if d.listenKey == "" {
		d.listenKey = "listen-key"
	}
	return d.listenKey, nil
}

func (d *integrationDeps) KeepAliveListenKey(context.Context, string) error {
	atomic.AddInt32(&d.keepAliveCount, 1)
	return nil
}

func (d *integrationDeps) CloseListenKey(context.Context, string) error {
	atomic.AddInt32(&d.closeCount, 1)
	return nil
}

func (d *integrationDeps) BookDepthSnapshot(context.Context, string, int) (corestreams.BookEvent, int64, error) {
	return corestreams.BookEvent{}, 0, nil
}

func newMockStreamSubscription(messages chan coretransport.RawMessage, errs chan error) *mocks.StreamSubscription {
	var once sync.Once
	return &mocks.StreamSubscription{
		MessagesFunc: func() <-chan coretransport.RawMessage { return messages },
		ErrorsFunc:   func() <-chan error { return errs },
		CloseFunc: func() error {
			once.Do(func() {
				close(messages)
				close(errs)
			})
			return nil
		},
	}
}

func mustJSON(tb testing.TB, v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		tb.Fatalf("marshal payload: %v", err)
	}
	return b
}

func TestFrameworkIntegrationRoutesPublicMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := &integrationDeps{}
	messages := make(chan coretransport.RawMessage, 1)
	errs := make(chan error, 1)
	subscription := newMockStreamSubscription(messages, errs)

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		require.Len(t, topics, 1)
		require.Equal(t, coretransport.StreamScopePublic, topics[0].Scope)
		require.Equal(t, "btcusdt@trade", topics[0].Name)
		return subscription, nil
	}

	router := NewWSRouter(client, deps)
	t.Cleanup(func() {
		require.NoError(t, router.Close())
	})

	stream, err := router.SubscribePublic(ctx, Trade("BTC-USDT"))
	require.NoError(t, err)
	require.NotNil(t, stream)

	defer func() {
		require.NoError(t, stream.Close())
	}()

	payload := map[string]any{
		"stream": "btcusdt@trade",
		"data": map[string]any{
			"e": "trade",
			"s": "BTCUSDT",
			"p": "100.12",
			"q": "0.25",
			"T": 1700000000000,
		},
	}
	messages <- coretransport.RawMessage{Data: mustJSON(t, payload)}

	select {
	case msg := <-stream.C():
		require.Equal(t, corestreams.RouteTradeUpdate, msg.Route)
		require.Equal(t, core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT"), msg.Topic)
		trade, ok := msg.Parsed.(*corestreams.TradeEvent)
		require.True(t, ok)
		require.Equal(t, "BTC-USDT", trade.Symbol)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for routed trade message")
	}

	select {
	case err := <-stream.Err():
		require.NoError(t, err)
	default:
	}
}

func TestFrameworkIntegrationRoutesPrivateMessages(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := &integrationDeps{}
	messages := make(chan coretransport.RawMessage, 1)
	errs := make(chan error, 1)
	subscription := newMockStreamSubscription(messages, errs)

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		require.Len(t, topics, 1)
		require.Equal(t, coretransport.StreamScopePrivate, topics[0].Scope)
		require.Equal(t, "listen-key", topics[0].Name)
		return subscription, nil
	}

	router := NewWSRouter(client, deps)
	t.Cleanup(func() {
		require.NoError(t, router.Close())
	})

	stream, err := router.SubscribePrivate(ctx)
	require.NoError(t, err)
	require.NotNil(t, stream)

	raw := []byte("{\"e\":\"balanceUpdate\",\"a\":\"USDT\",\"d\":\"5.123\"}")
	messages <- coretransport.RawMessage{Data: raw}

	select {
	case msg := <-stream.C():
		require.Equal(t, corestreams.RouteBalanceSnapshot, msg.Route)
		require.Equal(t, core.MustCanonicalTopic(core.TopicUserBalance, ""), msg.Topic)
		balance, ok := msg.Parsed.(*corestreams.BalanceEvent)
		require.True(t, ok)
		require.Len(t, balance.Balances, 1)
		require.Equal(t, "USDT", balance.Balances[0].Asset)
	case err := <-stream.Err():
		require.NoError(t, err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for private message")
	}

	require.NoError(t, stream.Close())
	require.EqualValues(t, 1, atomic.LoadInt32(&deps.closeCount))
}

func TestPrivateDispatcherKeepsListenKeyAlive(t *testing.T) {
	original := privateKeepAliveInterval
	privateKeepAliveInterval = 5 * time.Millisecond
	defer func() { privateKeepAliveInterval = original }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deps := &integrationDeps{}
	messages := make(chan coretransport.RawMessage)
	errs := make(chan error)
	subscription := newMockStreamSubscription(messages, errs)

	client := &mocks.StreamClient{}
	client.SubscribeFunc = func(_ context.Context, topics ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
		return subscription, nil
	}

	router := NewWSRouter(client, deps)
	t.Cleanup(func() {
		require.NoError(t, router.Close())
	})

	stream, err := router.SubscribePrivate(ctx)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&deps.keepAliveCount) > 0
	}, time.Second, 5*time.Millisecond)

	require.NoError(t, stream.Close())
	require.EqualValues(t, 1, atomic.LoadInt32(&deps.closeCount))
}
