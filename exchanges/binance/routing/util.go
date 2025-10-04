package routing

import (
	"math/big"

	"github.com/coachpo/meltica/exchanges/infra/numeric"
)

func parseDecimalToRat(s string) (*big.Rat, bool) {
	return numeric.Parse(s)
}
