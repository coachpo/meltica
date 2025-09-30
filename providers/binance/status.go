package binance

import "github.com/yourorg/meltica/core"

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

func mapBTIF(t core.TimeInForce) string {
	switch t {
	case core.TIFGTC:
		return "GTC"
	case core.TIFIC:
		return "IOC"
	case core.TIFFOK:
		return "FOK"
	default:
		return ""
	}
}
