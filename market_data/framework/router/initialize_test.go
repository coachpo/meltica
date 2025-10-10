package router

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestInitializeRouterRegistersProcessors(t *testing.T) {
	rt, err := InitializeRouter()
	require.NoError(t, err)

	for _, id := range []string{"trade", "orderbook", "account", "funding"} {
		reg := rt.Lookup(id)
		require.NotNil(t, reg)
		require.Equal(t, id, reg.MessageTypeID)
		require.Equal(t, ProcessorStatusAvailable, reg.Status)
	}

	messageType, err := rt.Detect(loadTestPayload(t, "trade.json"))
	require.NoError(t, err)
	require.Equal(t, "trade", messageType)

	reg := rt.Lookup(messageType)
	result, err := reg.Processor.Process(context.Background(), loadTestPayload(t, "trade.json"))
	require.NoError(t, err)
	_, ok := result.(*market_data.TradePayload)
	require.True(t, ok)
}

func TestInitializeRouterDefaultProcessor(t *testing.T) {
	rt, err := InitializeRouter()
	require.NoError(t, err)

	reg := rt.Lookup("unknown-type")
	require.NotNil(t, reg)
	require.Equal(t, "default", reg.MessageTypeID)
	require.Equal(t, ProcessorStatusAvailable, reg.Status)

	raw := []byte("raw")
	result, err := reg.Processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload, ok := result.(*market_data.RawPayload)
	require.True(t, ok)
	require.Equal(t, []byte("raw"), payload.Data)
	raw[0] = 'X'
	require.Equal(t, byte('r'), payload.Data[0])
}
