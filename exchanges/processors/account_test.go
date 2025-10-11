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

func TestAccountProcessorProcessSuccess(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	result, err := processor.Process(context.Background(), AccountFixture(t))
	require.NoError(t, err)
	payload, ok := result.(*market_data.AccountPayload)
	require.True(t, ok, "expected *AccountPayload")
	require.Equal(t, market_data.AccountBalanceDelta, payload.Kind)
	require.Len(t, payload.Balances, 2)
	require.Equal(t, "USDT", payload.Balances[0].Asset)
	require.Zero(t, payload.Balances[0].Total.Cmp(big.NewRat(1000, 1)))
	require.Zero(t, payload.Balances[0].Available.Cmp(big.NewRat(800, 1)))
}

func TestAccountProcessorValidation(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"balances":[{"asset":"usd","total":"1","available":"2"}]}`)
	_, err := processor.Process(context.Background(), raw)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "balances.available cannot exceed total")
}

func TestAccountProcessorRequiresBalances(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	_, err := processor.Process(context.Background(), []byte(`{"balances":[]}`))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Contains(t, e.Message, "balances must contain at least one entry")
}

func TestAccountProcessorContextCancelled(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := processor.Process(ctx, AccountFixture(t))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
