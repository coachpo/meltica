package processors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data"
)

func TestUserStory2Acceptance(t *testing.T) {
	t.Run("AS2.1_TradeValidation", func(t *testing.T) {
		processor := NewTradeProcessor()
		require.NoError(t, processor.Initialize(context.Background()))
		result, err := processor.Process(context.Background(), TradeFixture(t))
		require.NoError(t, err)
		payload := result.(*market_data.TradePayload)
		require.Equal(t, "12345", payload.VenueTradeID)

		_, err = processor.Process(context.Background(), []byte(`{"price":"1","quantity":"1","side":"buy","trade_id":""}`))
		require.Error(t, err)
		var e *errs.E
		require.True(t, errors.As(err, &e))
		require.Contains(t, e.Message, "trade_id value required")
	})

	t.Run("AS2.2_OrderBookValidation", func(t *testing.T) {
		processor := NewOrderBookProcessor()
		require.NoError(t, processor.Initialize(context.Background()))
		result, err := processor.Process(context.Background(), OrderBookFixture(t))
		require.NoError(t, err)
		payload := result.(*market_data.OrderBookPayload)
		require.True(t, payload.Snapshot)
		require.Len(t, payload.Bids, 2)
		require.Len(t, payload.Asks, 2)

		_, err = processor.Process(context.Background(), []byte(`{"snapshot":false,"bids":[],"asks":[]}`))
		require.Error(t, err)
		var e *errs.E
		require.True(t, errors.As(err, &e))
		require.Contains(t, e.Message, "bids must contain at least one level")
	})

	t.Run("AS2.3_AccountValidation", func(t *testing.T) {
		processor := NewAccountProcessor()
		require.NoError(t, processor.Initialize(context.Background()))
		result, err := processor.Process(context.Background(), AccountFixture(t))
		require.NoError(t, err)
		payload := result.(*market_data.AccountPayload)
		require.Equal(t, market_data.AccountBalanceDelta, payload.Kind)
		require.Len(t, payload.Balances, 2)

		_, err = processor.Process(context.Background(), []byte(`{"balances":[{"asset":"btc","total":"1","available":"2"}]}`))
		require.Error(t, err)
		var e *errs.E
		require.True(t, errors.As(err, &e))
		require.Contains(t, e.Message, "balances.available cannot exceed total")
	})

	t.Run("AS2.4_MalformedHandling", func(t *testing.T) {
		processor := NewTradeProcessor()
		require.NoError(t, processor.Initialize(context.Background()))

		_, err := processor.Process(context.Background(), []byte("{"))
		require.Error(t, err)
		var e *errs.E
		require.True(t, errors.As(err, &e))
		require.Contains(t, e.Message, "trade payload decode failed")

		_, err = processor.Process(context.Background(), TradeFixture(t))
		require.NoError(t, err)
	})
}
