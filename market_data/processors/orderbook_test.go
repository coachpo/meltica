package processors

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data"
)

func TestOrderBookProcessorProcessSuccess(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	result, err := processor.Process(context.Background(), OrderBookFixture(t))
	require.NoError(t, err)
	payload, ok := result.(*market_data.OrderBookPayload)
	require.True(t, ok, "expected *OrderBookPayload")
	require.True(t, payload.Snapshot)
	require.Len(t, payload.Bids, 2)
	require.Len(t, payload.Asks, 2)
	expectedBid := big.NewRat(300010, 100)
	require.Zero(t, payload.Bids[0].Price.Cmp(expectedBid))
}

func TestOrderBookProcessorValidation(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"bids":[{"price":"0","quantity":"1"}],"asks":[]}`)
	_, err := processor.Process(context.Background(), raw)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "asks must contain at least one level")
}

func TestOrderBookProcessorRequiresLevels(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	raw := []byte(`{"bids":[],"asks":[{"price":"1","quantity":"1"}]}`)
	_, err := processor.Process(context.Background(), raw)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "bids must contain at least one level")
}

func TestOrderBookProcessorContextCancelled(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := processor.Process(ctx, OrderBookFixture(t))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
