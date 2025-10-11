package processors

import (
	"context"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestTradeProcessorPrecision(t *testing.T) {
	processor := NewTradeProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"price":"12345678901234567890.123456789","quantity":"0.000000000123456789","side":"sell","taker":false,"trade_id":"precision-1"}`)
	result, err := processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload := result.(*market_data.TradePayload)
	expectedPrice, ok := new(big.Rat).SetString("12345678901234567890.123456789")
	require.True(t, ok)
	require.Zero(t, payload.Price.Cmp(expectedPrice))
	expectedQty, ok := new(big.Rat).SetString("0.000000000123456789")
	require.True(t, ok)
	require.Zero(t, payload.Quantity.Cmp(expectedQty))
}

func TestOrderBookProcessorPrecision(t *testing.T) {
	processor := NewOrderBookProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"snapshot":false,"bids":[{"price":"12345.678901","quantity":"0.00000001"}],"asks":[{"price":"12346.000001","quantity":"1000000.00000001"}]}`)
	result, err := processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload := result.(*market_data.OrderBookPayload)
	require.False(t, payload.Snapshot)
	require.Len(t, payload.Bids, 1)
	require.Len(t, payload.Asks, 1)
	bidPrice, _ := new(big.Rat).SetString("12345.678901")
	require.Zero(t, payload.Bids[0].Price.Cmp(bidPrice))
	askQty, _ := new(big.Rat).SetString("1000000.00000001")
	require.Zero(t, payload.Asks[0].Qty.Cmp(askQty))
}

func TestAccountProcessorPrecision(t *testing.T) {
	processor := NewAccountProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"balances":[{"asset":"btc","total":"123456789.000000001","available":"123456788.999999999"}]}`)
	result, err := processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload := result.(*market_data.AccountPayload)
	require.Len(t, payload.Balances, 1)
	require.Equal(t, "BTC", payload.Balances[0].Asset)
	total, _ := new(big.Rat).SetString("123456789.000000001")
	require.Zero(t, payload.Balances[0].Total.Cmp(total))
	available, _ := new(big.Rat).SetString("123456788.999999999")
	require.Zero(t, payload.Balances[0].Available.Cmp(available))
}

func TestFundingProcessorPrecision(t *testing.T) {
	processor := NewFundingProcessor()
	require.NoError(t, processor.Initialize(context.Background()))

	raw := []byte(`{"rate":"0.000000001","interval_hours":12,"next_funding":"2024-04-01T00:00:00Z"}`)
	result, err := processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload := result.(*market_data.FundingPayload)
	rate, _ := new(big.Rat).SetString("0.000000001")
	require.Zero(t, payload.Rate.Cmp(rate))
	require.Equal(t, 12*60*60, int(payload.Interval.Seconds()))
}
