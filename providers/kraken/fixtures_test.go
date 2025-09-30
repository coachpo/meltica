package kraken

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/yourorg/meltica/core"
)

func Test_Fixtures_Balances(t *testing.T) {
	path := os.Getenv("KRAKEN_FIXTURE_BALANCES")
	if path == "" {
		t.Skip("set KRAKEN_FIXTURE_BALANCES to a balance fixture payload captured with real credentials")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read balance fixture: %v", err)
	}
	var payload struct {
		Result map[string]string `json:"result"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode balance fixture: %v", err)
	}
	balances := TestOnlyNormalizeBalances(payload.Result)
	if len(balances) == 0 {
		t.Fatal("expected at least one normalized balance from fixture")
	}
}

func Test_Fixtures_Trades(t *testing.T) {
	path := os.Getenv("KRAKEN_FIXTURE_TRADESHISTORY")
	if path == "" {
		t.Skip("set KRAKEN_FIXTURE_TRADESHISTORY to a trades history fixture payload captured with real credentials")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read trades fixture: %v", err)
	}
	var payload struct {
		Result struct {
			Trades map[string]krakenTradeRecord `json:"trades"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("decode trades fixture: %v", err)
	}
	p := TestOnlyNewProvider()
	if len(payload.Result.Trades) == 0 {
		t.Fatal("fixture contained no trades")
	}
	trades := make([]core.Trade, 0, len(payload.Result.Trades))
	for _, tr := range payload.Result.Trades {
		canon := p.mapNativeToCanon(strings.ToUpper(tr.Pair))
		trade := core.Trade{
			Symbol:   canon,
			Price:    parseDecimalStr(tr.Price),
			Quantity: parseDecimalStr(tr.Volume),
		}
		trades = append(trades, trade)
	}
	if len(trades) == 0 {
		t.Fatal("expected normalized trades output")
	}
}
