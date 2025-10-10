package core

import "testing"

func TestCapabilitiesBuildsSet(t *testing.T) {
	set := Capabilities(CapabilitySpotPublicREST, CapabilityMarketTrades)
	if !set.Has(CapabilitySpotPublicREST) || !set.Has(CapabilityMarketTrades) {
		t.Fatalf("expected capability set to contain provided capabilities")
	}
	if set.Has(CapabilityLinearTradingREST) {
		t.Fatalf("unexpected capability present in set")
	}
}
