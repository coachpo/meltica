package core

import "testing"

func withSymbolState(t *testing.T, fn func()) {
	t.Helper()
	prevRegistry := registry
	registry = newSymbolRegistry()

	translatorMu.Lock()
	original := make(map[ExchangeName]SymbolTranslator, len(translators))
	for k, v := range translators {
		original[k] = v
	}
	translators = make(map[ExchangeName]SymbolTranslator)
	translatorMu.Unlock()

	t.Cleanup(func() {
		registry = prevRegistry
		translatorMu.Lock()
		translators = make(map[ExchangeName]SymbolTranslator, len(original))
		for k, v := range original {
			translators[k] = v
		}
		translatorMu.Unlock()
	})

	fn()
}

func TestRegisterSymbolAndResolve(t *testing.T) {
	withSymbolState(t, func() {
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
	})
}

func TestRegisterSymbolDuplicateAlias(t *testing.T) {
	withSymbolState(t, func() {
		if err := RegisterSymbol(SymbolDefinition{Canonical: "BTC-USDT", Aliases: []string{"BTCUSDT"}}); err != nil {
			t.Fatalf("register primary symbol: %v", err)
		}
		if err := RegisterSymbol(SymbolDefinition{Canonical: "ETH-USDT", Aliases: []string{"BTCUSDT"}}); err == nil {
			t.Fatalf("expected duplicate alias to be rejected")
		}
	})
}

func TestRegisterSymbolsBatch(t *testing.T) {
	withSymbolState(t, func() {
		err := RegisterSymbols(
			SymbolDefinition{Canonical: "ETH-USDT"},
			SymbolDefinition{Canonical: "invalid"},
		)
		if err == nil {
			t.Fatal("expected batch registration to fail on invalid definition")
		}
		if _, lookupErr := CanonicalSymbolFor("ETH-USDT"); lookupErr != nil {
			t.Fatalf("first symbol should still register: %v", lookupErr)
		}
	})
}

func TestInvalidCanonicalSymbol(t *testing.T) {
	withSymbolState(t, func() {
		if err := RegisterSymbol(SymbolDefinition{Canonical: "btc_usdt"}); err == nil {
			t.Fatalf("expected invalid canonical symbol to fail registration")
		}
	})
}

func TestCanonicalSymbolForUnknown(t *testing.T) {
	withSymbolState(t, func() {
		if _, err := CanonicalSymbolFor("UNKNOWN"); err == nil {
			t.Fatalf("expected error resolving unknown symbol")
		}
	})
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

type stubSymbolTranslator struct{}

func (stubSymbolTranslator) Native(canonical string) (string, error) {
	return "native-" + canonical, nil
}

func (stubSymbolTranslator) Canonical(native string) (string, error) {
	return native[len("native-"):], nil
}

func TestSymbolTranslatorRegistration(t *testing.T) {
	withSymbolState(t, func() {
		RegisterSymbolTranslator("", nil)
		RegisterSymbolTranslator("binance", stubSymbolTranslator{})

		native, err := NativeSymbol("binance", "BTC-USDT")
		if err != nil || native != "native-BTC-USDT" {
			t.Fatalf("unexpected native symbol %q err=%v", native, err)
		}

		canonical, err := CanonicalFromNative("binance", "native-BTC-USDT")
		if err != nil || canonical != "BTC-USDT" {
			t.Fatalf("unexpected canonical result %q err=%v", canonical, err)
		}

		if _, err := SymbolTranslatorFor("missing"); err == nil {
			t.Fatal("expected error for missing translator")
		}
	})
}

func TestCanonicalSymbolHelper(t *testing.T) {
	if got := CanonicalSymbol("btc", "usdt"); got != "BTC-USDT" {
		t.Fatalf("expected uppercase canonical symbol, got %s", got)
	}
}
