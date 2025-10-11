package integration

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	bnwsrouting "github.com/coachpo/meltica/exchanges/binance/wsrouting"
	wsroutingtest "github.com/coachpo/meltica/internal/testhelpers/wsrouting"
)

func TestWSRoutingParityFixtures(t *testing.T) {
	recorder := wsroutingtest.NewRecorder()
	pipeline, err := bnwsrouting.NewPipeline(recorder.Sink())
	require.NoError(t, err)
	deps := parityDeps{}
	table := bnwsrouting.NewRoutingTable()
	require.NoError(t, bnrouting.RegisterProcessors(table, deps))

	cases := []wsroutingtest.ConformanceCase{
		{
			Name: "trade combined",
			Raw: mustJSON(map[string]any{
				"stream": "btcusdt@trade",
				"data": map[string]any{
					"e": "trade",
					"s": "BTCUSDT",
					"p": "56000.1234",
					"q": "0.0025",
				},
			}),
			ExpectedType:   "binance.trade",
			ExpectedStored: mustJSON(map[string]any{"e": "trade", "s": "BTCUSDT", "p": "56000.1234", "q": "0.0025"}),
		},
		{
			Name: "depth update",
			Raw: mustJSON(map[string]any{
				"e": "depthUpdate",
				"s": "ETHUSDT",
				"b": [][]any{{"3000.10", "1.5"}},
				"a": [][]any{{"3001.10", "2.5"}},
			}),
			ExpectedType:   "binance.orderbook",
			ExpectedStored: mustJSON(map[string]any{"e": "depthUpdate", "s": "ETHUSDT", "b": [][]any{{"3000.10", "1.5"}}, "a": [][]any{{"3001.10", "2.5"}}}),
		},
		{
			Name: "ticker update",
			Raw: mustJSON(map[string]any{
				"e": "24hrTicker",
				"s": "LTCUSDT",
				"b": "90.5",
				"a": "91.2",
			}),
			ExpectedType:   "binance.ticker",
			ExpectedStored: mustJSON(map[string]any{"e": "24hrTicker", "s": "LTCUSDT", "b": "90.5", "a": "91.2"}),
		},
		{
			Name: "order update",
			Raw: mustJSON(map[string]any{
				"e": "ORDER_TRADE_UPDATE",
				"s": "BTCUSDT",
			}),
			ExpectedType:   "binance.user.order",
			ExpectedStored: mustJSON(map[string]any{"e": "ORDER_TRADE_UPDATE", "s": "BTCUSDT"}),
		},
		{
			Name: "balance update",
			Raw: mustJSON(map[string]any{
				"e": "balanceUpdate",
				"a": "usdt",
			}),
			ExpectedType:   "binance.user.balance",
			ExpectedStored: mustJSON(map[string]any{"e": "balanceUpdate", "a": "usdt"}),
		},
	}

	adapter := wsroutingtest.Adapter{
		Detector: table,
		Parser:   pipeline.Parser(),
		Publish:  pipeline.Publisher(),
		Recorder: recorder,
	}

	wsroutingtest.AssertAdapterConformance(t, adapter, cases)
}

type parityDeps struct{}

func (parityDeps) BookDepthSnapshot(context.Context, string, int) (corestreams.BookEvent, int64, error) {
	return corestreams.BookEvent{}, 0, nil
}

func (parityDeps) CanonicalSymbol(symbol string) (string, error) { return symbol, nil }

func (parityDeps) NativeSymbol(canonical string) (string, error) { return canonical, nil }

func (parityDeps) NativeTopic(topic core.Topic) (string, error) { return string(topic), nil }

func (parityDeps) CreateListenKey(context.Context) (string, error) { return "listen-key", nil }

func (parityDeps) KeepAliveListenKey(context.Context, string) error { return nil }

func (parityDeps) CloseListenKey(context.Context, string) error { return nil }

func mustJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}
