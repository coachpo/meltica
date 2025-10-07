package main

import (
	"math/big"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
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
	reqs := buildDefaultSubscriptions([]string{"BTC-USDT", "ETH-USDT"}, instruments)
	if len(reqs) != 2 {
		t.Fatalf("expected 2 subscription requests, got %d", len(reqs))
	}
	seen := make(map[string]struct{}, len(reqs))
	for _, req := range reqs {
		seen[req.Symbol] = struct{}{}
		if !req.Book {
			t.Fatalf("expected %s to include order book subscription", req.Symbol)
		}
		if req.Config.BookDepthLevels != 2 {
			t.Fatalf("expected %s depth levels to be 2, got %d", req.Symbol, req.Config.BookDepthLevels)
		}
		if req.Config.UpdateInterval != defaultBookLogInterval {
			t.Fatalf("expected %s to use default book interval, got %s", req.Symbol, req.Config.UpdateInterval)
		}
	}
	if _, ok := seen["BTC-USDT"]; !ok {
		t.Fatalf("expected BTC-USDT in subscriptions")
	}
	if _, ok := seen["ETH-USDT"]; !ok {
		t.Fatalf("expected ETH-USDT in subscriptions")
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

func TestParseSymbolList(t *testing.T) {
	got := parseSymbolList("btc-usdt, ETH-USDT, btc-usdt , ")
	if len(got) != 2 {
		t.Fatalf("expected 2 symbols, got %v", got)
	}
	if got[0] != "BTC-USDT" || got[1] != "ETH-USDT" {
		t.Fatalf("unexpected ordering: %v", got)
	}
	if parseSymbolList("   ") != nil {
		t.Fatalf("expected nil for empty input")
	}
}

func TestParseControlCommand(t *testing.T) {
	cases := []struct {
		input   string
		kind    userCommandKind
		symbols []string
	}{
		{input: "pause btc-usdt eth-usdt", kind: commandPause, symbols: []string{"BTC-USDT", "ETH-USDT"}},
		{input: "resume btc-usdt", kind: commandResume, symbols: []string{"BTC-USDT"}},
		{input: "help", kind: commandHelp},
		{input: "quit", kind: commandQuit},
		{input: "subscribe btc-usdt btc-usdt", kind: commandSubscribe, symbols: []string{"BTC-USDT"}},
		{input: "subscribe 1 2", kind: commandSubscribe, symbols: []string{"1", "2"}},
	}
	for _, tc := range cases {
		cmd := parseControlCommand(tc.input)
		if cmd.kind != tc.kind {
			t.Fatalf("input %q: expected kind %v got %v", tc.input, tc.kind, cmd.kind)
		}
		if len(tc.symbols) != len(cmd.symbols) {
			t.Fatalf("input %q: expected symbols %v got %v", tc.input, tc.symbols, cmd.symbols)
		}
		for i := range tc.symbols {
			if tc.symbols[i] != cmd.symbols[i] {
				t.Fatalf("input %q: expected symbol %s got %s", tc.input, tc.symbols[i], cmd.symbols[i])
			}
		}
	}
	unknown := parseControlCommand("nonsense")
	if unknown.kind != commandUnknown {
		t.Fatalf("expected unknown command for invalid input")
	}
}

func TestResolveCommandSymbolsWithMenu(t *testing.T) {
	state := &controlState{}
	options := []string{"BTC-USDT", "ETH-USDT"}
	state.setMenu(commandSubscribe, options)
	allowed := makeSymbolSet(options)
	inputs := []string{"1", "eth-usdt"}
	got := resolveCommandSymbols(inputs, commandSubscribe, state, allowed)
	if len(got) != 2 || got[0] != "BTC-USDT" || got[1] != "ETH-USDT" {
		t.Fatalf("unexpected resolution: %v", got)
	}
	state.setMenu(commandPause, options)
	if resolved := resolveCommandSymbols([]string{"1"}, commandSubscribe, state, allowed); len(resolved) != 0 {
		t.Fatalf("expected no resolution when menu kind mismatched")
	}
}

func TestFilterAvailableSymbols(t *testing.T) {
	catalog := []string{"BTC-USDT", "ETH-USDT", "SOL-USDT"}
	active := []symbolSubscription{{Symbol: "BTC-USDT"}}
	got := filterAvailableSymbols(catalog, active)
	if len(got) != 2 || got[0] != "ETH-USDT" || got[1] != "SOL-USDT" {
		t.Fatalf("unexpected available symbols: %v", got)
	}
}
