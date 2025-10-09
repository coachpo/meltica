package pipeline

import (
	"testing"

	corestreams "github.com/coachpo/meltica/core/streams"
)

func TestClientEventFromPipelineAllocations(t *testing.T) {
	trade := corestreams.TradeEvent{Symbol: "BTC-USDT"}
	evt := Event{
		Transport: TransportPublicWS,
		Symbol:    "BTC-USDT",
		Payload:   TradePayload{Trade: &trade},
		Metadata: map[string]any{
			metadataKeySourceFeed:     "trade",
			metadataKeySourceSymbol:   "BTC-USDT",
			metadataKeySourceSequence: uint64(99),
			metadataKeySourceExchange: "binance",
		},
	}

	if allocs := testing.AllocsPerRun(1, func() {
		_ = clientEventFromPipeline(evt)
	}); allocs != 0 {
		t.Fatalf("clientEventFromPipeline allocated %.0f objects", allocs)
	}
}
