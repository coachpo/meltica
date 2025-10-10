package core

import "testing"

type snapshotStub struct {
	canonical string
	native    string
}

func (s snapshotStub) Canonical() string { return s.canonical }

func (s snapshotStub) Native() string { return s.native }

func TestSymbolDriftErrorString(t *testing.T) {
	err := (&SymbolDriftError{Alert: SymbolDriftAlert{Exchange: "binance", Feed: "trades", ExpectedSymbol: "BTCUSDT", ObservedSymbol: "ETHUSDT"}}).Error()
	want := "symbol drift on binance feed trades: expected BTCUSDT, observed ETHUSDT"
	if err != want {
		t.Fatalf("unexpected error string: %s", err)
	}
	if (&SymbolDriftError{}).Error() == "" {
		t.Fatal("expected non-empty string")
	}
	if (*SymbolDriftError)(nil).Error() != "symbol drift" {
		t.Fatal("nil receiver should use default message")
	}
}

func TestSymbolGuardCheckInitializesAndDetectsDrift(t *testing.T) {
	var captured SymbolDriftAlert
	guard := NewSymbolGuard("binance", func(alert SymbolDriftAlert) {
		captured = alert
	})

	if err := guard.Check("trade", snapshotStub{canonical: " btc-usdt ", native: " btcusdt "}); err != nil {
		t.Fatalf("initial check returned error: %v", err)
	}

	if err := guard.Check("trade", snapshotStub{canonical: "BTC-USDT", native: "BTCUSDT"}); err != nil {
		t.Fatalf("repeat check returned error: %v", err)
	}

	driftErr := guard.Check("trade", snapshotStub{canonical: "", native: "btcusd"})
	if driftErr == nil {
		t.Fatal("expected drift error")
	}
	typed, ok := driftErr.(*SymbolDriftError)
	if !ok {
		t.Fatalf("expected *SymbolDriftError, got %T", driftErr)
	}
	if !typed.HaltFeed {
		t.Fatal("expected halt flag set")
	}
	if captured.Exchange != "binance" || captured.Feed != "trade" {
		t.Fatalf("unexpected alert captured: %+v", captured)
	}
	if captured.ObservedSymbol != "BTCUSD" {
		t.Fatalf("expected sanitized observed symbol, got %s", captured.ObservedSymbol)
	}

	// Subsequent calls should return the stored drift error.
	if err := guard.Check("trade", snapshotStub{canonical: "btc-usdt", native: "btc-usdt"}); err != driftErr {
		t.Fatal("expected halted error to be reused")
	}
}

func TestSymbolGuardSanitizers(t *testing.T) {
	if got := sanitizeCanonical("  eth-usdt "); got != "ETH-USDT" {
		t.Fatalf("sanitizeCanonical returned %q", got)
	}
	if sanitizeCanonical(" ") != "" {
		t.Fatal("expected empty result for blank canonical")
	}
	if got := sanitizeNative("  ethusdt  ", "fallback"); got != "ETHUSDT" {
		t.Fatalf("sanitizeNative returned %q", got)
	}
	if got := sanitizeNative("  ", "fallback"); got != "fallback" {
		t.Fatalf("sanitizeNative fallback returned %q", got)
	}
}
