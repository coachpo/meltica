package core

import (
	"math/big"
	"strconv"
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

	mantissa := step
	exponent := 0
	if idx := strings.IndexAny(mantissa, "eE"); idx >= 0 {
		mantissaPart := strings.TrimSpace(mantissa[:idx])
		exponentPart := strings.TrimSpace(mantissa[idx+1:])
		if mantissaPart == "" {
			return 0
		}
		if exp, err := strconv.Atoi(exponentPart); err == nil {
			exponent = exp
			mantissa = mantissaPart
		} else {
			// If the exponent is malformed we fall back to using the raw string.
			mantissa = mantissaPart
		}
	}

	if mantissa == "" {
		return 0
	}
	if mantissa[0] == '+' || mantissa[0] == '-' {
		mantissa = mantissa[1:]
	}
	if mantissa == "" {
		return 0
	}

	decimals := 0
	if idx := strings.IndexByte(mantissa, '.'); idx >= 0 {
		frac := strings.TrimRight(mantissa[idx+1:], "0")
		decimals = len(frac)
	}

	scale := decimals - exponent
	if scale < 0 {
		scale = 0
	}
	return scale
}
