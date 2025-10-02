package coinbase

import "strings"

func (w ws) nativeProduct(symbol string) string {
	if symbol == "" {
		return symbol
	}
	if w.p == nil {
		return strings.ToUpper(symbol)
	}
	if native := w.p.native(strings.ToUpper(symbol)); native != "" {
		return native
	}
	return strings.ToUpper(symbol)
}

func (w ws) canonicalSymbol(native string) string {
	native = strings.ToUpper(native)
	if native == "" {
		return native
	}
	if w.p == nil {
		return native
	}
	w.p.mu.RLock()
	defer w.p.mu.RUnlock()
	if canon := w.p.nativeToCanon[native]; canon != "" {
		return canon
	}
	return native
}
