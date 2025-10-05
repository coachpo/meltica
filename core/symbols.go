package core

import (
	"fmt"
	"strings"
	"sync"
)

// ExchangeName identifies a registered exchange implementation.
type ExchangeName string

// SymbolMapper exposes canonical/native conversion helpers for an exchange.
type SymbolMapper interface {
	NativeSymbol(canonical string) (string, error)
	CanonicalSymbol(native string) (string, error)
}

var (
	mapperMu sync.RWMutex
	mappers  = make(map[ExchangeName]SymbolMapper)
)

// RegisterSymbolMapper associates the mapper with the given exchange name.
func RegisterSymbolMapper(name ExchangeName, mapper SymbolMapper) {
	if name == "" || mapper == nil {
		return
	}
	mapperMu.Lock()
	defer mapperMu.Unlock()
	mappers[name] = mapper
}

// NativeSymbol resolves a canonical symbol to the exchange native representation.
func NativeSymbol(name ExchangeName, canonical string) (string, error) {
	mapperMu.RLock()
	mapper, ok := mappers[name]
	mapperMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("core: symbol mapper for exchange %s not registered", name)
	}
	return mapper.NativeSymbol(canonical)
}

// CanonicalFromNative converts an exchange native symbol into canonical form.
func CanonicalFromNative(name ExchangeName, native string) (string, error) {
	mapperMu.RLock()
	mapper, ok := mappers[name]
	mapperMu.RUnlock()
	if !ok {
		return "", fmt.Errorf("core: symbol mapper for exchange %s not registered", name)
	}
	return mapper.CanonicalSymbol(native)
}

// CanonicalSymbol returns a canonical symbol form Base-Quote (e.g., BTC-USDT)
func CanonicalSymbol(base, quote string) string {
	return strings.ToUpper(base) + "-" + strings.ToUpper(quote)
}
