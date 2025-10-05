package main

import (
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

func TestPrecisionFormatterUsesInstrumentScales(t *testing.T) {
	instruments := map[string]core.Instrument{
		"BTC-USDT": {PriceScale: 5, QtyScale: 8},
	}
	formatter := newPrecisionFormatter(instruments)

	priceValue := big.NewRat(123456789, 10000) // 12345.6789
	if got, want := formatter.price("BTC-USDT", priceValue), numeric.Format(priceValue, 5); got != want {
		t.Fatalf("price formatting mismatch, got %s want %s", got, want)
	}

	qtyValue := big.NewRat(12345, 100000000) // 0.00012345
	if got, want := formatter.quantity("BTC-USDT", qtyValue), numeric.Format(qtyValue, 8); got != want {
		t.Fatalf("quantity formatting mismatch, got %s want %s", got, want)
	}
}

func TestPrecisionFormatterFallbacks(t *testing.T) {
	formatter := newPrecisionFormatter(nil)

	value := big.NewRat(1, 3)
	if got, want := formatter.price("UNKNOWN", value), numeric.Format(value, fallbackPriceScale); got != want {
		t.Fatalf("fallback price formatting mismatch, got %s want %s", got, want)
	}
	if got, want := formatter.quantity("UNKNOWN", value), numeric.Format(value, fallbackQuantityScale); got != want {
		t.Fatalf("fallback quantity formatting mismatch, got %s want %s", got, want)
	}
	if got := formatter.price("UNKNOWN", nil); got != "0" {
		t.Fatalf("expected nil value to format as 0, got %s", got)
	}
}

func TestResolveEventSymbol(t *testing.T) {
	if got := resolveEventSymbol("BTC-USDT", "trade:ETH-USDT"); got != "BTC-USDT" {
		t.Fatalf("expected explicit symbol to win, got %s", got)
	}
	if got := resolveEventSymbol("", "ticker:ETH-USDT"); got != "ETH-USDT" {
		t.Fatalf("expected symbol from topic, got %s", got)
	}
	if got := resolveEventSymbol("", "invalid"); got != "" {
		t.Fatalf("expected empty string on malformed topic, got %s", got)
	}
}

func TestBuildDefaultSubscriptionsIncludesAvailableSymbols(t *testing.T) {
	instruments := map[string]core.Instrument{
		"BTC-USDT": {},
		"ETH-USDT": {},
	}
	reqs := buildDefaultSubscriptions("BTC-USDT", instruments)
	if len(reqs) == 0 {
		t.Fatalf("expected at least one subscription request")
	}
	foundPrimary := false
	for _, req := range reqs {
		if req.Symbol == "BTC-USDT" {
			foundPrimary = true
			if !req.Book {
				t.Fatalf("expected BTC-USDT to include order book subscription")
			}
			if req.Config.DepthLevels <= 0 {
				t.Fatalf("expected depth levels to be configured")
			}
			if req.Config.UpdateInterval != defaultBookLogInterval {
				t.Fatalf("expected default book interval, got %s", req.Config.UpdateInterval)
			}
		} else if req.Symbol == "ETH-USDT" {
			if req.Config.UpdateInterval != 500*time.Millisecond {
				t.Fatalf("expected throttled interval, got %s", req.Config.UpdateInterval)
			}
		}
	}
	if !foundPrimary {
		t.Fatalf("expected primary symbol in subscriptions")
	}
}

func TestChunkTopics(t *testing.T) {
	topics := []string{"a", "b", "c", "d", "e"}
	chunks := chunkTopics(topics, 2)
	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if len(chunks[0]) != 2 || len(chunks[1]) != 2 || len(chunks[2]) != 1 {
		t.Fatalf("unexpected chunk sizes: %#v", chunks)
	}
	if chunkTopics(nil, 2) != nil {
		t.Fatalf("expected nil chunk for empty topics")
	}
}

func TestFormatBookLevels(t *testing.T) {
	levels := []bookLevel{
		{price: big.NewRat(12345, 1), qty: big.NewRat(1, 10)},
		{price: big.NewRat(12340, 1), qty: big.NewRat(2, 10)},
	}
	formatter := newPrecisionFormatter(map[string]core.Instrument{
		"BTC-USDT": {PriceScale: 2, QtyScale: 3},
	})
	out := formatBookLevels("BTC-USDT", levels, formatter)
	if out == "-" || out == "" {
		t.Fatalf("expected formatted book levels, got %q", out)
	}
	if got := formatBookLevels("BTC-USDT", nil, formatter); got != "-" {
		t.Fatalf("expected '-' for empty levels, got %s", got)
	}
}
