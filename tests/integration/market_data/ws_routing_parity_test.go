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
)

func TestWSRoutingParityFixtures(t *testing.T) {
	ctx := context.Background()
	sink := &capturedSink{}
	pipeline, err := bnwsrouting.NewPipeline(sink.record)
	require.NoError(t, err)
	parser := pipeline.Parser()
	require.NotNil(t, parser)
	publisher := pipeline.Publisher()
	require.NotNil(t, publisher)

	deps := parityDeps{}
	table := bnwsrouting.NewRoutingTable()
	require.NoError(t, bnrouting.RegisterProcessors(table, deps))

	testCases := []struct {
		name         string
		raw          []byte
		expectType   string
		expectStored []byte
	}{
		{
			name: "trade combined",
			raw: mustJSON(map[string]any{
				"stream": "btcusdt@trade",
				"data": map[string]any{
					"e": "trade",
					"s": "BTCUSDT",
					"p": "56000.1234",
					"q": "0.0025",
				},
			}),
			expectType:   "binance.trade",
			expectStored: mustJSON(map[string]any{"e": "trade", "s": "BTCUSDT", "p": "56000.1234", "q": "0.0025"}),
		},
		{
			name: "depth update",
			raw: mustJSON(map[string]any{
				"e": "depthUpdate",
				"s": "ETHUSDT",
				"b": [][]any{{"3000.10", "1.5"}},
				"a": [][]any{{"3001.10", "2.5"}},
			}),
			expectType:   "binance.orderbook",
			expectStored: mustJSON(map[string]any{"e": "depthUpdate", "s": "ETHUSDT", "b": [][]any{{"3000.10", "1.5"}}, "a": [][]any{{"3001.10", "2.5"}}}),
		},
		{
			name: "ticker update",
			raw: mustJSON(map[string]any{
				"e": "24hrTicker",
				"s": "LTCUSDT",
				"b": "90.5",
				"a": "91.2",
			}),
			expectType:   "binance.ticker",
			expectStored: mustJSON(map[string]any{"e": "24hrTicker", "s": "LTCUSDT", "b": "90.5", "a": "91.2"}),
		},
		{
			name: "order update",
			raw: mustJSON(map[string]any{
				"e": "ORDER_TRADE_UPDATE",
				"s": "BTCUSDT",
			}),
			expectType:   "binance.user.order",
			expectStored: mustJSON(map[string]any{"e": "ORDER_TRADE_UPDATE", "s": "BTCUSDT"}),
		},
		{
			name: "balance update",
			raw: mustJSON(map[string]any{
				"e": "balanceUpdate",
				"a": "usdt",
			}),
			expectType:   "binance.user.balance",
			expectStored: mustJSON(map[string]any{"e": "balanceUpdate", "a": "usdt"}),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink.reset()
			msgType, err := table.Detect(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.expectType, msgType)

			parsed, err := parser.Parse(ctx, tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.expectType, parsed.Type)
			stored, ok := parsed.Payload["raw"].([]byte)
			require.True(t, ok)
			require.Equal(t, tc.expectStored, stored)

			require.NoError(t, publisher(ctx, parsed))
			recordedType, recordedPayload, ok := sink.last()
			require.True(t, ok)
			require.Equal(t, tc.expectType, recordedType)
			require.Equal(t, tc.expectStored, recordedPayload)
		})
	}
}

type capturedSink struct {
	items []struct {
		typ string
		raw []byte
	}
}

func (c *capturedSink) record(_ context.Context, typ string, payload []byte) error {
	c.items = append(c.items, struct {
		typ string
		raw []byte
	}{typ: typ, raw: append([]byte(nil), payload...)})
	return nil
}

func (c *capturedSink) reset() {
	c.items = nil
}

func (c *capturedSink) last() (string, []byte, bool) {
	if len(c.items) == 0 {
		return "", nil, false
	}
	item := c.items[len(c.items)-1]
	return item.typ, item.raw, true
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
