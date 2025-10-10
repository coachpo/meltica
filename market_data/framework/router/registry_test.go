package router

import (
	"errors"
	"testing"

	"github.com/coachpo/meltica/errs"
)

func TestRoutingTableRegisterAndLookup(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	desc := &MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "e",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade",
	}
	proc := newMockProcessor("trade")

	if err := rt.Register(desc, proc); err != nil {
		t.Fatalf("register: %v", err)
	}

	reg := rt.Lookup("trade")
	if reg == nil {
		t.Fatalf("expected registration for trade")
	}
	if reg.Status != ProcessorStatusAvailable {
		t.Fatalf("unexpected status: %s", reg.Status)
	}

	unknown := rt.Lookup("unknown")
	if unknown != rt.defaultProc {
		t.Fatalf("expected default registration for unknown type")
	}

	// ensure descriptors are stored as copies
	desc.DetectionRules[0].ExpectedValue = "modified"
	rt.descMu.RLock()
	stored := rt.descriptors["trade"]
	rt.descMu.RUnlock()
	if stored.DetectionRules[0].ExpectedValue != "trade" {
		t.Fatalf("descriptor clone mutated; got %s", stored.DetectionRules[0].ExpectedValue)
	}
}

func TestRoutingTableRegisterUnavailable(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	desc := &MessageTypeDescriptor{
		ID: "account",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "account",
		}},
		ProcessorRef: "account",
	}
	proc := newMockProcessor("account")
	proc.initErr = errors.New("boom")

	if err := rt.Register(desc, proc); err != nil {
		t.Fatalf("register: %v", err)
	}

	reg := rt.Lookup("account")
	if reg.Status != ProcessorStatusUnavailable {
		t.Fatalf("expected unavailable status, got %s", reg.Status)
	}
	if reg.InitError == nil {
		t.Fatalf("expected initialization error")
	}
	var e *errs.E
	if !errors.As(reg.InitError, &e) {
		t.Fatalf("init error should be errs.E, got %T", reg.InitError)
	}
}

func TestRoutingTableRegisterMismatch(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	desc := &MessageTypeDescriptor{
		ID: "orderbook",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "depth",
		}},
		ProcessorRef: "orderbook",
	}
	proc := newMockProcessor("book")
	if err := rt.Register(desc, proc); err == nil {
		t.Fatalf("expected mismatch error")
	}
}

func TestRoutingTableDetectMatch(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	tradeDesc := &MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade",
	}
	accountDesc := &MessageTypeDescriptor{
		ID: "account",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "account",
		}},
		ProcessorRef: "account",
	}

	if err := rt.Register(tradeDesc, newMockProcessor("trade")); err != nil {
		t.Fatalf("register trade: %v", err)
	}
	if err := rt.Register(accountDesc, newMockProcessor("account")); err != nil {
		t.Fatalf("register account: %v", err)
	}

	tradePayload := loadTestPayload(t, "trade.json")
	msgType, err := rt.Detect(tradePayload)
	if err != nil {
		t.Fatalf("detect trade: %v", err)
	}
	if msgType != "trade" {
		t.Fatalf("expected trade, got %s", msgType)
	}

	metrics := rt.GetMetrics()
	if metrics.MessagesRouted["trade"] != 1 {
		t.Fatalf("expected routed metric increment, got %d", metrics.MessagesRouted["trade"])
	}
}

func TestRoutingTableDetectNoMatch(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	tradeDesc := &MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade",
	}

	if err := rt.Register(tradeDesc, newMockProcessor("trade")); err != nil {
		t.Fatalf("register trade: %v", err)
	}

	unknownPayload := loadTestPayload(t, "funding.json")
	msgType, err := rt.Detect(unknownPayload)
	if msgType != "" {
		t.Fatalf("expected empty message type, got %s", msgType)
	}
	if err == nil {
		t.Fatalf("expected detection error for no match")
	}
	if !isNoMatchError(err) {
		t.Fatalf("expected no match error, got %v", err)
	}
	metrics := rt.GetMetrics()
	if metrics.RoutingErrors == 0 {
		t.Fatalf("expected routing error metric to increment")
	}
}

func TestRoutingTableDetectInvalidJSON(t *testing.T) {
	rt := NewRoutingTable()
	if err := rt.SetDefault(newMockProcessor("default")); err != nil {
		t.Fatalf("set default: %v", err)
	}

	tradeDesc := &MessageTypeDescriptor{
		ID: "trade",
		DetectionRules: []DetectionRule{{
			Strategy:      DetectionStrategyFieldBased,
			FieldPath:     "type",
			ExpectedValue: "trade",
		}},
		ProcessorRef: "trade",
	}

	if err := rt.Register(tradeDesc, newMockProcessor("trade")); err != nil {
		t.Fatalf("register trade: %v", err)
	}

	msgType, err := rt.Detect([]byte("{invalid"))
	if msgType != "" {
		t.Fatalf("expected empty type on error")
	}
	if err == nil {
		t.Fatalf("expected parse error")
	}
	if isNoMatchError(err) {
		t.Fatalf("parse error should not be treated as no match")
	}
	metrics := rt.GetMetrics()
	if metrics.RoutingErrors == 0 {
		t.Fatalf("expected routing error metric increment")
	}
}
