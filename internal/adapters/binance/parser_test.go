package binance

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

func TestParser_ParseDepthUpdate(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@depth",
		"data": {
			"e": "depthUpdate",
			"E": 1234567890,
			"s": "BTCUSDT",
			"u": 157,
			"b": [["50000.00", "1.5"], ["49999.00", "2.0"]],
			"a": [["50001.00", "0.5"], ["50002.00", "1.0"]],
			"checksum": "12345"
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeBookSnapshot {
		t.Errorf("expected BookSnapshot type, got %v", evt.Type)
	}
	if evt.Symbol != "BTC-USDT" {
		t.Errorf("expected symbol BTC-USDT, got %s", evt.Symbol)
	}
	if evt.SeqProvider != 157 {
		t.Errorf("expected seq 157, got %d", evt.SeqProvider)
	}

	payload, ok := evt.Payload.(schema.BookSnapshotPayload)
	if !ok {
		t.Fatalf("expected BookSnapshotPayload, got %T", evt.Payload)
	}
	if len(payload.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(payload.Bids))
	}
	if len(payload.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(payload.Asks))
	}
}

func TestParser_ParseAggTrade(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@aggTrade",
		"data": {
			"e": "aggTrade",
			"E": 1234567890,
			"s": "BTCUSDT",
			"t": 12345,
			"p": "50000.00",
			"q": "0.5",
			"m": false
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeTrade {
		t.Errorf("expected Trade type, got %v", evt.Type)
	}
	if evt.Symbol != "BTC-USDT" {
		t.Errorf("expected symbol BTC-USDT, got %s", evt.Symbol)
	}

	payload, ok := evt.Payload.(schema.TradePayload)
	if !ok {
		t.Fatalf("expected TradePayload, got %T", evt.Payload)
	}
	if payload.Side != schema.TradeSideBuy {
		t.Errorf("expected buy side, got %v", payload.Side)
	}
	if payload.Price != "50000.00" {
		t.Errorf("expected price 50000.00, got %s", payload.Price)
	}
	if payload.Quantity != "0.5" {
		t.Errorf("expected quantity 0.5, got %s", payload.Quantity)
	}
}

func TestParser_ParseAggTrade_SellerMaker(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@aggTrade",
		"data": {
			"e": "aggTrade",
			"E": 1234567890,
			"s": "BTCUSDT",
			"t": 12345,
			"p": "50000.00",
			"q": "0.5",
			"m": true
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	payload, ok := events[0].Payload.(schema.TradePayload)
	if !ok {
		t.Fatalf("expected TradePayload, got %T", events[0].Payload)
	}
	if payload.Side != schema.TradeSideSell {
		t.Errorf("expected sell side for buyer maker, got %v", payload.Side)
	}
}

func TestParser_ParseTicker(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@ticker",
		"data": {
			"e": "24hrTicker",
			"E": 1234567890,
			"s": "BTCUSDT",
			"c": "50000.00",
			"b": "49999.00",
			"a": "50001.00",
			"v": "1000.5"
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeTicker {
		t.Errorf("expected Ticker type, got %v", evt.Type)
	}

	payload, ok := evt.Payload.(schema.TickerPayload)
	if !ok {
		t.Fatalf("expected TickerPayload, got %T", evt.Payload)
	}
	if payload.LastPrice != "50000.00" {
		t.Errorf("expected last price 50000.00, got %s", payload.LastPrice)
	}
	if payload.BidPrice != "49999.00" {
		t.Errorf("expected bid price 49999.00, got %s", payload.BidPrice)
	}
	if payload.AskPrice != "50001.00" {
		t.Errorf("expected ask price 50001.00, got %s", payload.AskPrice)
	}
	if payload.Volume24h != "1000.5" {
		t.Errorf("expected volume 1000.5, got %s", payload.Volume24h)
	}
}

func TestParser_ParseKline(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@kline_1m",
		"data": {
			"e": "kline",
			"E": 1234567890,
			"s": "BTCUSDT",
			"k": {
				"t": 1234560000,
				"T": 1234567999,
				"s": "BTCUSDT",
				"i": "1m",
				"o": "50000.00",
				"c": "50100.00",
				"h": "50200.00",
				"l": "49900.00",
				"v": "100.5",
				"x": true
			}
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeKlineSummary {
		t.Errorf("expected KlineSummary type, got %v", evt.Type)
	}

	payload, ok := evt.Payload.(schema.KlineSummaryPayload)
	if !ok {
		t.Fatalf("expected KlineSummaryPayload, got %T", evt.Payload)
	}
	if payload.OpenPrice != "50000.00" {
		t.Errorf("expected open price 50000.00, got %s", payload.OpenPrice)
	}
	if payload.ClosePrice != "50100.00" {
		t.Errorf("expected close price 50100.00, got %s", payload.ClosePrice)
	}
	if payload.HighPrice != "50200.00" {
		t.Errorf("expected high price 50200.00, got %s", payload.HighPrice)
	}
	if payload.LowPrice != "49900.00" {
		t.Errorf("expected low price 49900.00, got %s", payload.LowPrice)
	}
}

func TestParser_ParseExecutionReport(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "user_stream",
		"data": {
			"e": "executionReport",
			"E": 1234567890,
			"s": "BTCUSDT",
			"c": "client123",
			"S": "BUY",
			"o": "LIMIT",
			"X": "NEW",
			"i": 12345,
			"l": "0",
			"z": "0",
			"L": "0",
			"n": "0",
			"N": "BNB",
			"T": 1234567890,
			"t": 0,
			"q": "1.5",
			"p": "50000.00"
		}
	}`)

	events, err := parser.Parse(context.Background(), frame, time.Now())
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeExecReport {
		t.Errorf("expected ExecReport type, got %v", evt.Type)
	}

	payload, ok := evt.Payload.(schema.ExecReportPayload)
	if !ok {
		t.Fatalf("expected ExecReportPayload, got %T", evt.Payload)
	}
	if payload.ClientOrderID != "client123" {
		t.Errorf("expected client order ID client123, got %s", payload.ClientOrderID)
	}
	if payload.State != schema.ExecReportStateACK {
		t.Errorf("expected ACK state, got %v", payload.State)
	}
	if payload.Side != schema.TradeSideBuy {
		t.Errorf("expected buy side, got %v", payload.Side)
	}
	if payload.OrderType != schema.OrderTypeLimit {
		t.Errorf("expected limit order type, got %v", payload.OrderType)
	}
}

func TestParser_ParseExecutionReport_States(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	tests := []struct {
		status   string
		expected schema.ExecReportState
	}{
		{"NEW", schema.ExecReportStateACK},
		{"PARTIALLY_FILLED", schema.ExecReportStatePARTIAL},
		{"FILLED", schema.ExecReportStateFILLED},
		{"CANCELED", schema.ExecReportStateCANCELLED},
		{"CANCELLED", schema.ExecReportStateCANCELLED},
		{"REJECTED", schema.ExecReportStateREJECTED},
		{"EXPIRED", schema.ExecReportStateEXPIRED},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			frame := []byte(`{
				"stream": "user_stream",
				"data": {
					"e": "executionReport",
					"E": 1234567890,
					"s": "BTCUSDT",
					"c": "client123",
					"S": "BUY",
					"o": "LIMIT",
					"X": "` + tt.status + `",
					"i": 12345,
					"T": 1234567890,
					"q": "1.5",
					"p": "50000.00"
				}
			}`)

			events, err := parser.Parse(context.Background(), frame, time.Now())
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			payload := events[0].Payload.(schema.ExecReportPayload)
			if payload.State != tt.expected {
				t.Errorf("expected state %v, got %v", tt.expected, payload.State)
			}
		})
	}
}

func TestParser_ParseSnapshot_Orderbook(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	body := []byte(`{
		"s": "BTCUSDT",
		"lastUpdateId": 12345,
		"bids": [["50000.00", "1.5"], ["49999.00", "2.0"]],
		"asks": [["50001.00", "0.5"], ["50002.00", "1.0"]],
		"checksum": "12345",
		"E": 1234567890
	}`)

	events, err := parser.ParseSnapshot(context.Background(), "orderbook", body, time.Now())
	if err != nil {
		t.Fatalf("parse snapshot failed: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	evt := events[0]
	if evt.Type != schema.EventTypeBookSnapshot {
		t.Errorf("expected BookSnapshot type, got %v", evt.Type)
	}
	if evt.Symbol != "BTC-USDT" {
		t.Errorf("expected symbol BTC-USDT, got %s", evt.Symbol)
	}

	payload, ok := evt.Payload.(schema.BookSnapshotPayload)
	if !ok {
		t.Fatalf("expected BookSnapshotPayload, got %T", evt.Payload)
	}
	if len(payload.Bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(payload.Bids))
	}
	if len(payload.Asks) != 2 {
		t.Errorf("expected 2 asks, got %d", len(payload.Asks))
	}
}

func TestParser_ParseSnapshot_UnsupportedType(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	_, err := parser.ParseSnapshot(context.Background(), "unknown", []byte("{}"), time.Now())
	if err == nil {
		t.Error("expected error for unsupported snapshot type")
	}
	if !strings.Contains(err.Error(), "unsupported rest parser") {
		t.Errorf("expected 'unsupported rest parser' error, got: %v", err)
	}
}

func TestParser_InvalidJSON(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	_, err := parser.Parse(context.Background(), []byte("invalid json"), time.Now())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParser_MissingSymbol(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@depth",
		"data": {
			"e": "depthUpdate",
			"E": 1234567890,
			"u": 157,
			"b": [["50000.00", "1.5"]],
			"a": [["50001.00", "0.5"]]
		}
	}`)

	_, err := parser.Parse(context.Background(), frame, time.Now())
	if err == nil {
		t.Error("expected error for missing symbol")
	}
	if !strings.Contains(err.Error(), "missing symbol") {
		t.Errorf("expected 'missing symbol' error, got: %v", err)
	}
}

func TestCanonicalInstrument(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"BTCUSDT", "BTC-USDT"},
		{"ETHUSDT", "ETH-USDT"},
		{"BNBBUSD", "BNB-BUSD"},
		{"BTCUSDC", "BTC-USDC"},
		{"ETHBTC", "ETH-BTC"},
		{"btcusdt", "BTC-USDT"},
		{"  BTCUSDT  ", "BTC-USDT"},
		{"", ""},
		{"BTC", "BTC"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := canonicalInstrument(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestInferStreamType(t *testing.T) {
	tests := []struct {
		stream   string
		expected string
	}{
		{"btcusdt@depth", "depthupdate"},
		{"btcusdt@depth@100ms", "depthupdate"},
		{"btcusdt@aggTrade", "aggtrade"},
		{"btcusdt@ticker", "24hrticker"},
		{"btcusdt@kline_1m", ""},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.stream, func(t *testing.T) {
			result := inferStreamType(tt.stream)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParser_NegativeTimestamp(t *testing.T) {
	pools := pool.NewPoolManager()
	pools.RegisterPool("Event", 10, func() any { return new(schema.Event) })
	parser := NewParserWithPool("binance", pools)

	frame := []byte(`{
		"stream": "btcusdt@ticker",
		"data": {
			"e": "24hrTicker",
			"E": -1,
			"s": "BTCUSDT",
			"c": "50000.00",
			"b": "49999.00",
			"a": "50001.00",
			"v": "1000.5"
		}
	}`)

	_, err := parser.Parse(context.Background(), frame, time.Now())
	if err == nil {
		t.Error("expected error for negative timestamp")
	}
	if !strings.Contains(err.Error(), "negative") {
		t.Errorf("expected 'negative' error, got: %v", err)
	}
}
