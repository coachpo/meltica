package kraken

import "github.com/coachpo/meltica/core"

// mapStatus normalizes Kraken order status strings.
func mapStatus(state string) core.OrderStatus {
	switch state {
	case "pending", "open", "partial":
		return core.OrderNew
	case "filled", "closed":
		return core.OrderFilled
	case "canceled", "cancelled":
		return core.OrderCanceled
	case "rejected", "expired":
		return core.OrderRejected
	default:
		return core.OrderNew
	}
}
