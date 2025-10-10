package internal

import (
	"testing"

	"github.com/coachpo/meltica/core"
)

func TestMapOrderStatus(t *testing.T) {
	cases := map[string]core.OrderStatus{
		"NEW":              core.OrderNew,
		"PARTIALLY_FILLED": core.OrderPartFilled,
		"FILLED":           core.OrderFilled,
		"CANCELED":         core.OrderCanceled,
		"PENDING_CANCEL":   core.OrderCanceled,
		"CANCELLED":        core.OrderCanceled,
		"REJECTED":         core.OrderRejected,
		"EXPIRED":          core.OrderRejected,
		"UNKNOWN":          core.OrderNew,
		"":                 core.OrderNew,
	}
	for input, expected := range cases {
		if got := MapOrderStatus(input); got != expected {
			t.Fatalf("status %q: expected %v, got %v", input, expected, got)
		}
	}
}

func TestMapTimeInForce(t *testing.T) {
	cases := map[core.TimeInForce]string{
		core.GTC: "GTC",
		core.IOC: "IOC",
		core.FOK: "FOK",
		"":       "",
	}
	for input, expected := range cases {
		if got := MapTimeInForce(input); got != expected {
			t.Fatalf("time in force %q: expected %q, got %q", input, expected, got)
		}
	}
}
