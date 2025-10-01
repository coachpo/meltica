package binance

import (
	"math/big"

	"github.com/coachpo/meltica/core"
)

func parseDecimalToRat(s string) (*big.Rat, bool) { return core.ParseDecimalToRat(s) }

// ParseDecimalToRat is an exported helper used in tests and mapping goldens.
func ParseDecimalToRat(s string) (*big.Rat, bool) { return core.ParseDecimalToRat(s) }

// scaleFromStep returns number of decimal places implied by a decimal step such as "0.01000000".
func scaleFromStep(step string) int { return core.ScaleFromStep(step) }
