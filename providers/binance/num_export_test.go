package binance

import "math/big"

func ParseDecimalToRatForTest(s string) (*big.Rat, bool) { return parseDecimalToRat(s) }
