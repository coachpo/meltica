package events

import (
	"testing"
	"time"
)

// Example unit test - same package, tests internal implementation
func TestEventReset(t *testing.T) {
	ev := &Event{
		TraceID:        "test-123",
		RoutingVersion: 5,
		Kind:           KindTrade,
		Payload:        "test data",
	}

	ev.Reset()

	if ev.TraceID != "" {
		t.Errorf("expected TraceID to be empty after Reset, got %s", ev.TraceID)
	}
	if ev.RoutingVersion != 0 {
		t.Errorf("expected RoutingVersion to be 0 after Reset, got %d", ev.RoutingVersion)
	}
	if ev.Payload != nil {
		t.Errorf("expected Payload to be nil after Reset, got %v", ev.Payload)
	}
}

func TestEventKindString(t *testing.T) {
	tests := []struct {
		kind EventKind
		want string
	}{
		{KindTrade, "Trade"},
		{KindTicker, "Ticker"},
		{KindBookSnapshot, "BookSnapshot"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.kind.String(); got != tt.want {
				t.Errorf("EventKind.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEventKindIsCritical(t *testing.T) {
	tests := []struct {
		name string
		kind EventKind
		want bool
	}{
		{"Trade is critical", KindTrade, true},
		{"ExecReport is critical", KindExecReport, true},
		{"Ticker is not critical", KindTicker, false},
		{"BookUpdate is not critical", KindBookUpdate, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.kind.IsCritical(); got != tt.want {
				t.Errorf("EventKind.IsCritical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func BenchmarkEventReset(b *testing.B) {
	ev := &Event{
		TraceID:        "test-trace",
		RoutingVersion: 10,
		Kind:           KindTrade,
		EmitTS:         time.Now(),
		IngestTS:       time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev.Reset()
	}
}
