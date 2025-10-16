package schema

import (
	"testing"
	"time"
)

func TestOrderRequestReset(t *testing.T) {
	price := "50000.00"
	order := &OrderRequest{
		ClientOrderID: "order-123",
		ConsumerID:    "consumer-1",
		Provider:      "binance",
		Symbol:        "BTC-USD",
		Side:          TradeSideBuy,
		OrderType:     OrderTypeLimit,
		Price:         &price,
		Quantity:      "1.5",
		Timestamp:     time.Now(),
	}
	
	order.Reset()
	
	if order.ClientOrderID != "" {
		t.Errorf("ClientOrderID not reset, got %q", order.ClientOrderID)
	}
	if order.ConsumerID != "" {
		t.Errorf("ConsumerID not reset, got %q", order.ConsumerID)
	}
	if order.Provider != "" {
		t.Errorf("Provider not reset, got %q", order.Provider)
	}
	if order.Symbol != "" {
		t.Errorf("Symbol not reset, got %q", order.Symbol)
	}
	if order.Price != nil {
		t.Errorf("Price not reset, got %v", order.Price)
	}
	if order.Quantity != "" {
		t.Errorf("Quantity not reset, got %q", order.Quantity)
	}
}

func TestOrderRequestSetReturned(t *testing.T) {
	order := &OrderRequest{}
	
	if order.IsReturned() {
		t.Error("new order should not be returned")
	}
	
	order.SetReturned(true)
	if !order.IsReturned() {
		t.Error("order should be marked as returned")
	}
	
	order.SetReturned(false)
	if order.IsReturned() {
		t.Error("order should not be marked as returned")
	}
}

func TestOrderRequestNilHandling(t *testing.T) {
	var order *OrderRequest
	
	// Should not panic
	order.Reset()
	order.SetReturned(true)
	
	if order.IsReturned() {
		t.Error("nil order should return false for IsReturned")
	}
}

func TestExecReportReset(t *testing.T) {
	report := &ExecReport{
		ClientOrderID:   "order-123",
		ExchangeOrderID: "ex-order-456",
		Provider:        "binance",
		Symbol:          "BTC-USD",
		Status:          ExecReportStateACK,
		FilledQty:       "1.0",
		RemainingQty:    "0.5",
		AvgPrice:        "50000.00",
		TransactTime:    time.Now().UnixNano(),
		TraceID:         "trace-123",
		DecisionID:      "decision-456",
	}
	
	report.Reset()
	
	if report.ClientOrderID != "" {
		t.Errorf("ClientOrderID not reset, got %q", report.ClientOrderID)
	}
	if report.ExchangeOrderID != "" {
		t.Errorf("ExchangeOrderID not reset, got %q", report.ExchangeOrderID)
	}
	if report.Provider != "" {
		t.Errorf("Provider not reset, got %q", report.Provider)
	}
	if report.FilledQty != "" {
		t.Errorf("FilledQty not reset, got %q", report.FilledQty)
	}
	if report.TransactTime != 0 {
		t.Errorf("TransactTime not reset, got %d", report.TransactTime)
	}
}

func TestExecReportSetReturned(t *testing.T) {
	report := &ExecReport{}
	
	if report.IsReturned() {
		t.Error("new report should not be returned")
	}
	
	report.SetReturned(true)
	if !report.IsReturned() {
		t.Error("report should be marked as returned")
	}
	
	report.SetReturned(false)
	if report.IsReturned() {
		t.Error("report should not be marked as returned")
	}
}

func TestExecReportNilHandling(t *testing.T) {
	var report *ExecReport
	
	// Should not panic
	report.Reset()
	report.SetReturned(true)
	
	if report.IsReturned() {
		t.Error("nil report should return false for IsReturned")
	}
}
