package main

import (
	"math/big"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/internal/numeric"
)

func TestParseSymbolList(t *testing.T) {
	input := "btc-usdt, ETH-USDT, btc-usdt , ,sol-usdt"
	got := parseSymbolList(input)
	want := []string{"BTC-USDT", "ETH-USDT", "SOL-USDT"}
	if len(got) != len(want) {
		t.Fatalf("expected %d symbols, got %v", len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected symbol %s at index %d, got %s", want[i], i, got[i])
		}
	}
	if parseSymbolList("   ") != nil {
		t.Fatalf("expected nil for whitespace input")
	}
}

func TestBuildTopics(t *testing.T) {
	symbols := []string{"BTC-USDT"}
	topics := buildTopics(symbols)
	expected := []string{
		core.MustCanonicalTopic(core.TopicTrade, "BTC-USDT"),
		core.MustCanonicalTopic(core.TopicTicker, "BTC-USDT"),
	}
	if len(topics) != len(expected) {
		t.Fatalf("expected %d topics, got %v", len(expected), topics)
	}
	for i, topic := range topics {
		if topic != expected[i] {
			t.Fatalf("expected topic %s at index %d, got %s", expected[i], i, topic)
		}
	}
}

func TestPrecisionFormatterUsesInstrumentScales(t *testing.T) {
	instruments := map[string]core.Instrument{
		"BTC-USDT": {PriceScale: 4, QtyScale: 7},
	}
	formatter := newPrecisionFormatter(instruments)

	priceValue := big.NewRat(123456, 10) // 12345.6
	qtyValue := big.NewRat(123, 1000000) // 0.000123

	if got, want := formatter.price("BTC-USDT", priceValue), numeric.Format(priceValue, 4); got != want {
		t.Fatalf("expected price %s got %s", want, got)
	}
	if got, want := formatter.quantity("BTC-USDT", qtyValue), numeric.Format(qtyValue, 7); got != want {
		t.Fatalf("expected qty %s got %s", want, got)
	}
}

func TestPrecisionFormatterFallbacks(t *testing.T) {
	formatter := newPrecisionFormatter(nil)
	value := big.NewRat(1, 3)
	if got, want := formatter.price("UNKNOWN", value), numeric.Format(value, fallbackPriceScale); got != want {
		t.Fatalf("expected fallback price %s got %s", want, got)
	}
	if got, want := formatter.quantity("UNKNOWN", value), numeric.Format(value, fallbackQuantityScale); got != want {
		t.Fatalf("expected fallback qty %s got %s", want, got)
	}
	if got := formatter.price("UNKNOWN", nil); got != "0" {
		t.Fatalf("expected nil price to render 0, got %s", got)
	}
}

func TestDescribeTopLevel(t *testing.T) {
	formatter := newPrecisionFormatter(map[string]core.Instrument{
		"BTC-USDT": {PriceScale: 2, QtyScale: 3},
	})
	if got := describeTopLevel(formatter, "BTC-USDT", nil); got != "-" {
		t.Fatalf("expected '-', got %s", got)
	}
	level := []core.BookDepthLevel{
		{
			Price: big.NewRat(20000, 1),
			Qty:   big.NewRat(1, 100),
		},
	}
	if got := describeTopLevel(formatter, "BTC-USDT", level); got == "-" {
		t.Fatalf("expected formatted level, got %s", got)
	}
}

func TestFormatDecimalHandlesNil(t *testing.T) {
	if got := formatDecimal(nil, 2); got != "0" {
		t.Fatalf("expected 0 for nil rat, got %s", got)
	}
}
