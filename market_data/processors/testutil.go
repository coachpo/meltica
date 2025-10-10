package processors

import (
	"os"
	"path/filepath"
	"testing"
)

func loadFixture(tb testing.TB, name string) []byte {
	tb.Helper()
	path := filepath.Join("..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("load fixture %s: %v", name, err)
	}
	return data
}

func TradeFixture(tb testing.TB) []byte {
	return loadFixture(tb, "trade.json")
}

func OrderBookFixture(tb testing.TB) []byte {
	return loadFixture(tb, "book.json")
}

func AccountFixture(tb testing.TB) []byte {
	return loadFixture(tb, "account.json")
}

func FundingFixture(tb testing.TB) []byte {
	return loadFixture(tb, "funding.json")
}
