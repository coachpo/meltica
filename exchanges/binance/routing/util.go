package routing

import (
	"math/big"

	"github.com/coachpo/meltica/core"
)

func parseDecimalToRat(s string) (*big.Rat, bool) {
	return core.ParseDecimalToRat(s)
}
