package telemetry_test

import (
	"context"
	"sync"
	"testing"

	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

type captureEntry struct {
	level   telemetry.Level
	message string
	fields  []telemetry.Field
}

type captureSink struct {
	mu      sync.Mutex
	entries []captureEntry
}

func (s *captureSink) Log(_ context.Context, level telemetry.Level, message string, fields ...telemetry.Field) {
	s.mu.Lock()
	s.entries = append(s.entries, captureEntry{level: level, message: message, fields: append([]telemetry.Field(nil), fields...)})
	s.mu.Unlock()
}

func TestStructuredLoggerDispatchesToSink(t *testing.T) {
	sink := &captureSink{}
	logger := telemetry.New(sink)
	ctx := context.Background()
	logger.Info(ctx, "router started", telemetry.Field{Key: "session_id", Value: "abc"})
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if len(sink.entries) != 1 {
		t.Fatalf("expected exactly one entry, got %d", len(sink.entries))
	}
	entry := sink.entries[0]
	if entry.level != telemetry.LevelInfo {
		t.Fatalf("expected level info, got %s", entry.level)
	}
	if entry.message != "router started" {
		t.Fatalf("unexpected message %q", entry.message)
	}
	if len(entry.fields) != 1 || entry.fields[0].Key != "session_id" || entry.fields[0].Value != "abc" {
		t.Fatalf("unexpected fields %#v", entry.fields)
	}
}

func TestStructuredLoggerClonesFields(t *testing.T) {
	sink := &captureSink{}
	logger := telemetry.New(sink)
	ctx := context.Background()
	fields := []telemetry.Field{{Key: "symbol", Value: "BTC-USD"}}
	logger.Warn(ctx, "slow handler", fields...)
	fields[0].Value = "ETH-USD"
	sink.mu.Lock()
	defer sink.mu.Unlock()
	if sink.entries[0].fields[0].Value != "BTC-USD" {
		t.Fatalf("expected field value to remain BTC-USD, got %v", sink.entries[0].fields[0].Value)
	}
}

func TestNoopLoggerDoesNotPanic(t *testing.T) {
	logger := telemetry.NewNoop()
	logger.Debug(context.Background(), "noop")
	logger.Info(context.Background(), "noop")
	logger.Warn(context.Background(), "noop")
	logger.Error(context.Background(), "noop")
}
