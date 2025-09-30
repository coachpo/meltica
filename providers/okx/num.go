package okx

import (
	"math/big"
	"strings"
)

func parseDecimalToRat(s string) (*big.Rat, bool) {
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

// Exported for tests
func ParseDecimalToRat(s string) (*big.Rat, bool) { return parseDecimalToRat(s) }

func scaleFromStep(step string) int {
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
