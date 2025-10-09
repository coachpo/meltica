package core

import (
	"fmt"
	"strings"
	"sync"
)

type SymbolSnapshot interface {
	Canonical() string
	Native() string
}

type SymbolDriftAlert struct {
	Exchange       ExchangeName
	Feed           string
	Canonical      string
	ExpectedSymbol string
	ObservedSymbol string
	Remediation    string
}

type SymbolDriftError struct {
	Alert    SymbolDriftAlert
	HaltFeed bool
}

func (e *SymbolDriftError) Error() string {
	if e == nil {
		return "symbol drift"
	}
	return fmt.Sprintf("symbol drift on %s feed %s: expected %s, observed %s", e.Alert.Exchange, e.Alert.Feed, e.Alert.ExpectedSymbol, e.Alert.ObservedSymbol)
}

type SymbolGuard struct {
	mu       sync.Mutex
	exchange ExchangeName
	onDrift  func(SymbolDriftAlert)
	state    map[string]symbolGuardState
	halted   map[string]*SymbolDriftError
}

type symbolGuardState struct {
	canonical string
	native    string
}

func NewSymbolGuard(exchange ExchangeName, onDrift func(SymbolDriftAlert)) *SymbolGuard {
	return &SymbolGuard{
		exchange: exchange,
		onDrift:  onDrift,
		state:    make(map[string]symbolGuardState),
		halted:   make(map[string]*SymbolDriftError),
	}
}

func (g *SymbolGuard) Check(feed string, snapshot SymbolSnapshot) error {
	if snapshot == nil || feed == "" {
		return nil
	}

	canonical := sanitizeCanonical(snapshot.Canonical())
	native := sanitizeNative(snapshot.Native(), canonical)

	g.mu.Lock()
	if err, ok := g.halted[feed]; ok {
		g.mu.Unlock()
		return err
	}

	state, exists := g.state[feed]
	if !exists {
		g.state[feed] = symbolGuardState{canonical: canonical, native: native}
		g.mu.Unlock()
		return nil
	}

	if canonical == state.canonical && native == state.native {
		g.mu.Unlock()
		return nil
	}

	if canonical == "" {
		canonical = state.canonical
	}

	alert := SymbolDriftAlert{
		Exchange:       g.exchange,
		Feed:           feed,
		Canonical:      state.canonical,
		ExpectedSymbol: state.native,
		ObservedSymbol: native,
		Remediation:    fmt.Sprintf("Symbol format changed from %s to %s; check venue API changelog and update translators.", state.native, native),
	}

	err := &SymbolDriftError{Alert: alert, HaltFeed: true}
	g.halted[feed] = err
	callback := g.onDrift
	g.mu.Unlock()

	if callback != nil {
		callback(alert)
	}

	return err
}

func sanitizeCanonical(value string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed == "" {
		return ""
	}
	return trimmed
}

func sanitizeNative(value string, fallback string) string {
	trimmed := strings.ToUpper(strings.TrimSpace(value))
	if trimmed != "" {
		return trimmed
	}
	return fallback
}
