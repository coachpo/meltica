package routing

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"testing"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	sharedrouting "github.com/coachpo/meltica/exchanges/shared/routing"
)

func TestOrderTranslatorPrepareCreate(t *testing.T) {
	translator := newTestTranslator(t, OrderTranslatorConfig{SupportReduceOnly: true})
	req := core.OrderRequest{
		Symbol:      "BTC-USDT",
		Side:        core.SideBuy,
		Type:        core.TypeLimit,
		Quantity:    big.NewRat(12345, 100000), // 0.12345
		Price:       big.NewRat(270005, 10),    // 27000.5
		TimeInForce: core.GTC,
		ClientID:    "cli-1",
		ReduceOnly:  true,
	}
	spec, err := translator.PrepareCreate(context.Background(), req)
	if err != nil {
		t.Fatalf("PrepareCreate error: %v", err)
	}
	msg := spec.Message
	if msg.Method != "POST" || msg.Path != "/api/v3/order" || msg.API != "spot" {
		t.Fatalf("unexpected rest message: %+v", msg)
	}
	if msg.Query["quantity"] != "0.123" {
		t.Fatalf("expected scaled quantity 0.123, got %s", msg.Query["quantity"])
	}
	if msg.Query["price"] != "27000.50" {
		t.Fatalf("expected scaled price 27000.50, got %s", msg.Query["price"])
	}
	if msg.Query["reduceOnly"] != "true" {
		t.Fatalf("expected reduceOnly flag, got %v", msg.Query["reduceOnly"])
	}
	if spec.Decode == nil {
		t.Fatal("expected decode function")
	}
	if spec.OnError == nil {
		t.Fatal("expected onError handler")
	}
	annotated := spec.OnError(errs.New("binance", errs.CodeInvalid))
	var annotatedErr *errs.E
	if !errors.As(annotated, &annotatedErr) {
		t.Fatalf("expected errs.E, got %T", annotated)
	}
	if annotatedErr.VenueMetadata["symbol"] != "BTC-USDT" {
		t.Fatalf("expected symbol metadata, got %+v", annotatedErr.VenueMetadata)
	}
	if annotatedErr.VenueMetadata["action"] != string(sharedrouting.ActionPlace) {
		t.Fatalf("expected action metadata, got %+v", annotatedErr.VenueMetadata)
	}
	env := &orderEnvelope{OrderID: json.Number("12345"), Status: "NEW", ExecutedQty: "0.123", AvgPrice: "27000.50"}
	order, err := spec.Decode(env)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if order.ID != "12345" || order.Status != core.OrderNew {
		t.Fatalf("unexpected order decoded: %+v", order)
	}
	if order.FilledQty == nil || order.FilledQty.RatString() != "123/1000" {
		t.Fatalf("unexpected filled qty: %v", order.FilledQty)
	}
}

func TestOrderTranslatorPrepareCancelRequiresIdentifier(t *testing.T) {
	translator := newTestTranslator(t, OrderTranslatorConfig{})
	_, err := translator.PrepareCancel(context.Background(), sharedrouting.OrderCancelRequest{Symbol: "BTC-USDT"})
	if err == nil {
		t.Fatal("expected error when identifiers missing")
	}
}

// newTestTranslator constructs a translator with deterministic dependencies for unit testing.
func newTestTranslator(t *testing.T, cfg OrderTranslatorConfig) sharedrouting.OrderTranslator {
	t.Helper()
	base := OrderTranslatorConfig{
		Market:    core.MarketSpot,
		API:       "spot",
		OrderPath: "/api/v3/order",
		ResolveNative: func(context.Context, string) (string, error) {
			return "BTCUSDT", nil
		},
		LookupInstrument: func(context.Context, string) (core.Instrument, error) {
			return core.Instrument{PriceScale: 2, QtyScale: 3}, nil
		},
		TimeInForce: func(core.TimeInForce) string { return "GTC" },
	}
	merged := base
	merged.SupportReduceOnly = cfg.SupportReduceOnly
	translator, err := NewOrderTranslator(merged)
	if err != nil {
		t.Fatalf("NewOrderTranslator error: %v", err)
	}
	return translator
}
