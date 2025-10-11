package wsrouting_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/exchanges/binance/wsrouting"
)

type record struct {
	typ     string
	payload []byte
}

type captureSink struct {
	mu    sync.Mutex
	items []record
}

func (c *captureSink) record(_ context.Context, typ string, payload []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, record{typ: typ, payload: append([]byte(nil), payload...)})
	return nil
}

func (c *captureSink) reset() {
	c.mu.Lock()
	c.items = nil
	c.mu.Unlock()
}

func (c *captureSink) last() (record, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.items) == 0 {
		return record{}, false
	}
	return c.items[len(c.items)-1], true
}

func TestPipelineParsesAndPublishes(t *testing.T) {
	sink := &captureSink{}
	pipeline, err := wsrouting.NewPipeline(sink.record)
	require.NoError(t, err)
	parser := pipeline.Parser()
	require.NotNil(t, parser)
	publish := pipeline.Publisher()
	require.NotNil(t, publish)

	testCases := []struct {
		name         string
		raw          []byte
		expectType   string
		expectStored []byte
	}{
		{
			name:         "combined trade",
			raw:          []byte(`{"stream":"btcusdt@trade","data":{"e":"trade","s":"BTCUSDT","p":"100.10","q":"0.5"}}`),
			expectType:   "binance.trade",
			expectStored: []byte(`{"e":"trade","s":"BTCUSDT","p":"100.10","q":"0.5"}`),
		},
		{
			name:         "order update",
			raw:          []byte(`{"e":"ORDER_TRADE_UPDATE","s":"LTCUSDT"}`),
			expectType:   "binance.user.order",
			expectStored: []byte(`{"e":"ORDER_TRADE_UPDATE","s":"LTCUSDT"}`),
		},
	}

	ctx := context.Background()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sink.reset()
			msg, err := parser.Parse(ctx, tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.expectType, msg.Type)
			stored, ok := msg.Payload["raw"]
			require.True(t, ok)
			payload, ok := stored.([]byte)
			require.True(t, ok)
			require.Equal(t, tc.expectStored, payload)
			require.NoError(t, publish(ctx, msg))
			recorded, ok := sink.last()
			require.True(t, ok)
			require.Equal(t, tc.expectType, recorded.typ)
			require.Equal(t, tc.expectStored, recorded.payload)
		})
	}
}

func TestUnsupportedEvent(t *testing.T) {
	pipeline, err := wsrouting.NewPipeline(func(context.Context, string, []byte) error { return nil })
	require.NoError(t, err)
	parser := pipeline.Parser()
	require.NotNil(t, parser)
	_, err = parser.Parse(context.Background(), []byte(`{"e":"unknown"}`))
	require.Error(t, err)
}
