package routing

import (
	"testing"

	"github.com/coachpo/meltica/core"
)

func TestOrderSideMappings(t *testing.T) {
	cases := []struct {
		enum   core.OrderSide
		string string
	}{
		{core.SideBuy, "BUY"},
		{core.SideSell, "SELL"},
	}

	for _, tc := range cases {
		str, err := OrderSideString(tc.enum)
		if err != nil {
			t.Fatalf("OrderSideString(%v) returned error: %v", tc.enum, err)
		}
		if str != tc.string {
			t.Fatalf("expected %s got %s", tc.string, str)
		}

		recovered, err := OrderSideFromString(tc.string)
		if err != nil {
			t.Fatalf("OrderSideFromString(%s) returned error: %v", tc.string, err)
		}
		if recovered != tc.enum {
			t.Fatalf("expected %v got %v", tc.enum, recovered)
		}
	}

	if _, err := OrderSideString(core.OrderSide("invalid")); err == nil {
		t.Fatal("expected error for invalid order side")
	}
	if _, err := OrderSideFromString("unknown"); err == nil {
		t.Fatal("expected error for unknown order side string")
	}
}

func TestOrderTypeMappings(t *testing.T) {
	cases := []struct {
		enum   core.OrderType
		string string
	}{
		{core.TypeMarket, "MARKET"},
		{core.TypeLimit, "LIMIT"},
	}

	for _, tc := range cases {
		str, err := OrderTypeString(tc.enum)
		if err != nil {
			t.Fatalf("OrderTypeString(%v) returned error: %v", tc.enum, err)
		}
		if str != tc.string {
			t.Fatalf("expected %s got %s", tc.string, str)
		}

		recovered, err := OrderTypeFromString(tc.string)
		if err != nil {
			t.Fatalf("OrderTypeFromString(%s) returned error: %v", tc.string, err)
		}
		if recovered != tc.enum {
			t.Fatalf("expected %v got %v", tc.enum, recovered)
		}
	}

	if _, err := OrderTypeString(core.OrderType("invalid")); err == nil {
		t.Fatal("expected error for invalid order type")
	}
	if _, err := OrderTypeFromString("unknown"); err == nil {
		t.Fatal("expected error for unknown order type string")
	}
}

func TestTimeInForceMappings(t *testing.T) {
	cases := []struct {
		enum   core.TimeInForce
		string string
	}{
		{core.GTC, "GTC"},
		{core.IOC, "IOC"},
		{core.FOK, "FOK"},
	}

	for _, tc := range cases {
		str, err := TimeInForceString(tc.enum)
		if err != nil {
			t.Fatalf("TimeInForceString(%v) returned error: %v", tc.enum, err)
		}
		if str != tc.string {
			t.Fatalf("expected %s got %s", tc.string, str)
		}

		recovered, err := TimeInForceFromString(tc.string)
		if err != nil {
			t.Fatalf("TimeInForceFromString(%s) returned error: %v", tc.string, err)
		}
		if recovered != tc.enum {
			t.Fatalf("expected %v got %v", tc.enum, recovered)
		}
	}

	if _, err := TimeInForceString(core.TimeInForce("invalid")); err == nil {
		t.Fatal("expected error for invalid time-in-force")
	}
	if _, err := TimeInForceFromString("unknown"); err == nil {
		t.Fatal("expected error for unknown time-in-force string")
	}
}

func TestOrderStatusMappings(t *testing.T) {
	cases := []struct {
		enum   core.OrderStatus
		string string
	}{
		{core.OrderNew, "NEW"},
		{core.OrderPartFilled, "PARTIALLY_FILLED"},
		{core.OrderFilled, "FILLED"},
		{core.OrderCanceled, "CANCELED"},
		{core.OrderRejected, "REJECTED"},
	}

	for _, tc := range cases {
		str, err := OrderStatusString(tc.enum)
		if err != nil {
			t.Fatalf("OrderStatusString(%v) returned error: %v", tc.enum, err)
		}
		if str != tc.string {
			t.Fatalf("expected %s got %s", tc.string, str)
		}

		recovered, err := OrderStatusFromString(tc.string)
		if err != nil {
			t.Fatalf("OrderStatusFromString(%s) returned error: %v", tc.string, err)
		}
		if recovered != tc.enum {
			t.Fatalf("expected %v got %v", tc.enum, recovered)
		}
	}

	if _, err := OrderStatusString(core.OrderStatus("invalid")); err == nil {
		t.Fatal("expected error for invalid order status")
	}
	if _, err := OrderStatusFromString("unknown"); err == nil {
		t.Fatal("expected error for unknown order status string")
	}
}

func TestMarketMappings(t *testing.T) {
	cases := []struct {
		enum   core.Market
		string string
	}{
		{core.MarketSpot, "SPOT"},
		{core.MarketLinearFutures, "LINEAR_FUTURES"},
		{core.MarketInverseFutures, "INVERSE_FUTURES"},
	}

	for _, tc := range cases {
		str, err := MarketString(tc.enum)
		if err != nil {
			t.Fatalf("MarketString(%v) returned error: %v", tc.enum, err)
		}
		if str != tc.string {
			t.Fatalf("expected %s got %s", tc.string, str)
		}

		recovered, err := MarketFromString(tc.string)
		if err != nil {
			t.Fatalf("MarketFromString(%s) returned error: %v", tc.string, err)
		}
		if recovered != tc.enum {
			t.Fatalf("expected %v got %v", tc.enum, recovered)
		}
	}

	if _, err := MarketString(core.Market("invalid")); err == nil {
		t.Fatal("expected error for invalid market")
	}
	if _, err := MarketFromString("unknown"); err == nil {
		t.Fatal("expected error for unknown market string")
	}
}
