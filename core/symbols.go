package core

import (
	"fmt"
	"strings"
	"sync"
)

// ExchangeName identifies a registered exchange implementation.
type ExchangeName string

// SymbolTranslator exposes canonical/native conversion helpers for an exchange.
type SymbolTranslator interface {
	Native(canonical string) (string, error)
	Canonical(native string) (string, error)
}

var (
	translatorMu sync.RWMutex
	translators  = make(map[ExchangeName]SymbolTranslator)
)

// RegisterSymbolTranslator associates the translator with the given exchange name.
func RegisterSymbolTranslator(name ExchangeName, translator SymbolTranslator) {
	if name == "" || translator == nil {
		return
	}
	translatorMu.Lock()
	defer translatorMu.Unlock()
	translators[name] = translator
}

// SymbolTranslatorFor returns the registered translator for the exchange.
func SymbolTranslatorFor(name ExchangeName) (SymbolTranslator, error) {
	translatorMu.RLock()
	translator, ok := translators[name]
	translatorMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("core: symbol translator for exchange %s not registered", name)
	}
	return translator, nil
}

// NativeSymbol resolves a canonical symbol to the exchange native representation.
func NativeSymbol(name ExchangeName, canonical string) (string, error) {
	translator, err := SymbolTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Native(canonical)
}

// CanonicalFromNative converts an exchange native symbol into canonical form.
func CanonicalFromNative(name ExchangeName, native string) (string, error) {
	translator, err := SymbolTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Canonical(native)
}

// CanonicalSymbol returns a canonical symbol form Base-Quote (e.g., BTC-USDT)
func CanonicalSymbol(base, quote string) string {
	return strings.ToUpper(base) + "-" + strings.ToUpper(quote)
}
