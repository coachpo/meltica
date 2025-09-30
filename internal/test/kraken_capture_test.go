package test

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/yourorg/meltica/providers/kraken"
)

func TestKrakenCaptureFixtures(t *testing.T) {
	apiKey := os.Getenv("KRAKEN_API_KEY")
	secret := os.Getenv("KRAKEN_API_SECRET")
	if apiKey == "" || secret == "" || apiKey == "placeholder" {
		t.Skip("set KRAKEN_API_KEY / KRAKEN_API_SECRET with real credentials to capture fixtures")
	}

	balancesOut := os.Getenv("KRAKEN_FIXTURE_BALANCES_OUT")
	tradesOut := os.Getenv("KRAKEN_FIXTURE_TRADES_OUT")
	if balancesOut == "" && tradesOut == "" {
		t.Skip("set KRAKEN_FIXTURE_BALANCES_OUT or KRAKEN_FIXTURE_TRADES_OUT to capture fixtures")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p, err := kraken.New(apiKey, secret)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}
	defer p.Close()

	spot := p.Spot(ctx)

	if balancesOut != "" {
		balances, err := spot.Balances(ctx)
		if err != nil {
			t.Fatalf("fetch balances: %v", err)
		}
		if err := writeJSON(balancesOut, balances); err != nil {
			t.Fatalf("write balances fixture: %v", err)
		}
		t.Logf("wrote Kraken balances fixture to %s", balancesOut)
	}

	if tradesOut != "" {
		symbol := os.Getenv("KRAKEN_FIXTURE_SYMBOL")
		if symbol == "" {
			symbol = "BTC-USD"
		}
		since := int64(0)
		if rawSince := os.Getenv("KRAKEN_FIXTURE_SINCE"); rawSince != "" {
			v, err := strconv.ParseInt(rawSince, 10, 64)
			if err != nil {
				t.Fatalf("invalid KRAKEN_FIXTURE_SINCE: %v", err)
			}
			since = v
		}
		trades, err := spot.Trades(ctx, symbol, since)
		if err != nil {
			t.Fatalf("fetch trades: %v", err)
		}
		const maxTrades = 100
		if len(trades) > maxTrades {
			trades = trades[:maxTrades]
		}
		if err := writeJSON(tradesOut, trades); err != nil {
			t.Fatalf("write trades fixture: %v", err)
		}
		t.Logf("wrote %d Kraken trades for %s to %s", len(trades), symbol, tradesOut)
	}
}
