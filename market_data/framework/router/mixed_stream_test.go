package router

import (
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestMixedStreamRoutingAccuracy(t *testing.T) {
	const (
		tradeCount     = 100
		orderBookCount = 50
	)

	rt, err := InitializeRouter()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dispatcher := NewRouterDispatcher(ctx, rt.metrics)
	defer dispatcher.Shutdown()

	tradeReg := rt.Lookup("trade")
	require.NotNil(t, tradeReg)
	orderReg := rt.Lookup("orderbook")
	require.NotNil(t, orderReg)

	tradeInbox := dispatcher.Bind("trade", tradeReg)
	orderInbox := dispatcher.Bind("orderbook", orderReg)

	tradeResults := make(map[string]*market_data.TradePayload, tradeCount)
	orderResults := make(map[string]*market_data.OrderBookPayload, orderBookCount)

	var tradeMu sync.Mutex
	tradeErr := make(chan error, 1)
	go func() {
		defer close(tradeErr)
		for i := 0; i < tradeCount; i++ {
			raw := <-tradeInbox
			result, err := tradeReg.Processor.Process(context.Background(), raw)
			if err != nil {
				tradeErr <- fmt.Errorf("trade processing: %w", err)
				return
			}
			payload, ok := result.(*market_data.TradePayload)
			if !ok {
				tradeErr <- fmt.Errorf("expected *TradePayload, got %T", result)
				return
			}
			tradeMu.Lock()
			tradeResults[payload.VenueTradeID] = payload
			tradeMu.Unlock()
		}
		tradeErr <- nil
	}()

	var orderMu sync.Mutex
	orderErr := make(chan error, 1)
	go func() {
		defer close(orderErr)
		for i := 0; i < orderBookCount; i++ {
			raw := <-orderInbox
			result, err := orderReg.Processor.Process(context.Background(), raw)
			if err != nil {
				orderErr <- fmt.Errorf("orderbook processing: %w", err)
				return
			}
			payload, ok := result.(*market_data.OrderBookPayload)
			if !ok {
				orderErr <- fmt.Errorf("expected *OrderBookPayload, got %T", result)
				return
			}
			orderMu.Lock()
			orderResults[payload.Bids[0].Price.String()] = payload
			orderMu.Unlock()
		}
		orderErr <- nil
	}()

	type message struct {
		raw      []byte
		expected string
	}

	messages := make([]message, 0, tradeCount+orderBookCount)
	for i := 0; i < tradeCount; i++ {
		price := fmt.Sprintf("%d.50", 100+i)
		quantity := fmt.Sprintf("0.%03d", i+1)
		tradeID := fmt.Sprintf("trade-%d", i)
		raw := []byte(fmt.Sprintf(`{"type":"trade","price":"%s","quantity":"%s","side":"buy","taker":true,"trade_id":"%s"}`, price, quantity, tradeID))
		messages = append(messages, message{raw: raw, expected: "trade"})
	}
	for i := 0; i < orderBookCount; i++ {
		bidPrice := fmt.Sprintf("%d.10", 200+i)
		askPrice := fmt.Sprintf("%d.90", 200+i)
		raw := []byte(fmt.Sprintf(`{"type":"book","snapshot":false,"bids":[{"price":"%s","quantity":"1"}],"asks":[{"price":"%s","quantity":"2"}]}`, bidPrice, askPrice))
		messages = append(messages, message{raw: raw, expected: "orderbook"})
	}

	r := rand.New(rand.NewSource(42))
	r.Shuffle(len(messages), func(i, j int) { messages[i], messages[j] = messages[j], messages[i] })

	for _, msg := range messages {
		messageType, err := rt.Detect(msg.raw)
		require.NoError(t, err)
		require.Equal(t, msg.expected, messageType)
		require.NoError(t, dispatcher.Dispatch(messageType, msg.raw))
	}

	require.NoError(t, <-tradeErr)
	require.NoError(t, <-orderErr)

	require.Len(t, tradeResults, tradeCount)
	require.Len(t, orderResults, orderBookCount)

	for i := 0; i < tradeCount; i++ {
		tradeID := fmt.Sprintf("trade-%d", i)
		payload, ok := tradeResults[tradeID]
		require.True(t, ok, "trade %s missing", tradeID)
		require.Equal(t, tradeID, payload.VenueTradeID)
	}

	for i := 0; i < orderBookCount; i++ {
		bidPrice := fmt.Sprintf("%d.10", 200+i)
		expectedKey, ok := new(big.Rat).SetString(bidPrice)
		require.True(t, ok, "failed to parse %s", bidPrice)
		key := expectedKey.String()
		payload, ok := orderResults[key]
		require.True(t, ok, "order book %s missing", bidPrice)
		require.Equal(t, key, payload.Bids[0].Price.String())
	}

	metrics := rt.GetMetrics()
	require.Equal(t, uint64(tradeCount), metrics.MessagesRouted["trade"])
	require.Equal(t, uint64(orderBookCount), metrics.MessagesRouted["orderbook"])
	require.Zero(t, metrics.RoutingErrors)
}
