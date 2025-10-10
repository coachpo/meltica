package router

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestConcurrentStreamsRoutingIsolation(t *testing.T) {
	tradeTargets := []int{20, 0, 15, 10, 30, 5, 25, 0, 12, 18}
	orderTargets := []int{5, 25, 10, 0, 5, 20, 0, 15, 8, 2}
	require.Len(t, tradeTargets, len(orderTargets))
	streamCount := len(tradeTargets)

	rt, err := InitializeRouter()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tradeReg := rt.Lookup("trade")
	require.NotNil(t, tradeReg)
	orderReg := rt.Lookup("orderbook")
	require.NotNil(t, orderReg)

	type streamState struct {
		tradeInbox <-chan []byte
		orderInbox <-chan []byte
		tradeTotal int
		orderTotal int
		trades     map[string]*market_data.TradePayload
		orders     map[string]*market_data.OrderBookPayload
		tradeMu    sync.Mutex
		orderMu    sync.Mutex
	}

	dispatchers := make([]*RouterDispatcher, streamCount)
	states := make([]*streamState, streamCount)

	for i := 0; i < streamCount; i++ {
		disp := NewRouterDispatcher(ctx, rt.metrics)
		dispatchers[i] = disp
		state := &streamState{
			tradeInbox: disp.Bind("trade", tradeReg),
			orderInbox: disp.Bind("orderbook", orderReg),
			tradeTotal: tradeTargets[i],
			orderTotal: orderTargets[i],
			trades:     make(map[string]*market_data.TradePayload, tradeTargets[i]),
			orders:     make(map[string]*market_data.OrderBookPayload, orderTargets[i]),
		}
		states[i] = state
	}
	defer func() {
		for _, disp := range dispatchers {
			if disp != nil {
				disp.Shutdown()
			}
		}
	}()

	errCh := make(chan error, 1)
	var errOnce sync.Once
	fail := func(err error) {
		if err == nil {
			return
		}
		errOnce.Do(func() {
			errCh <- err
			cancel()
		})
	}

	var consumerWG sync.WaitGroup
	for streamID, state := range states {
		if state.tradeTotal > 0 {
			consumerWG.Add(1)
			go func(id int, st *streamState) {
				defer consumerWG.Done()
				for i := 0; i < st.tradeTotal; i++ {
					select {
					case <-ctx.Done():
						return
					case raw := <-st.tradeInbox:
						result, err := tradeReg.Processor.Process(ctx, raw)
						if err != nil {
							fail(fmt.Errorf("stream %d trade processing %d: %w", id, i, err))
							return
						}
						payload, ok := result.(*market_data.TradePayload)
						if !ok {
							fail(fmt.Errorf("stream %d expected *TradePayload, got %T", id, result))
							return
						}
						st.tradeMu.Lock()
						st.trades[payload.VenueTradeID] = payload
						st.tradeMu.Unlock()
					}
				}
			}(streamID, state)
		}
		if state.orderTotal > 0 {
			consumerWG.Add(1)
			go func(id int, st *streamState) {
				defer consumerWG.Done()
				for i := 0; i < st.orderTotal; i++ {
					select {
					case <-ctx.Done():
						return
					case raw := <-st.orderInbox:
						result, err := orderReg.Processor.Process(ctx, raw)
						if err != nil {
							fail(fmt.Errorf("stream %d orderbook processing %d: %w", id, i, err))
							return
						}
						payload, ok := result.(*market_data.OrderBookPayload)
						if !ok {
							fail(fmt.Errorf("stream %d expected *OrderBookPayload, got %T", id, result))
							return
						}
						st.orderMu.Lock()
						if len(payload.Bids) > 0 {
							st.orders[payload.Bids[0].Price.String()] = payload
						}
						st.orderMu.Unlock()
					}
				}
			}(streamID, state)
		}
	}

	var sendWG sync.WaitGroup
	for streamID, state := range states {
		state := state
		dispatcher := dispatchers[streamID]
		sendWG.Add(1)
		go func(id int) {
			defer sendWG.Done()
			for i := 0; i < state.tradeTotal; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				tradeID := fmt.Sprintf("stream-%d-trade-%d", id, i)
				price := fmt.Sprintf("%d.%02d", 100+id, i%100)
				quantity := fmt.Sprintf("0.%04d", (i+1)%10000)
				raw := []byte(fmt.Sprintf(`{"type":"trade","price":"%s","quantity":"%s","side":"buy","taker":true,"trade_id":"%s"}`, price, quantity, tradeID))
				msgType, err := rt.Detect(raw)
				if err != nil {
					fail(fmt.Errorf("stream %d detect trade %d: %w", id, i, err))
					return
				}
				if msgType != "trade" {
					fail(fmt.Errorf("stream %d expected trade, got %s", id, msgType))
					return
				}
				if err := dispatcher.Dispatch(msgType, raw); err != nil {
					fail(fmt.Errorf("stream %d dispatch trade %d: %w", id, i, err))
					return
				}
			}
			for i := 0; i < state.orderTotal; i++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				bid := fmt.Sprintf("%d.%02d", 200+id, i%100)
				ask := fmt.Sprintf("%d.%02d", 201+id, i%100)
				raw := []byte(fmt.Sprintf(`{"type":"book","snapshot":false,"bids":[{"price":"%s","quantity":"1"}],"asks":[{"price":"%s","quantity":"2"}]}`, bid, ask))
				msgType, err := rt.Detect(raw)
				if err != nil {
					fail(fmt.Errorf("stream %d detect book %d: %w", id, i, err))
					return
				}
				if msgType != "orderbook" {
					fail(fmt.Errorf("stream %d expected orderbook, got %s", id, msgType))
					return
				}
				if err := dispatcher.Dispatch(msgType, raw); err != nil {
					fail(fmt.Errorf("stream %d dispatch book %d: %w", id, i, err))
					return
				}
			}
		}(streamID)
	}

	sendWG.Wait()
	consumerWG.Wait()

	select {
	case err := <-errCh:
		require.NoError(t, err)
	default:
	}

	for i, state := range states {
		require.Len(t, state.trades, tradeTargets[i])
		require.Len(t, state.orders, orderTargets[i])
		prefix := fmt.Sprintf("stream-%d-trade-", i)
		for tradeID := range state.trades {
			require.True(t, strings.HasPrefix(tradeID, prefix))
		}
		for j := 0; j < orderTargets[i]; j++ {
			bid := fmt.Sprintf("%d.%02d", 200+i, j%100)
			expected, ok := new(big.Rat).SetString(bid)
			require.True(t, ok)
			key := expected.String()
			require.Contains(t, state.orders, key)
		}
	}

	metrics := rt.GetMetrics()
	var totalTrades, totalBooks uint64
	for _, count := range tradeTargets {
		totalTrades += uint64(count)
	}
	for _, count := range orderTargets {
		totalBooks += uint64(count)
	}
	require.Equal(t, totalTrades, metrics.MessagesRouted["trade"])
	require.Equal(t, totalBooks, metrics.MessagesRouted["orderbook"])
	require.Zero(t, metrics.RoutingErrors)
}
