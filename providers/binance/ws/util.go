package ws

import (
	"math/big"

	"github.com/coachpo/meltica/core"
)

func parseDecimalToRat(s string) (*big.Rat, bool) {
	return core.ParseDecimalToRat(s)
}

func mapBStatus(s string) core.OrderStatus {
	switch s {
	case "NEW":
		return core.OrderNew
	case "PARTIALLY_FILLED":
		return core.OrderPartFilled
	case "FILLED":
		return core.OrderFilled
	case "CANCELED", "PENDING_CANCEL", "CANCELLED":
		return core.OrderCanceled
	case "REJECTED", "EXPIRED":
		return core.OrderRejected
	default:
		return core.OrderNew
	}
}
