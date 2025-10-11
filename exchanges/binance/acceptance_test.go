package binance

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/routing"
	binancetelemetry "github.com/coachpo/meltica/exchanges/binance/telemetry"
	bnwsrouting "github.com/coachpo/meltica/exchanges/binance/wsrouting"
	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

func TestUserStory3Acceptance(t *testing.T) {
	ctx := context.Background()
	deps := &perfDeps{}
	table := bnwsrouting.NewRoutingTable()
	require.NoError(t, routing.RegisterProcessors(table, deps))

	payload := tradePayload("BTC-USDT")
	typeID, err := table.Detect(payload)
	require.NoError(t, err)
	require.Equal(t, "binance.trade", typeID)

	registration := table.Lookup(typeID)
	require.NotNil(t, registration)
	require.NotNil(t, registration.Processor)

	processed, err := registration.Processor.Process(ctx, payload)
	require.NoError(t, err)
	msg, ok := processed.(*corestreams.RoutedMessage)
	require.True(t, ok, "expected routed message output")
	require.Equal(t, corestreams.RouteTradeUpdate, msg.Route)
	require.NotNil(t, msg.Parsed)

	routingMetrics := bnwsrouting.NewRoutingMetrics()
	routingMetrics.RecordRoute(typeID)
	routingMetrics.RecordProcessing(typeID, time.Millisecond, nil)
	routingMetrics.RecordBackpressureStart(typeID)

	connMetrics := wsrouting.MetricsSnapshot{MessagesTotal: 1, ErrorsTotal: 0, Allocated: 512}
	bridgeMetrics := binancetelemetry.BridgeSnapshot{RESTRequests: 3, RESTFailures: 0}
	filterMetrics := binancetelemetry.FilterSnapshot{Forwarded: 5, Dropped: 0}

	layers := binancetelemetry.Collect(connMetrics, routingMetrics, bridgeMetrics, filterMetrics)
	flat := layers.Flatten("binance")

	require.Equal(t, float64(1), flat["binance_l1_messages_total"])
	require.Equal(t, float64(0), flat["binance_l1_errors_total"])
	require.Equal(t, float64(0), flat["binance_l2_routing_errors_total"])
	require.Equal(t, float64(1), flat["binance_l2_backpressure_total"])
	require.Equal(t, float64(3), flat["binance_l3_rest_requests_total"])
	require.Equal(t, float64(5), flat["binance_l4_forwarded_total"])
}

func TestUserStory4Acceptance(t *testing.T) {
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

	router := routing.NewWSRouter(newMockWSConnection(client), deps)
	t.Cleanup(func() { require.NoError(t, router.Close()) })

	svc := newWSService(router)
	pubSub, err := svc.SubscribePublic(context.Background(), routing.Trade("BTC-USDT"))
	require.NoError(t, err)
	require.NotNil(t, pubSub)
	privSub, err := svc.SubscribePrivate(context.Background())
	require.NoError(t, err)
	require.NotNil(t, privSub)

	var publicCount, privateCount int32
	select {
	case msg := <-pubSub.C():
		require.Equal(t, corestreams.RouteTradeUpdate, msg.Event)
		atomic.AddInt32(&publicCount, 1)
	case <-time.After(time.Second):
		t.Fatal("expected public message")
	}

	select {
	case msg := <-privSub.C():
		require.Equal(t, corestreams.RouteBalanceSnapshot, msg.Event)
		atomic.AddInt32(&privateCount, 1)
	case <-time.After(time.Second):
		t.Fatal("expected private message")
	}

	select {
	case <-deps.keepAliveSignal:
	case <-time.After(time.Second):
		t.Fatal("expected keep alive signal")
	}

	require.NoError(t, pubSub.Close())
	require.NoError(t, privSub.Close())

	select {
	case <-deps.closeSignal:
	case <-time.After(time.Second):
		t.Fatal("expected listen key close")
	}

	totalMessages := uint64(atomic.LoadInt32(&publicCount) + atomic.LoadInt32(&privateCount))
	connSnapshot := wsrouting.MetricsSnapshot{MessagesTotal: totalMessages, ErrorsTotal: 0, Allocated: 0}
	routingMetrics := bnwsrouting.NewRoutingMetrics()
	routingMetrics.RecordRoute("binance.trade")
	routingMetrics.RecordProcessing("binance.trade", time.Millisecond, nil)
	routingMetrics.RecordRoute("binance.user.balance")
	routingMetrics.RecordProcessing("binance.user.balance", time.Millisecond, nil)
	bridgeMetrics := binancetelemetry.BridgeSnapshot{RESTRequests: 1}
	filterMetrics := binancetelemetry.FilterSnapshot{Forwarded: totalMessages}

	layers := binancetelemetry.Collect(connSnapshot, routingMetrics, bridgeMetrics, filterMetrics)
	flat := layers.Flatten("binance")

	require.Equal(t, float64(totalMessages), flat["binance_l1_messages_total"])
	require.Equal(t, float64(0), flat["binance_l1_errors_total"])
	require.Equal(t, float64(1), flat["binance_l3_rest_requests_total"])
	require.Equal(t, float64(totalMessages), flat["binance_l4_forwarded_total"])
	require.Greater(t, atomic.LoadInt32(&deps.keepAliveCount), int32(0))
	require.Equal(t, int32(1), atomic.LoadInt32(&deps.closeCount))
}
