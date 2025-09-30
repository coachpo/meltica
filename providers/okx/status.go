package okx

import "github.com/yourorg/meltica/core"

func mapOKXStatus(s string) core.OrderStatus {
	switch s {
	case "live", "effective":
		return core.OrderNew
	case "partially-filled":
		return core.OrderPartFilled
	case "filled":
		return core.OrderFilled
	case "canceled":
		return core.OrderCanceled
	case "canceled-amend", "cancel_rejected", "order_failed":
		return core.OrderRejected
	default:
		return core.OrderNew
	}
}
