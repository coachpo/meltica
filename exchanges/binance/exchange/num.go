package exchange

import (
	"math/big"

	"github.com/coachpo/meltica/exchanges/infra/numeric"
)

func parseDecimalToRat(s string) (*big.Rat, bool) { return numeric.Parse(s) }

// ParseDecimalToRat is an exported helper used in tests and mapping goldens.
func ParseDecimalToRat(s string) (*big.Rat, bool) { return numeric.Parse(s) }

// scaleFromStep returns number of decimal places implied by a decimal step such as "0.01000000".
func scaleFromStep(step string) int { return numeric.ScaleFromStep(step) }
