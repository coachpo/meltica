package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/coachpo/meltica/providers/coinbase"
)

func TestCoinbaseCaptureFixtures(t *testing.T) {
	apiKey := os.Getenv("COINBASE_API_KEY")
	secret := os.Getenv("COINBASE_API_SECRET")
	passphrase := os.Getenv("COINBASE_API_PASSPHRASE")
	if apiKey == "" || secret == "" || passphrase == "" || apiKey == "placeholder" {
		t.Skip("set COINBASE_API_* env vars with real credentials to capture fixtures")
	}

	balancesOut := os.Getenv("COINBASE_FIXTURE_BALANCES_OUT")
	tradesOut := os.Getenv("COINBASE_FIXTURE_TRADES_OUT")
	if balancesOut == "" && tradesOut == "" {
		t.Skip("set COINBASE_FIXTURE_BALANCES_OUT or COINBASE_FIXTURE_TRADES_OUT to capture fixtures")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	p, err := coinbase.New(apiKey, secret, passphrase)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	spot := p.Spot(ctx)

	if balancesOut != "" {
		balances, err := spot.Balances(ctx)
		if err != nil {
			t.Fatalf("fetch balances: %v", err)
		}
		if err := writeJSON(balancesOut, balances); err != nil {
			t.Fatalf("write balances fixture: %v", err)
		}
		t.Logf("wrote balances fixture to %s", balancesOut)
	}

	if tradesOut != "" {
		symbol := os.Getenv("COINBASE_FIXTURE_SYMBOL")
		if symbol == "" {
			symbol = "BTC-USD"
		}
		trades, err := spot.Trades(ctx, symbol, 0)
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
		t.Logf("wrote %d trades for %s to %s", len(trades), symbol, tradesOut)
	}
}
