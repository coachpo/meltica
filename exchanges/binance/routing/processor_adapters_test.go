package routing

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/errs"
)

func TestContextCancelledHelper(t *testing.T) {
	require.NoError(t, contextCancelled(nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := contextCancelled(ctx)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)
}

func TestTradeProcessorAdapterProcess(t *testing.T) {
	resolver := stubResolver{mapping: map[string]string{"BTCUSDT": "BTC-USD"}}
	proc := NewTradeProcessorAdapter(resolver)

	_, err := proc.Process(context.Background(), []byte(`{`))
	require.Error(t, err)

	_, err = proc.Process(context.Background(), []byte(`{"s":"","p":"1","q":"1","T":1}`))
	require.Error(t, err)

	resolver.err = errors.New("symbol lookup failed")
	proc = NewTradeProcessorAdapter(resolver)
	_, err = proc.Process(context.Background(), []byte(`{"s":"BTCUSDT","p":"1","q":"1","T":1}`))
	require.Error(t, err)

	resolver = stubResolver{mapping: map[string]string{"BTCUSDT": "BTC-USD"}}
	proc = NewTradeProcessorAdapter(resolver)
	payload := mustJSON(t, map[string]any{
		"s": "BTCUSDT",
		"p": "100",
		"q": "0.5",
		"T": 12345,
	})
	msg, err := proc.Process(context.Background(), payload)
	require.NoError(t, err)
	routed := msg.(*corestreams.RoutedMessage)
	require.Equal(t, corestreams.RouteTradeUpdate, routed.Route)
	require.Equal(t, Trade("BTC-USD"), routed.Topic)
	evt, ok := routed.Parsed.(*corestreams.TradeEvent)
	require.True(t, ok)
	require.Equal(t, "BTC-USD", evt.Symbol)
}

func TestOrderBookProcessorAdapterProcess(t *testing.T) {
	resolver := stubResolver{mapping: map[string]string{"BTCUSDT": "BTC-USD"}}
	proc := NewOrderBookProcessorAdapter(resolver)

	_, err := proc.Process(context.Background(), []byte(`{"e":"depthUpdate","s":"","E":1}`))
	require.Error(t, err)

	_, err = proc.Process(context.Background(), []byte(`{"e":"trade"}`))
	require.Error(t, err)

	msg := &corestreams.RoutedMessage{}
	record := orderbookRecord{
		Symbol:        "BTCUSDT",
		FirstUpdateID: 1,
		LastUpdateID:  2,
		Bids:          [][]interface{}{{"100", "1"}},
		Asks:          [][]interface{}{{"101", "1"}},
		EventTime:     12345,
	}
	require.NoError(t, parseOrderbookEvent(msg, &record, "BTC-USD", "depthUpdate"))
	require.Equal(t, corestreams.RouteDepthDelta, msg.Route)
	require.Equal(t, OrderBook("BTC-USD"), msg.Topic)
}

func TestTickerProcessorAdapterProcess(t *testing.T) {
	resolver := stubResolver{mapping: map[string]string{"BTCUSDT": "BTC-USD"}}
	proc := NewTickerProcessorAdapter(resolver)

	payload := mustJSON(t, map[string]any{
		"s": "BTCUSDT",
		"E": 12345,
		"b": "10",
		"B": "1",
		"a": "11",
		"A": "2",
		"c": "10",
		"Q": "0.2",
	})
	msg, err := proc.Process(context.Background(), payload)
	require.NoError(t, err)
	routed := msg.(*corestreams.RoutedMessage)
	require.Equal(t, corestreams.RouteTickerUpdate, routed.Route)
	require.Equal(t, Ticker("BTC-USD"), routed.Topic)
}

func TestOrderAndBalanceProcessorAdapters(t *testing.T) {
	orderProc := NewOrderUpdateProcessorAdapter()
	orderPayload := mustJSON(t, map[string]any{
		"E": 12345,
		"s": "BTCUSDT",
		"S": "BUY",
		"o": "MARKET",
		"f": "GTC",
		"q": "1",
		"p": "100",
		"X": "FILLED",
		"i": 42,
		"z": "1",
		"L": "100",
		"T": 12346,
	})
	msg, err := orderProc.Process(context.Background(), orderPayload)
	require.NoError(t, err)
	routed := msg.(*corestreams.RoutedMessage)
	require.Equal(t, corestreams.RouteOrderUpdate, routed.Route)

	balanceProc := NewBalanceProcessorAdapter()
	balancePayload := mustJSON(t, map[string]any{
		"e": "balanceUpdate",
		"a": "usd",
		"d": "10",
		"E": 12345,
	})
	msg, err = balanceProc.Process(context.Background(), balancePayload)
	require.NoError(t, err)
	require.Equal(t, corestreams.RouteBalanceSnapshot, msg.(*corestreams.RoutedMessage).Route)

	oapPayload := mustJSON(t, map[string]any{
		"e": "outboundAccountPosition",
		"E": 12345,
		"B": []map[string]any{{"a": "usd", "f": "10", "l": "2"}},
	})
	msg, err = balanceProc.Process(context.Background(), oapPayload)
	require.NoError(t, err)
	require.Equal(t, corestreams.RouteBalanceSnapshot, msg.(*corestreams.RoutedMessage).Route)
}

type stubResolver struct {
	mapping map[string]string
	err     error
}

func (s stubResolver) CanonicalSymbol(symbol string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	if mapped, ok := s.mapping[symbol]; ok {
		return mapped, nil
	}
	return "", errors.New("symbol not found")
}
