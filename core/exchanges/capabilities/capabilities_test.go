package capabilities

import "testing"

func TestOfAndHas(t *testing.T) {
	set := Of(
		CapabilitySpotPublicREST,
		CapabilityWebsocketPrivate,
		CapabilitySpotTradingREST,
		CapabilityMarketTrades,
	)

	cases := []struct {
		cap  Capability
		want bool
	}{
		{CapabilitySpotPublicREST, true},
		{CapabilityLinearPublicREST, false},
		{CapabilityWebsocketPrivate, true},
		{CapabilitySpotTradingREST, true},
		{CapabilityTradingSpotCancel, false},
		{CapabilityMarketTrades, true},
	}

	for _, tc := range cases {
		if got := set.Has(tc.cap); got != tc.want {
			t.Fatalf("Has(%v) = %v, want %v", tc.cap, got, tc.want)
		}
	}
}

func TestWithWithout(t *testing.T) {
	set := Of(CapabilityLinearTradingREST)
	set = set.With(CapabilityTradingLinearAmend)

	if !set.Has(CapabilityTradingLinearAmend) {
		t.Fatalf("expected capability to be present after With")
	}

	trimmed := set.Without(CapabilityLinearTradingREST)
	if trimmed.Has(CapabilityLinearTradingREST) {
		t.Fatalf("expected capability to be absent after Without")
	}
	if !trimmed.Has(CapabilityTradingLinearAmend) {
		t.Fatalf("expected remaining capability to stay set")
	}
}

func TestAllAnyMissing(t *testing.T) {
	set := Of(
		CapabilitySpotTradingREST,
		CapabilityTradingSpotCancel,
		CapabilityAccountBalances,
	)

	if !set.All(CapabilitySpotTradingREST, CapabilityTradingSpotCancel) {
		t.Fatalf("All should report true when all capabilities are present")
	}

	if set.All(CapabilitySpotTradingREST, CapabilityTradingSpotAmend) {
		t.Fatalf("All should report false when a capability is missing")
	}

	if !set.Any(CapabilityTradingSpotAmend, CapabilityTradingSpotCancel) {
		t.Fatalf("Any should report true when at least one capability is present")
	}

	if set.Any(CapabilityLinearTradingREST, CapabilityTradingLinearAmend) {
		t.Fatalf("Any should report false when no capabilities are present")
	}

	missing := set.Missing(CapabilityTradingSpotAmend, CapabilityAccountBalances, CapabilityAccountPositions)
	expected := map[Capability]bool{
		CapabilityTradingSpotAmend: true,
		CapabilityAccountPositions: true,
	}

	if len(missing) != len(expected) {
		t.Fatalf("expected %d missing capabilities, got %d", len(expected), len(missing))
	}

	for _, cap := range missing {
		if !expected[cap] {
			t.Fatalf("unexpected missing capability %v", cap)
		}
		delete(expected, cap)
	}

	if len(expected) != 0 {
		t.Fatalf("expected missing capabilities were not returned: %v", expected)
	}
}

func TestSliceRoundTrip(t *testing.T) {
	set := Of(
		CapabilityMarketTrades,
		CapabilityMarketTicker,
		CapabilityMarketOrderBook,
		CapabilityMarketCandles,
	)

	observed := set.Slice()
	want := map[Capability]bool{
		CapabilityMarketTrades:    true,
		CapabilityMarketTicker:    true,
		CapabilityMarketOrderBook: true,
		CapabilityMarketCandles:   true,
	}

	if len(observed) != len(want) {
		t.Fatalf("expected %d capabilities in slice, got %d", len(want), len(observed))
	}

	for _, cap := range observed {
		if !want[cap] {
			t.Fatalf("unexpected capability in slice: %v", cap)
		}
		delete(want, cap)
	}

	if len(want) != 0 {
		t.Fatalf("slice missing capabilities: %v", want)
	}
}
