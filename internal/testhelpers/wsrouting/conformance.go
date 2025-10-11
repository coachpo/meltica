package wsroutingtest

import (
	"bytes"
	"context"
	"sync"
	"testing"

	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

// Detector exposes the Detect contract required by adapter routing tables.
type Detector interface {
	Detect([]byte) (string, error)
}

// ConformanceCase enumerates an adapter message scenario.
type ConformanceCase struct {
	Name           string
	Raw            []byte
	ExpectedType   string
	ExpectedStored []byte
}

// Adapter wires the components required to exercise routing conformance.
type Adapter struct {
	Detector Detector
	Parser   wsrouting.Parser
	Publish  wsrouting.PublishFunc
	Recorder *Recorder
}

// AssertAdapterConformance verifies that the supplied adapter satisfies the expected scenarios.
func AssertAdapterConformance(t *testing.T, adapter Adapter, cases []ConformanceCase) {
	t.Helper()
	if adapter.Detector == nil {
		t.Fatalf("adapter detector required")
	}
	if adapter.Parser == nil {
		t.Fatalf("adapter parser required")
	}
	if adapter.Publish == nil {
		t.Fatalf("adapter publish function required")
	}
	if adapter.Recorder == nil {
		t.Fatalf("adapter recorder required")
	}
	ctx := context.Background()
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Helper()
			adapter.Recorder.Reset()
			typeID, err := adapter.Detector.Detect(tc.Raw)
			if err != nil {
				t.Fatalf("detect: %v", err)
			}
			if typeID != tc.ExpectedType {
				t.Fatalf("unexpected detected type: got %q want %q", typeID, tc.ExpectedType)
			}
			msg, err := adapter.Parser.Parse(ctx, tc.Raw)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			if msg == nil {
				t.Fatalf("parser returned nil message")
			}
			if msg.Type != tc.ExpectedType {
				t.Fatalf("unexpected message type: got %q want %q", msg.Type, tc.ExpectedType)
			}
			rawValue, ok := msg.Payload["raw"]
			if !ok {
				t.Fatalf("message payload missing raw entry")
			}
			stored, ok := rawValue.([]byte)
			if !ok {
				t.Fatalf("raw payload expected []byte, got %T", rawValue)
			}
			if !bytes.Equal(stored, tc.ExpectedStored) {
				t.Fatalf("unexpected stored payload: got %q want %q", stored, tc.ExpectedStored)
			}
			if err := adapter.Publish(ctx, msg); err != nil {
				t.Fatalf("publish: %v", err)
			}
			recorded, ok := adapter.Recorder.Last()
			if !ok {
				t.Fatalf("expected recorder to capture event")
			}
			if recorded.Type != tc.ExpectedType {
				t.Fatalf("unexpected recorded type: got %q want %q", recorded.Type, tc.ExpectedType)
			}
			if !bytes.Equal(recorded.Payload, tc.ExpectedStored) {
				t.Fatalf("unexpected recorded payload: got %q want %q", recorded.Payload, tc.ExpectedStored)
			}
		})
	}
}

// RecordedEvent captures a published routing payload.
type RecordedEvent struct {
	Type    string
	Payload []byte
}

// Recorder accumulates published routing events.
type Recorder struct {
	mu    sync.Mutex
	items []RecordedEvent
}

// NewRecorder constructs an empty recorder instance.
func NewRecorder() *Recorder {
	return &Recorder{}
}

// Sink exposes a RawMessagePublisher-compatible function for capturing events.
func (r *Recorder) Sink() func(context.Context, string, []byte) error {
	return func(_ context.Context, typ string, payload []byte) error {
		r.mu.Lock()
		defer r.mu.Unlock()
		clone := append([]byte(nil), payload...)
		r.items = append(r.items, RecordedEvent{Type: typ, Payload: clone})
		return nil
	}
}

// Reset clears recorded events.
func (r *Recorder) Reset() {
	r.mu.Lock()
	r.items = nil
	r.mu.Unlock()
}

// Last returns the most recently recorded event.
func (r *Recorder) Last() (RecordedEvent, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.items) == 0 {
		return RecordedEvent{}, false
	}
	item := r.items[len(r.items)-1]
	clone := RecordedEvent{Type: item.Type, Payload: append([]byte(nil), item.Payload...)}
	return clone, true
}
