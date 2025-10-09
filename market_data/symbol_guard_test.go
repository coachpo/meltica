package market_data

import (
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/stretchr/testify/require"
)

func TestSymbolGuardHaltsOnMidSessionDrift(t *testing.T) {
	ensureSymbolFixtures(t)

	feedID := "public.trades:BTCUSDT"
	first := &Event{
		Type:         EventTypeTrade,
		Venue:        core.ExchangeName("binance"),
		Symbol:       "BTC-USDT",
		NativeSymbol: "BTC-USDT",
		Timestamp:    time.Now(),
		Payload: &TradePayload{
			Price:    testRat("42000"),
			Quantity: testRat("0.1"),
			Side:     core.SideBuy,
		},
	}
	drifted := &Event{
		Type:         EventTypeTrade,
		Venue:        core.ExchangeName("binance"),
		Symbol:       "BTC-USDT",
		NativeSymbol: "BTCUSDT",
		Timestamp:    time.Now().Add(time.Millisecond),
		Payload: &TradePayload{
			Price:    testRat("42000"),
			Quantity: testRat("0.1"),
			Side:     core.SideSell,
		},
	}

	var alerts []core.SymbolDriftAlert
	guard := core.NewSymbolGuard(core.ExchangeName("binance"), func(alert core.SymbolDriftAlert) {
		alerts = append(alerts, alert)
	})

	forwarded := 0
	if err := guard.Check(feedID, first); err != nil {
		t.Fatalf("unexpected error for initial event: %v", err)
	} else {
		forwarded++
	}

	err := guard.Check(feedID, drifted)
	require.Error(t, err)

	var driftErr *core.SymbolDriftError
	require.ErrorAs(t, err, &driftErr)
	require.True(t, driftErr.HaltFeed)
	require.Equal(t, core.ExchangeName("binance"), driftErr.Alert.Exchange)
	require.Equal(t, feedID, driftErr.Alert.Feed)
	require.Equal(t, "BTC-USDT", driftErr.Alert.ExpectedSymbol)
	require.Equal(t, "BTCUSDT", driftErr.Alert.ObservedSymbol)
	require.Equal(t, "BTC-USDT", driftErr.Alert.Canonical)
	require.Contains(t, driftErr.Alert.Remediation, "Symbol format changed from BTC-USDT to BTCUSDT")

	require.Len(t, alerts, 1)
	require.Equal(t, driftErr.Alert, alerts[0])

	if err == nil {
		forwarded++
	}
	require.Equal(t, 1, forwarded, "drifted event must not be forwarded")
}

func testRat(v string) *big.Rat {
	r, _ := new(big.Rat).SetString(v)
	return r
}
