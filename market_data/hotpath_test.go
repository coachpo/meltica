package market_data

import "testing"

func TestEventCanonicalAllocation(t *testing.T) {
	evt := &Event{Symbol: "ETH-USDT"}
	if allocs := testing.AllocsPerRun(1, func() {
		_ = evt.Canonical()
	}); allocs != 0 {
		t.Fatalf("Event.Canonical allocated %.0f objects", allocs)
	}
}
