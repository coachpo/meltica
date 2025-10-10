package router

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestUnrecognizedMessageRoutesToDefault(t *testing.T) {
	rt, err := InitializeRouter()
	require.NoError(t, err)

	ctx := context.Background()
	dispatcher := NewRouterDispatcher(ctx, rt.metrics)
	defer dispatcher.Shutdown()

	tradeReg := rt.Lookup("trade")
	require.NotNil(t, tradeReg)
	tradeInbox := dispatcher.Bind("trade", tradeReg)
	defaultInbox := dispatcher.BindDefault(rt.defaultProc)

	type tradeResult struct {
		payload *market_data.TradePayload
		err     error
	}

	consumeTrade := func() <-chan tradeResult {
		resultCh := make(chan tradeResult, 1)
		go func() {
			raw, ok := <-tradeInbox
			if !ok {
				resultCh <- tradeResult{err: fmt.Errorf("trade inbox closed")}
				return
			}
			result, err := tradeReg.Processor.Process(ctx, raw)
			if err != nil {
				resultCh <- tradeResult{err: err}
				return
			}
			payload, ok := result.(*market_data.TradePayload)
			if !ok {
				resultCh <- tradeResult{err: fmt.Errorf("expected *TradePayload, got %T", result)}
				return
			}
			resultCh <- tradeResult{payload: payload}
		}()
		return resultCh
	}

	type defaultResult struct {
		payload *market_data.RawPayload
		err     error
	}

	consumeDefault := func() <-chan defaultResult {
		resultCh := make(chan defaultResult, 1)
		go func() {
			raw, ok := <-defaultInbox
			if !ok {
				resultCh <- defaultResult{err: fmt.Errorf("default inbox closed")}
				return
			}
			result, err := rt.defaultProc.Processor.Process(ctx, raw)
			if err != nil {
				resultCh <- defaultResult{err: err}
				return
			}
			payload, ok := result.(*market_data.RawPayload)
			if !ok {
				resultCh <- defaultResult{err: fmt.Errorf("expected *RawPayload, got %T", result)}
				return
			}
			resultCh <- defaultResult{payload: payload}
		}()
		return resultCh
	}

	trade1 := []byte(`{"type":"trade","price":"1","quantity":"1","side":"buy","taker":false,"trade_id":"known-1"}`)
	msgType, err := rt.Detect(trade1)
	require.NoError(t, err)
	require.Equal(t, "trade", msgType)
	tradeCh := consumeTrade()
	require.NoError(t, dispatcher.Dispatch(msgType, trade1))
	res1 := <-tradeCh
	require.NoError(t, res1.err)
	payload1 := res1.payload
	require.Equal(t, "known-1", payload1.VenueTradeID)

	unknown := []byte(`{"type":"mystery","value":42}`)
	_, err = rt.Detect(unknown)
	require.Error(t, err)
	defaultCh := consumeDefault()
	require.NoError(t, dispatcher.Dispatch("", unknown))
	defaultRes := <-defaultCh
	require.NoError(t, defaultRes.err)
	defaultPayload := defaultRes.payload
	require.Equal(t, unknown, defaultPayload.Data)
	require.Equal(t, "default", defaultPayload.Metadata["processor"])

	trade2 := []byte(`{"type":"trade","price":"2","quantity":"3","side":"sell","taker":true,"trade_id":"known-2"}`)
	msgType, err = rt.Detect(trade2)
	require.NoError(t, err)
	require.Equal(t, "trade", msgType)
	tradeCh = consumeTrade()
	require.NoError(t, dispatcher.Dispatch(msgType, trade2))
	res2 := <-tradeCh
	require.NoError(t, res2.err)
	payload2 := res2.payload
	require.Equal(t, "known-2", payload2.VenueTradeID)

	metrics := rt.GetMetrics()
	require.Equal(t, uint64(2), metrics.MessagesRouted["trade"])
	require.Equal(t, uint64(1), metrics.RoutingErrors)
}
