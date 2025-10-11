package processors

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data"
)

func TestTradeProcessorProcessSuccess(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	result, err := processor.Process(context.Background(), TradeFixture(t))
	require.NoError(t, err)
	payload, ok := result.(*market_data.TradePayload)
	require.True(t, ok, "expected *TradePayload")

	expectedPrice := big.NewRat(560001234, 10000)
	require.Zero(t, payload.Price.Cmp(expectedPrice))

	expectedQty := big.NewRat(25, 10000)
	require.Zero(t, payload.Quantity.Cmp(expectedQty))
	require.Equal(t, core.SideBuy, payload.Side)
	require.True(t, payload.IsTaker)
	require.Equal(t, "12345", payload.VenueTradeID)
}

func TestTradeProcessorProcessInvalidJSON(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	_, err := processor.Process(context.Background(), []byte("{invalid"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "trade payload decode failed")
}

func TestTradeProcessorValidationErrors(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	payload := []byte(`{"price":"0","quantity":"-1","side":"noop"}`)
	_, err := processor.Process(context.Background(), payload)
	require.Error(t, err)

	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "price must be positive")
}

func TestTradeProcessorRequiresTradeID(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	payload := []byte(`{"price":"1","quantity":"1","side":"buy","taker":false,"trade_id":"   "}`)
	_, err := processor.Process(context.Background(), payload)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "trade_id value required")
}

func TestTradeProcessorContextCancelled(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := processor.Process(ctx, TradeFixture(t))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
