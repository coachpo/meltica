package market_data

import (
	"encoding/json"
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/stretchr/testify/require"
)

func TestEncodeDecodeTradeEvent(t *testing.T) {
	ensureSymbolFixtures(t)
	price := mustRat(t, "56000.1234")
	qty := mustRat(t, "0.50")
	evt := &Event{
		Type:         EventTypeTrade,
		Venue:        core.ExchangeName("binance"),
		Symbol:       "BTC-USDT",
		NativeSymbol: "BTCUSDT",
		Timestamp:    time.Date(2024, 4, 1, 12, 0, 0, 0, time.UTC),
		Sequence:     42,
		Metadata:     map[string]string{"source": "test"},
		Payload: &TradePayload{
			Price:        price,
			Quantity:     qty,
			Side:         core.SideBuy,
			IsTaker:      true,
			VenueTradeID: "abc-123",
		},
	}

	encoded, err := EncodeEvent(evt)
	require.NoError(t, err)

	var envelope map[string]any
	require.NoError(t, json.Unmarshal(encoded, &envelope))
	require.Equal(t, "BTC-USDT", envelope["symbol"])
	require.Equal(t, "BTCUSDT", envelope["native_symbol"])
	require.Equal(t, "280000617/5000", envelope["price"])
	require.Equal(t, "1/2", envelope["quantity"])

	decoded, err := DecodeEvent(evt.Venue, encoded)
	require.NoError(t, err)
	trade := requirePayload[*TradePayload](t, decoded.Payload)
	require.Zero(t, trade.Price.Cmp(price))
	require.Zero(t, trade.Quantity.Cmp(qty))
	require.Equal(t, core.SideBuy, trade.Side)
	require.True(t, trade.IsTaker)
	require.Equal(t, "abc-123", trade.VenueTradeID)
}

func TestEncodeEventMissingRat(t *testing.T) {
	evt := &Event{
		Type:         EventTypeTrade,
		Venue:        core.ExchangeName("binance"),
		Symbol:       "BTC-USDT",
		NativeSymbol: "BTCUSDT",
		Timestamp:    time.Now(),
		Payload: &TradePayload{
			Price:    nil,
			Quantity: mustRat(t, "1"),
			Side:     core.SideSell,
		},
	}
	_, err := EncodeEvent(evt)
	require.Error(t, err)
	require.Contains(t, err.Error(), "trade.price")
}

func TestDecodeEventInvalidRat(t *testing.T) {
	ensureSymbolFixtures(t)
	badJSON := []byte(`{
		"type":"trade",
		"symbol":"BTC-USDT",
		"native_symbol":"BTCUSDT",
		"timestamp":"2024-04-01T00:00:00Z",
		"price":"not-a-rat",
		"quantity":"1",
		"side":"buy",
		"taker":false
	}`)
	_, err := DecodeEvent(core.ExchangeName("binance"), badJSON)
	require.Error(t, err)
}

func mustRat(t *testing.T, s string) *big.Rat {
	t.Helper()
	value, ok := new(big.Rat).SetString(s)
	if !ok {
		t.Fatalf("invalid rational %q", s)
	}
	return value
}
