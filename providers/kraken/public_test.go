package kraken

import (
	"testing"

	"github.com/coachpo/meltica/core"
)

func Test_TestOnlyParseTrades(t *testing.T) {
	msg := core.Message{}
	payload := []any{
		map[string]any{
			"symbol":    "BTC/USD",
			"price":     "50000",
			"qty":       "0.1",
			"timestamp": "2021-03-25T12:30:00Z",
		},
	}
	if err := TestOnlyParseTrades(&msg, payload, "BTC-USD"); err != nil {
		t.Fatalf("parse trades: %v", err)
	}
	if msg.Event != "trade" {
		t.Fatalf("expected trade event")
	}
}

func Test_TestOnlyParseTicker(t *testing.T) {
	msg := core.Message{}
	payload := map[string]any{
		"symbol": "BTC/USD",
		"bid":    "50000",
		"ask":    "50010",
	}
	if err := TestOnlyParseTicker(&msg, payload, "BTC-USD"); err != nil {
		t.Fatalf("parse ticker: %v", err)
	}
	ev, ok := msg.Parsed.(*core.TickerEvent)
	if !ok {
		t.Fatalf("expected ticker event, got %T", msg.Parsed)
	}
	if ev.Symbol != "BTC-USD" {
		t.Fatalf("unexpected symbol %s", ev.Symbol)
	}
}

func Test_TestOnlyParseBook(t *testing.T) {
	msg := core.Message{}
	payload := map[string]any{
		"symbol": "BTC/USD",
		"bids": []any{
			map[string]any{"price": "50000", "qty": "0.4"},
		},
		"asks": []any{
			map[string]any{"price": "50010", "qty": "0.3"},
		},
	}
	if err := TestOnlyParseBook(&msg, payload, "BTC-USD"); err != nil {
		t.Fatalf("parse book: %v", err)
	}
	ev, ok := msg.Parsed.(*core.DepthEvent)
	if !ok {
		t.Fatalf("expected depth event, got %T", msg.Parsed)
	}
	if len(ev.Bids) != 1 || ev.Bids[0].Price.RatString() != "50000" {
		t.Fatalf("unexpected bids %+v", ev.Bids)
	}
	if len(ev.Asks) != 1 || ev.Asks[0].Price.RatString() != "50010" {
		t.Fatalf("unexpected asks %+v", ev.Asks)
	}
}

func Test_ParseLevel3(t *testing.T) {
	msg := core.Message{}
	payload := []any{
		map[string]any{
			"symbol": "BTC/USD",
			"orders": []any{
				map[string]any{"price": "50000", "qty": "0.25", "side": "buy"},
				map[string]any{"price": "50020", "qty": "0.10", "side": "sell"},
			},
		},
	}
	if err := (ws{}).parseLevel3(&msg, payload, "BTC-USD"); err != nil {
		t.Fatalf("parse level3: %v", err)
	}
	ev, ok := msg.Parsed.(*core.DepthEvent)
	if !ok {
		t.Fatalf("expected depth event, got %T", msg.Parsed)
	}
	if len(ev.Bids) != 1 || ev.Bids[0].Price.RatString() != "50000" {
		t.Fatalf("unexpected level3 bids %+v", ev.Bids)
	}
	if len(ev.Asks) != 1 || ev.Asks[0].Price.RatString() != "50020" {
		t.Fatalf("unexpected level3 asks %+v", ev.Asks)
	}
}

func Test_TestOnlyNormalizeBalances(t *testing.T) {
	raw := map[string]string{"XXBT": "0.5", "ZUSD": "1000"}
	balances := TestOnlyNormalizeBalances(raw)
	if len(balances) != 2 {
		t.Fatalf("expected 2 balances, got %d", len(balances))
	}
	if balances[0].Asset == "" || balances[0].Available == nil {
		t.Fatal("expected normalized balance entry")
	}
}

func Test_TestOnlyParseOwnTrades(t *testing.T) {
	msg := core.Message{}
	p := TestOnlyNewProvider()
	p.nativeToCanon = map[string]string{"XXBTZUSD": "BTC-USD"}
	payload := map[string]any{
		"tx1": map[string]any{
			"pair":   "XXBTZUSD",
			"vol":    "0.1",
			"price":  "50000",
			"status": "closed",
			"time":   "1616667000.0",
		},
	}
	if err := TestOnlyParseOwnTrades(&msg, payload, p); err != nil {
		t.Fatalf("parse own trades: %v", err)
	}
	if msg.Event != "order" {
		t.Fatalf("expected order event")
	}
	if msg.Topic != core.OrderTopic("BTC-USD") {
		t.Fatalf("unexpected topic %s", msg.Topic)
	}
}

func Test_TestOnlyParseOpenOrders(t *testing.T) {
	msg := core.Message{}
	p := TestOnlyNewProvider()
	p.nativeToCanon = map[string]string{"XXBTZUSD": "BTC-USD"}
	payload := map[string]any{
		"id1": map[string]any{
			"descr":    map[string]any{"pair": "XXBTZUSD"},
			"status":   "open",
			"vol_exec": "0.0",
			"price":    map[string]any{"last": "50000"},
		},
	}
	if err := TestOnlyParseOpenOrders(&msg, payload, p); err != nil {
		t.Fatalf("parse open orders: %v", err)
	}
	if msg.Event != "order" {
		t.Fatalf("expected order event")
	}
	if msg.Topic != core.OrderTopic("BTC-USD") {
		t.Fatalf("unexpected topic %s", msg.Topic)
	}
}
