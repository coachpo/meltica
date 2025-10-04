package provider

import "github.com/coachpo/meltica/core"

// MapOrderStatus converts Binance order status codes to core order statuses.
func MapOrderStatus(s string) core.OrderStatus {
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

// MapTimeInForce converts canonical time-in-force values to Binance codes.
func MapTimeInForce(t core.TimeInForce) string {
	switch t {
	case core.GTC:
		return "GTC"
	case core.ICO:
		return "IOC"
	case core.FOK:
		return "FOK"
	default:
		return ""
	}
}
