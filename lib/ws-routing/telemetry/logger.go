package telemetry

import "context"

// Field captures structured logging metadata.
type Field struct {
	Key   string
	Value any
}

// Level identifies the severity of a log entry.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Logger defines vendor-neutral structured logging operations.
type Logger interface {
	Debug(ctx context.Context, message string, fields ...Field)
	Info(ctx context.Context, message string, fields ...Field)
	Warn(ctx context.Context, message string, fields ...Field)
	Error(ctx context.Context, message string, fields ...Field)
}

// Sink receives fully rendered log entries.
type Sink interface {
	Log(ctx context.Context, level Level, message string, fields ...Field)
}

// New constructs a logger writing to the supplied sink.
func New(sink Sink) Logger {
	if sink == nil {
		sink = noopSink{}
	}
	return &structuredLogger{sink: sink}
}

// NewNoop returns a logger that discards all entries.
func NewNoop() Logger {
	return &structuredLogger{sink: noopSink{}}
}

type structuredLogger struct {
	sink Sink
}

func (l *structuredLogger) Debug(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, LevelDebug, message, fields...)
}

func (l *structuredLogger) Info(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, LevelInfo, message, fields...)
}

func (l *structuredLogger) Warn(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, LevelWarn, message, fields...)
}

func (l *structuredLogger) Error(ctx context.Context, message string, fields ...Field) {
	l.log(ctx, LevelError, message, fields...)
}

func (l *structuredLogger) log(ctx context.Context, level Level, message string, fields ...Field) {
	if l == nil || l.sink == nil {
		return
	}
	l.sink.Log(ctx, level, message, cloneFields(fields)...)
}

func cloneFields(fields []Field) []Field {
	if len(fields) == 0 {
		return nil
	}
	copyOf := make([]Field, len(fields))
	copy(copyOf, fields)
	return copyOf
}

type noopSink struct{}

func (noopSink) Log(context.Context, Level, string, ...Field) {}
