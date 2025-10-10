package processors

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data"
)

func TestFundingProcessorProcessSuccess(t *testing.T) {
	processor := NewFundingProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	result, err := processor.Process(context.Background(), FundingFixture(t))
	require.NoError(t, err)
	payload, ok := result.(*market_data.FundingPayload)
	require.True(t, ok, "expected *FundingPayload")
	require.Zero(t, payload.Rate.Cmp(big.NewRat(1, 10000)))
	require.Equal(t, 8*time.Hour, payload.Interval)
	require.Equal(t, time.Date(2024, 4, 1, 20, 0, 0, 0, time.UTC), payload.EffectiveAt)
}

func TestFundingProcessorValidation(t *testing.T) {
	processor := NewFundingProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"rate":"","interval_hours":-1,"next_funding":"2024-01-01T00:00:00Z"}`)
	_, err := processor.Process(context.Background(), raw)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "rate value required")
}

func TestFundingProcessorContextCancelled(t *testing.T) {
	processor := NewFundingProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := processor.Process(ctx, FundingFixture(t))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
