package kraken

import "strings"

// nativeSymbolForWS returns the Kraken-native symbol used for websocket subscriptions.
func (w ws) nativeSymbolForWS(canon string) string {
	if w.p == nil {
		return canon
	}
	if native := w.p.canonToKrakenWS[canon]; native != "" {
		return native
	}
	if native := w.p.canonToKraken[canon]; native != "" {
		return native
	}
	return canon
}

// canonicalSymbol resolves an exchange symbol to the canonical representation.
func (w ws) canonicalSymbol(exch string, requested []string) string {
	if exch == "" {
		return canonicalFromRequested(exch, requested)
	}
	upper := strings.ToUpper(strings.TrimSpace(exch))
	if w.p != nil {
		if w.p.nativeToCanon != nil {
			if canon := w.p.nativeToCanon[upper]; canon != "" {
				return canon
			}
			if canon := w.p.nativeToCanon[strings.ReplaceAll(upper, "/", "")]; canon != "" {
				return canon
			}
		}
		for c, native := range w.p.canonToKraken {
			if strings.EqualFold(native, upper) {
				return c
			}
		}
		for c, native := range w.p.canonToKrakenWS {
			if strings.EqualFold(native, upper) {
				return c
			}
		}
	}
	return canonicalFromRequested(upper, requested)
}

// canonicalFromRequested attempts to match an exchange symbol to a requested topic symbol.
func canonicalFromRequested(exch string, requested []string) string {
	candidates := []string{exch}
	if strings.Contains(exch, "/") {
		candidates = append(candidates, strings.ReplaceAll(exch, "/", "-"))
	} else if strings.Contains(exch, "-") {
		candidates = append(candidates, strings.ReplaceAll(exch, "-", "/"))
	}
	for _, t := range requested {
		_, sym := parseTopic(t)
		if sym == "" {
			continue
		}
		for _, cand := range candidates {
			if cand != "" && strings.EqualFold(sym, cand) {
				return sym
			}
		}
		if strings.Contains(sym, "-") {
			alt := strings.ReplaceAll(sym, "-", "/")
			for _, cand := range candidates {
				if cand != "" && strings.EqualFold(alt, cand) {
					return sym
				}
			}
		}
	}
	if strings.Contains(exch, "/") {
		return strings.ReplaceAll(exch, "/", "-")
	}
	return exch
}
