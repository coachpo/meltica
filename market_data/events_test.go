package market_data

import (
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/stretchr/testify/require"
)

var symbolRegistryOnce sync.Once

func ensureSymbolFixtures(t *testing.T) {
	t.Helper()
	symbolRegistryOnce.Do(func() {
		defs := []core.SymbolDefinition{
			{Canonical: "BTC-USDT", Aliases: []string{"BTCUSD", "BTCUSDT"}},
			{Canonical: "ETH-USDT", Aliases: []string{"ETHUSDT"}},
		}
		for _, def := range defs {
			if _, err := core.CanonicalSymbolFor(def.Canonical); err == nil {
				continue
			}
			require.NoError(t, core.RegisterSymbol(def))
		}
	})
}

func TestParseTradeEventFixture(t *testing.T) {
	ensureSymbolFixtures(t)

	evt := decodeFixture(t, "trade.json")

	require.Equal(t, EventTypeTrade, evt.Type)
	require.Equal(t, core.ExchangeName("binance"), evt.Venue)
	require.Equal(t, "BTC-USDT", evt.Symbol)
	require.Equal(t, "BTCUSDT", evt.NativeSymbol)
	require.EqualValues(t, 41234567, evt.Sequence)
	require.True(t, evt.Timestamp.Equal(time.Date(2024, 4, 1, 12, 0, 0, 500000000, time.UTC)))

	trade := requirePayload[*TradePayload](t, evt.Payload)
	requireRatEqual(t, "56000.1234", trade.Price)
	requireRatEqual(t, "0.0025", trade.Quantity)
	require.Equal(t, core.SideBuy, trade.Side)
	require.True(t, trade.IsTaker)
	require.Equal(t, "12345", trade.VenueTradeID)
}

func TestParseOrderBookFixture(t *testing.T) {
	ensureSymbolFixtures(t)

	evt := decodeFixture(t, "book.json")

	require.Equal(t, EventTypeOrderBook, evt.Type)
	require.Equal(t, "ETH-USDT", evt.Symbol)
	require.EqualValues(t, 9001, evt.Sequence)

	book := requirePayload[*OrderBookPayload](t, evt.Payload)
	require.True(t, book.Snapshot)
	require.Len(t, book.Bids, 2)
	require.Len(t, book.Asks, 2)
	requireRatEqual(t, "3000.10", book.Bids[0].Price)
	requireRatEqual(t, "1.5", book.Bids[0].Qty)
	requireRatEqual(t, "3001.10", book.Asks[0].Price)
	requireRatEqual(t, "2.5", book.Asks[0].Qty)
}

func TestParseFundingFixture(t *testing.T) {
	ensureSymbolFixtures(t)

	evt := decodeFixture(t, "funding.json")

	require.Equal(t, EventTypeFunding, evt.Type)
	require.Equal(t, "BTC-USDT", evt.Symbol)

	funding := requirePayload[*FundingPayload](t, evt.Payload)
	requireRatEqual(t, "0.0001", funding.Rate)
	require.True(t, funding.EffectiveAt.Equal(time.Date(2024, 4, 1, 20, 0, 0, 0, time.UTC)))
	require.Equal(t, 8*time.Hour, funding.Interval)
}

func TestParseAccountFixture(t *testing.T) {
	ensureSymbolFixtures(t)

	evt := decodeFixture(t, "account.json")

	require.Equal(t, EventTypeAccountUpdate, evt.Type)

	account := requirePayload[*AccountPayload](t, evt.Payload)
	require.Equal(t, AccountBalanceDelta, account.Kind)
	require.Len(t, account.Balances, 2)
	requireRatEqual(t, "1000", account.Balances[0].Total)
	requireRatEqual(t, "800", account.Balances[0].Available)
	require.Equal(t, "USDT", account.Balances[0].Asset)
}

func decodeFixture(t *testing.T, name string) *Event {
	t.Helper()
	path := filepath.Join("testdata", name)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	evt, err := ParseEvent(core.ExchangeName("binance"), raw)
	require.NoError(t, err)
	return evt
}

func requirePayload[T any](t *testing.T, payload EventPayload) T {
	t.Helper()
	v, ok := payload.(T)
	require.Truef(t, ok, "unexpected payload type %T", payload)
	return v
}

func requireRatEqual(t *testing.T, expected string, actual *big.Rat) {
	t.Helper()
	require.NotNil(t, actual)
	want, ok := new(big.Rat).SetString(expected)
	require.Truef(t, ok, "invalid expected rational %s", expected)
	require.Zero(t, actual.Cmp(want), "expected %s got %s", want.String(), actual.String())
}
