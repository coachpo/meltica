package core

import "testing"

func resetRegistry() { registry = newSymbolRegistry() }

func TestRegisterSymbolAndResolve(t *testing.T) {
	resetRegistry()

	def := SymbolDefinition{
		Canonical: "BTC-USDT",
		Aliases:   []string{"btc/usdt", "BTCUSDT"},
	}
	if err := RegisterSymbol(def); err != nil {
		t.Fatalf("register symbol: %v", err)
	}

	cases := []string{"BTC-USDT", "btc-usdt", "btc/usdt", "btcusdt"}
	for _, symbol := range cases {
		canonical, err := CanonicalSymbolFor(symbol)
		if err != nil {
			t.Fatalf("resolve %q: %v", symbol, err)
		}
		if canonical != "BTC-USDT" {
			t.Fatalf("expected canonical BTC-USDT for %q, got %s", symbol, canonical)
		}
	}

	aliases := SymbolAliases("btc-usdt")
	want := map[string]bool{"BTC/USDT": true, "BTCUSDT": true}
	if len(aliases) != len(want) {
		t.Fatalf("expected %d aliases, got %d", len(want), len(aliases))
	}
	for _, alias := range aliases {
		if !want[alias] {
			t.Fatalf("unexpected alias %q returned", alias)
		}
		delete(want, alias)
	}
	if len(want) != 0 {
		t.Fatalf("expected aliases missing from result: %v", want)
	}
}

func TestRegisterSymbolDuplicateAlias(t *testing.T) {
	resetRegistry()
	if err := RegisterSymbol(SymbolDefinition{Canonical: "BTC-USDT", Aliases: []string{"BTCUSDT"}}); err != nil {
		t.Fatalf("register primary symbol: %v", err)
	}
	if err := RegisterSymbol(SymbolDefinition{Canonical: "ETH-USDT", Aliases: []string{"BTCUSDT"}}); err == nil {
		t.Fatalf("expected duplicate alias to be rejected")
	}
}

func TestInvalidCanonicalSymbol(t *testing.T) {
	resetRegistry()
	if err := RegisterSymbol(SymbolDefinition{Canonical: "btc_usdt"}); err == nil {
		t.Fatalf("expected invalid canonical symbol to fail registration")
	}
}

func TestCanonicalSymbolForUnknown(t *testing.T) {
	resetRegistry()
	if _, err := CanonicalSymbolFor("UNKNOWN"); err == nil {
		t.Fatalf("expected error resolving unknown symbol")
	}
}

func TestParseCanonicalSymbol(t *testing.T) {
	base, quote, err := ParseCanonicalSymbol("btC-usdt")
	if err != nil {
		t.Fatalf("parse canonical symbol: %v", err)
	}
	if base != "BTC" || quote != "USDT" {
		t.Fatalf("expected BTC/USDT, got %s/%s", base, quote)
	}
}

func TestIsCanonicalSymbol(t *testing.T) {
	if !IsCanonicalSymbol("ETH-USDC") {
		t.Fatalf("expected ETH-USDC to be canonical")
	}
	if IsCanonicalSymbol("ethusdc") {
		t.Fatalf("expected lowercase symbol to fail canonical validation")
	}
	if IsCanonicalSymbol("ETH/USDC") {
		t.Fatalf("expected slash separated symbol to fail canonical validation")
	}
}
