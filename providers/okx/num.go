package okx

import (
	"math/big"

	"github.com/coachpo/meltica/core"
)

func parseDecimalToRat(s string) (*big.Rat, bool) { return core.ParseDecimalToRat(s) }

// Exported for tests
func ParseDecimalToRat(s string) (*big.Rat, bool) { return core.ParseDecimalToRat(s) }

func scaleFromStep(step string) int { return core.ScaleFromStep(step) }
