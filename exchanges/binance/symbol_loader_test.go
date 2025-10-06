package binance

import (
	"context"
	"reflect"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type stubRESTDispatcher struct {
	t       *testing.T
	fill    func(out any)
	lastMsg routingrest.RESTMessage
}

func (s *stubRESTDispatcher) Dispatch(_ context.Context, msg routingrest.RESTMessage, out any) error {
	s.lastMsg = msg
	if s.fill != nil {
		s.fill(out)
	}
	return nil
}

type symbolDef struct {
	Symbol              string
	Base                string
	Quote               string
	Status              string
	BasePrecision       int
	QuoteAssetPrecision int
	QuotePrecision      int
	Filters             []filterDef
}

type filterDef struct {
	FilterType string
	TickSize   string
	StepSize   string
}

func makeSymbolResponse(t *testing.T, defs []symbolDef) func(out any) {
	return func(out any) {
		t.Helper()
		rv := reflect.ValueOf(out)
		if rv.Kind() != reflect.Pointer {
			t.Fatalf("expected pointer output, got %T", out)
		}
		elem := rv.Elem()
		symbolsField := elem.FieldByName("Symbols")
		slice := reflect.MakeSlice(symbolsField.Type(), len(defs), len(defs))
		defType := symbolsField.Type().Elem()
		for i, d := range defs {
			entry := reflect.New(defType).Elem()
			entry.FieldByName("Symbol").SetString(d.Symbol)
			entry.FieldByName("Base").SetString(d.Base)
			entry.FieldByName("Quote").SetString(d.Quote)
			entry.FieldByName("Status").SetString(d.Status)
			entry.FieldByName("BasePrecision").SetInt(int64(d.BasePrecision))
			entry.FieldByName("QuoteAssetPrecision").SetInt(int64(d.QuoteAssetPrecision))
			entry.FieldByName("QuotePrecision").SetInt(int64(d.QuotePrecision))

			filtersField := entry.FieldByName("Filters")
			filterSlice := reflect.MakeSlice(filtersField.Type(), len(d.Filters), len(d.Filters))
			filterType := filtersField.Type().Elem()
			for j, f := range d.Filters {
				filter := reflect.New(filterType).Elem()
				filter.FieldByName("FilterType").SetString(f.FilterType)
				filter.FieldByName("TickSize").SetString(f.TickSize)
				filter.FieldByName("StepSize").SetString(f.StepSize)
				filterSlice.Index(j).Set(filter)
			}
			filtersField.Set(filterSlice)
			slice.Index(i).Set(entry)
		}
		symbolsField.Set(slice)
	}
}

func TestFetchMarketSymbolsUsesTradingStatusAndPrecision(t *testing.T) {
	t.Parallel()

	stub := &stubRESTDispatcher{t: t}
	stub.fill = makeSymbolResponse(t, []symbolDef{
		{
			Symbol:              "BTCUSDT",
			Base:                "BTC",
			Quote:               "USDT",
			Status:              "TRADING",
			BasePrecision:       8,
			QuoteAssetPrecision: 5,
			QuotePrecision:      4,
			Filters: []filterDef{
				{FilterType: "PRICE_FILTER", TickSize: "0.001"},
				{FilterType: "LOT_SIZE", StepSize: "0.00010000"},
			},
		},
		{
			Symbol:              "ETHUSDT",
			Base:                "ETH",
			Quote:               "USDT",
			Status:              "HALT",
			BasePrecision:       8,
			QuoteAssetPrecision: 4,
			QuotePrecision:      4,
		},
	})

	svc := newSymbolService(stub)

	spec := marketSpec{api: rest.SpotAPI, path: "/api/v3/exchangeInfo", market: core.MarketSpot}

	payload, err := svc.fetchMarketSymbols(context.Background(), spec, stub)
	if err != nil {
		t.Fatalf("fetchMarketSymbols returned error: %v", err)
	}

	if got := stub.lastMsg.Query["symbolStatus"]; got != "TRADING" {
		t.Fatalf("expected symbolStatus query param to be TRADING, got %q", got)
	}

	if len(payload) != 1 {
		t.Fatalf("expected 1 trading symbol, got %d", len(payload))
	}

	inst := payload[0].instrument
	if inst.Symbol != "BTC-USDT" {
		t.Fatalf("unexpected instrument symbol %q", inst.Symbol)
	}
	if inst.PriceScale != 5 {
		t.Fatalf("expected price scale 5, got %d", inst.PriceScale)
	}
	if inst.QtyScale != 8 {
		t.Fatalf("expected qty scale 8, got %d", inst.QtyScale)
	}
	if payload[0].native != "BTCUSDT" {
		t.Fatalf("expected native symbol BTCUSDT, got %q", payload[0].native)
	}
}
