package observability

import (
	"errors"
	"testing"
)

type testLogger struct {
	msg    string
	fields []Field
}

func (l *testLogger) Debug(string, ...Field) {}
func (l *testLogger) Info(string, ...Field)  {}

func (l *testLogger) Error(msg string, fields ...Field) {
	l.msg = msg
	l.fields = append([]Field(nil), fields...)
}

func TestAggregateErrorsReturnsNilForEmpty(t *testing.T) {
	if err := AggregateErrors("op", nil); err != nil {
		t.Fatalf("expected nil for empty errors, got %v", err)
	}
	if err := AggregateErrors("op", []error{nil, nil}); err != nil {
		t.Fatalf("expected nil for nil entries, got %v", err)
	}
}

func TestAggregateErrorsLogsAndWraps(t *testing.T) {
	logger := &testLogger{}
	SetLogger(logger)
	defer SetLogger(nil)

	inner := errors.New("boom")
	err := AggregateErrors("fanout", []error{inner}, Field{Key: "trace_id", Value: "abc"})
	if err == nil {
		t.Fatalf("expected aggregated error")
	}
	if !errors.Is(err, inner) {
		t.Fatalf("expected error to wrap original")
	}
	if logger.msg != "operation errors" {
		t.Fatalf("unexpected log message: %q", logger.msg)
	}
	foundTrace := false
	for _, field := range logger.fields {
		if field.Key == "trace_id" && field.Value == "abc" {
			foundTrace = true
		}
	}
	if !foundTrace {
		t.Fatalf("expected trace_id field in log output")
	}
}
