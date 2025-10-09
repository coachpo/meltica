package market_data

import (
	"math/big"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
)

var (
	benchmarkEncodedEvent []byte
	benchmarkSymbol       string
)

func BenchmarkEncodeTradeEvent(b *testing.B) {
	evt := &Event{
		Type:         EventTypeTrade,
		Venue:        core.ExchangeName("binance"),
		Symbol:       "BTC-USDT",
		NativeSymbol: "BTCUSDT",
		Timestamp:    time.Unix(171717, 0).UTC(),
		Sequence:     42,
		Payload: &TradePayload{
			Price:        big.NewRat(50000, 1),
			Quantity:     big.NewRat(1, 10),
			Side:         core.SideBuy,
			IsTaker:      true,
			VenueTradeID: "abc-123",
		},
	}

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		data, err := EncodeEvent(evt)
		if err != nil {
			b.Fatalf("encode event: %v", err)
		}
		benchmarkEncodedEvent = data
	}
}

func BenchmarkHotPathEventCanonical(b *testing.B) {
	evt := &Event{Symbol: "ETH-USDT"}
	b.ReportAllocs()
	if allocs := testing.AllocsPerRun(1, func() {
		benchmarkSymbol = evt.Canonical()
	}); allocs != 0 {
		b.Fatalf("event canonical accessor allocated %.0f objects", allocs)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkSymbol = evt.Canonical()
	}
}
