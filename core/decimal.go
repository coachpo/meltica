package core

import (
	"math/big"
	"strings"
)

// FormatDecimal scales a rational to a fixed decimal string with the given scale.
// It rounds towards zero.
func FormatDecimal(r *big.Rat, scale int) string {
	if r == nil {
		return ""
	}
	// r * 10^scale, then take floor towards zero and insert decimal point
	pow10 := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(scale)), nil)
	scaled := new(big.Rat).Mul(r, new(big.Rat).SetInt(pow10))
	// towards zero
	i := new(big.Int)
	if scaled.Sign() >= 0 {
		i.Div(scaled.Num(), scaled.Denom())
	} else {
		// for negative numbers, truncate towards zero: -( |num| / den )
		tmp := new(big.Int).Div(new(big.Int).Abs(scaled.Num()), scaled.Denom())
		i.Neg(tmp)
	}
	s := i.String()
	if scale == 0 {
		return s
	}
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	if len(s) <= scale {
		s = strings.Repeat("0", scale-len(s)+1) + s
	}
	dot := len(s) - scale
	out := s[:dot] + "." + s[dot:]
	if neg {
		out = "-" + out
	}
	return out
}
