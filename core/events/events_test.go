package events_test

import (
	"testing"
	"time"

	"github.com/coachpo/meltica/core/events"
)

func TestEventKindIsCritical(t *testing.T) {
	tests := []struct {
		kind     events.EventKind
		expected bool
	}{
		{events.KindMarketData, false},
		{events.KindExecReport, true},
		{events.KindControlAck, true},
		{events.KindControlResult, true},
		{events.EventKind(99), false},
	}

	for _, tc := range tests {
		if got := tc.kind.IsCritical(); got != tc.expected {
			t.Fatalf("IsCritical mismatch for kind %v: expected %v got %v", tc.kind, tc.expected, got)
		}
	}
}

func TestEventKindString(t *testing.T) {
	tests := map[events.EventKind]string{
		events.KindMarketData:    "market_data",
		events.KindExecReport:    "exec_report",
		events.KindControlAck:    "control_ack",
		events.KindControlResult: "control_result",
		events.EventKind(-1):     "unknown",
	}

	for kind, expected := range tests {
		if got := kind.String(); got != expected {
			t.Fatalf("String mismatch for kind %v: expected %q got %q", kind, expected, got)
		}
	}
}

func TestEventReset(t *testing.T) {
	ev := &events.Event{
		TraceID:        "trace",
		RoutingVersion: 42,
		Kind:           events.KindExecReport,
		Payload:        struct{}{},
		IngestTS:       time.Now(),
		SeqProvider:    99,
		ProviderID:     "binance",
	}
	ev.Reset()

	if ev.TraceID != "" || ev.RoutingVersion != 0 || ev.Kind != 0 || ev.Payload != nil || !ev.IngestTS.IsZero() || ev.SeqProvider != 0 || ev.ProviderID != "" {
		t.Fatalf("event fields not reset: %+v", ev)
	}
}

func TestMergedEventReset(t *testing.T) {
	merged := &events.MergedEvent{
		Event:           events.Event{TraceID: "trace", Kind: events.KindMarketData},
		SourceProviders: []string{"a", "b"},
		MergeWindowID:   "window",
	}

	merged.Reset()

	if merged.TraceID != "" || len(merged.SourceProviders) != 0 || merged.MergeWindowID != "" {
		t.Fatalf("merged event not reset: %+v", merged)
	}
}

func TestExecReportReset(t *testing.T) {
	report := &events.ExecReport{
		TraceID:       "trace",
		ClientOrderID: "order",
		ExchangeID:    "ex",
		Status:        "status",
		Reason:        "reason",
	}

	report.Reset()

	if report.TraceID != "" || report.ClientOrderID != "" || report.ExchangeID != "" || report.Status != "" || report.Reason != "" {
		t.Fatalf("exec report not reset: %+v", report)
	}
}
