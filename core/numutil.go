package core

import (
	"math/big"
	"strings"
)

// ParseDecimalToRat attempts to parse a decimal string into a rational number.
// The input is trimmed for whitespace; if parsing fails it returns (nil, false).
func ParseDecimalToRat(s string) (*big.Rat, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}
	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok {
		return nil, false
	}
	return r, true
}

// ScaleFromStep returns the number of fractional decimal places implied by a
// decimal "step" string, trimming trailing zeros to determine effective scale.
func ScaleFromStep(step string) int {
	step = strings.TrimSpace(step)
	if step == "" {
		return 0
	}
	idx := strings.IndexByte(step, '.')
	if idx < 0 {
		return 0
	}
	frac := strings.TrimRight(step[idx+1:], "0")
	return len(frac)
}
