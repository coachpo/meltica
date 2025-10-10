package processors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
)

func TestTradeProcessorMalformedPayload(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	_, err := processor.Process(context.Background(), []byte("{"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "trade payload decode failed")

	_, err = processor.Process(context.Background(), TradeFixture(t))
	require.NoError(t, err)
}

func TestOrderBookProcessorMalformedPayload(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	_, err := processor.Process(context.Background(), []byte("{"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "order book payload decode failed")

	_, err = processor.Process(context.Background(), OrderBookFixture(t))
	require.NoError(t, err)
}

func TestAccountProcessorMalformedPayload(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	_, err := processor.Process(context.Background(), []byte("{"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "account payload decode failed")

	_, err = processor.Process(context.Background(), AccountFixture(t))
	require.NoError(t, err)
}

func TestFundingProcessorMalformedPayload(t *testing.T) {
	processor := NewFundingProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	_, err := processor.Process(context.Background(), []byte("{"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "funding payload decode failed")

	_, err = processor.Process(context.Background(), FundingFixture(t))
	require.NoError(t, err)
}
