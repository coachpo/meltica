package routing

import (
	"fmt"
	"strings"

	"github.com/coachpo/meltica/core"
)

// OrderSideString returns the uppercase representation of the canonical order side.
func OrderSideString(side core.OrderSide) (string, error) {
	switch side {
	case core.SideBuy:
		return "BUY", nil
	case core.SideSell:
		return "SELL", nil
	default:
		return "", fmt.Errorf("routing: unsupported order side %q", side)
	}
}

// OrderSideFromString converts a string into the canonical order side.
func OrderSideFromString(value string) (core.OrderSide, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "BUY":
		return core.SideBuy, nil
	case "SELL":
		return core.SideSell, nil
	default:
		return "", fmt.Errorf("routing: unsupported order side %q", value)
	}
}

// OrderTypeString returns the uppercase representation of the canonical order type.
func OrderTypeString(orderType core.OrderType) (string, error) {
	switch orderType {
	case core.TypeMarket:
		return "MARKET", nil
	case core.TypeLimit:
		return "LIMIT", nil
	default:
		return "", fmt.Errorf("routing: unsupported order type %q", orderType)
	}
}

// OrderTypeFromString converts a string into the canonical order type.
func OrderTypeFromString(value string) (core.OrderType, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "MARKET":
		return core.TypeMarket, nil
	case "LIMIT":
		return core.TypeLimit, nil
	default:
		return "", fmt.Errorf("routing: unsupported order type %q", value)
	}
}

// TimeInForceString returns the uppercase representation of the canonical time-in-force policy.
func TimeInForceString(tif core.TimeInForce) (string, error) {
	switch tif {
	case core.GTC:
		return "GTC", nil
	case core.IOC:
		return "IOC", nil
	case core.FOK:
		return "FOK", nil
	default:
		return "", fmt.Errorf("routing: unsupported time-in-force %q", tif)
	}
}

// TimeInForceFromString converts a string into the canonical time-in-force policy.
func TimeInForceFromString(value string) (core.TimeInForce, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "GTC":
		return core.GTC, nil
	case "IOC":
		return core.IOC, nil
	case "FOK":
		return core.FOK, nil
	default:
		return "", fmt.Errorf("routing: unsupported time-in-force %q", value)
	}
}

// OrderStatusString returns the uppercase representation of the canonical order status.
func OrderStatusString(status core.OrderStatus) (string, error) {
	switch status {
	case core.OrderNew:
		return "NEW", nil
	case core.OrderPartFilled:
		return "PARTIALLY_FILLED", nil
	case core.OrderFilled:
		return "FILLED", nil
	case core.OrderCanceled:
		return "CANCELED", nil
	case core.OrderRejected:
		return "REJECTED", nil
	default:
		return "", fmt.Errorf("routing: unsupported order status %q", status)
	}
}

// OrderStatusFromString converts a string into the canonical order status.
func OrderStatusFromString(value string) (core.OrderStatus, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "NEW":
		return core.OrderNew, nil
	case "PARTIALLY_FILLED", "PART_FILLED", "PARTIAL_FILLED":
		return core.OrderPartFilled, nil
	case "FILLED":
		return core.OrderFilled, nil
	case "CANCELED", "CANCELLED", "PENDING_CANCEL":
		return core.OrderCanceled, nil
	case "REJECTED", "EXPIRED":
		return core.OrderRejected, nil
	default:
		return "", fmt.Errorf("routing: unsupported order status %q", value)
	}
}

// MarketString returns the uppercase representation of the canonical market.
func MarketString(market core.Market) (string, error) {
	switch market {
	case core.MarketSpot:
		return "SPOT", nil
	case core.MarketLinearFutures:
		return "LINEAR_FUTURES", nil
	case core.MarketInverseFutures:
		return "INVERSE_FUTURES", nil
	default:
		return "", fmt.Errorf("routing: unsupported market %q", market)
	}
}

// MarketFromString converts a string into the canonical market.
func MarketFromString(value string) (core.Market, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "SPOT":
		return core.MarketSpot, nil
	case "LINEAR_FUTURES", "LINEAR-FUTURES", "LINEAR":
		return core.MarketLinearFutures, nil
	case "INVERSE_FUTURES", "INVERSE-FUTURES", "INVERSE":
		return core.MarketInverseFutures, nil
	default:
		return "", fmt.Errorf("routing: unsupported market %q", value)
	}
}
