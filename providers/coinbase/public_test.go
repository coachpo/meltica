package coinbase

import (
	"encoding/json"
	"testing"

	"github.com/yourorg/meltica/core"
)

func Test_TestOnlyParseTicker(t *testing.T) {
	payload := []byte(`{"type":"ticker","product_id":"BTC-USD","best_bid":"20000","best_ask":"20010"}`)
	ev, err := TestOnlyParseTicker(payload)
	if err != nil {
		t.Fatalf("parse ticker: %v", err)
	}
	if ev.Symbol != "BTC-USD" {
		t.Fatalf("unexpected symbol %s", ev.Symbol)
	}
}

func Test_TestOnlyParseMatch(t *testing.T) {
	payload := []byte(`{"type":"match","product_id":"ETH-USD","price":"1500","size":"0.5","time":"2023-01-01T00:00:00Z"}`)
	ev, err := TestOnlyParseMatch(payload)
	if err != nil {
		t.Fatalf("parse trade: %v", err)
	}
	if ev.Symbol != "ETH-USD" {
		t.Fatalf("unexpected symbol %s", ev.Symbol)
	}
}

func Test_TestOnlyNormalizeBalances(t *testing.T) {
	raw := []accountBalance{{Currency: "USD", Available: "10"}, {Currency: "BTC", Available: "0.1"}}
	balances := TestOnlyNormalizeBalances(raw)
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}
	foundUSD := false
	for _, b := range balances {
		if b.Asset == "USD" {
			foundUSD = true
			break
		}
	}
	if !foundUSD {
		t.Fatalf("expected USD balance")
	}
}

func Test_FixtureBalances(t *testing.T) {
	payload := []byte(`{"accounts":[{"currency":"USD","balance":"100"}]}`)
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	msg := core.Message{}
	wsInstance := ws{p: TestOnlyNewProvider()}
	if err := wsInstance.parseBalance(&msg, env); err != nil {
		t.Fatalf("parse balance: %v", err)
	}
	if msg.Parsed == nil {
		t.Fatalf("expected balance event")
	}
}

func Test_FixtureTrades(t *testing.T) {
	provider := TestOnlyNewProvider()
	provider.nativeToCanon["BTC-USD"] = "BTC-USD"
	payload := []byte(`{"type":"match","product_id":"BTC-USD","price":"25000","size":"0.01","time":"2023-01-02T00:00:00Z"}`)
	var env map[string]any
	if err := json.Unmarshal(payload, &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	msg := core.Message{}
	wsInstance := ws{p: provider}
	if err := wsInstance.parseMatch(&msg, env); err != nil {
		t.Fatalf("parse match: %v", err)
	}
	ev, ok := msg.Parsed.(*core.TradeEvent)
	if !ok || ev.Symbol != "BTC-USD" {
		t.Fatalf("unexpected trade event: %+v", msg.Parsed)
	}
}

func Test_ParseSnapshotDepth(t *testing.T) {
	provider := TestOnlyNewProvider()
	provider.nativeToCanon["BTC-USD"] = "BTC-USD"
	wsInstance := ws{p: provider}
	msg := core.Message{}
	env := map[string]any{
		"product_id": "BTC-USD",
		"time":       "2023-01-02T00:00:00Z",
		"bids": []any{
			[]any{"25000", "0.5"},
		},
		"asks": []any{
			[]any{"25010", "0.4"},
		},
	}
	if err := wsInstance.parseSnapshot(&msg, env); err != nil {
		t.Fatalf("parse snapshot: %v", err)
	}
	if msg.Topic != core.DepthTopic("BTC-USD") {
		t.Fatalf("unexpected topic %s", msg.Topic)
	}
	ev, ok := msg.Parsed.(*core.DepthEvent)
	if !ok {
		t.Fatalf("expected depth event, got %T", msg.Parsed)
	}
	if len(ev.Bids) != 1 || ev.Bids[0].Price.RatString() != "25000" {
		t.Fatalf("unexpected bids %+v", ev.Bids)
	}
	if len(ev.Asks) != 1 || ev.Asks[0].Price.RatString() != "25010" {
		t.Fatalf("unexpected asks %+v", ev.Asks)
	}
}

func Test_ParseL2DepthUpdate(t *testing.T) {
	provider := TestOnlyNewProvider()
	provider.nativeToCanon["BTC-USD"] = "BTC-USD"
	wsInstance := ws{p: provider}
	msg := core.Message{}
	env := map[string]any{
		"product_id": "BTC-USD",
		"time":       "2023-01-02T00:00:05Z",
		"changes": []any{
			[]any{"buy", "25005", "0.3"},
			[]any{"sell", "25015", "0.2"},
		},
	}
	if err := wsInstance.parseL2(&msg, env); err != nil {
		t.Fatalf("parse l2: %v", err)
	}
	if msg.Topic != core.DepthTopic("BTC-USD") {
		t.Fatalf("unexpected topic %s", msg.Topic)
	}
	ev, ok := msg.Parsed.(*core.DepthEvent)
	if !ok {
		t.Fatalf("expected depth event, got %T", msg.Parsed)
	}
	if len(ev.Bids) != 1 || ev.Bids[0].Price.RatString() != "25005" {
		t.Fatalf("unexpected bids %+v", ev.Bids)
	}
	if len(ev.Asks) != 1 || ev.Asks[0].Price.RatString() != "25015" {
		t.Fatalf("unexpected asks %+v", ev.Asks)
	}
}

func Test_ParseOrderEvent(t *testing.T) {
	provider := TestOnlyNewProvider()
	provider.nativeToCanon["BTC-USD"] = "BTC-USD"
	wsInstance := ws{p: provider}
	msg := core.Message{}
	env := map[string]any{
		"type":        "done",
		"product_id":  "BTC-USD",
		"order_id":    "order1",
		"client_oid":  "client1",
		"filled_size": "0.1",
		"price":       "25000",
		"time":        "2023-01-02T00:00:10Z",
		"reason":      "filled",
	}
	if err := wsInstance.parseOrderEvent(&msg, env); err != nil {
		t.Fatalf("parse order event: %v", err)
	}
	ev, ok := msg.Parsed.(*core.OrderEvent)
	if !ok {
		t.Fatalf("expected order event, got %T", msg.Parsed)
	}
	if ev.Status != core.OrderFilled {
		t.Fatalf("unexpected status %s", ev.Status)
	}
}
