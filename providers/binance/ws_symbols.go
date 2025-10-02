package binance

import "strings"

func (w ws) canonicalSymbol(bin string) string {
	if bin == "" {
		return bin
	}
	bin = strings.ToUpper(bin)
	if w.p != nil && w.p.binToCanon != nil {
		if canon := w.p.binToCanon[bin]; canon != "" {
			return canon
		}
	}
	return bin
}
