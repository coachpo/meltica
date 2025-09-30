package core

import "strings"

// CanonicalSymbol returns a canonical symbol form Base-Quote (e.g., BTC-USDT)
func CanonicalSymbol(base, quote string) string { return base + "-" + quote }

// CanonicalToBinance converts canonical to Binance's symbol (BTCUSDT)
func CanonicalToBinance(canon string) string { return strings.ReplaceAll(canon, "-", "") }

// CanonicalToOKX leaves BTC-USDT unchanged
func CanonicalToOKX(canon string) string { return canon }
