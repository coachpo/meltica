package router

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/coachpo/meltica/market_data/processors"
)

// loadTestPayload retrieves a JSON fixture from the market_data/testdata directory.
func loadTestPayload(tb testing.TB, filename string) []byte {
	tb.Helper()
	path := filepath.Join("..", "..", "testdata", filename)
	data, err := os.ReadFile(path)
	if err != nil {
		tb.Fatalf("load fixture %s: %v", filename, err)
	}
	return data
}

// mockProcessor implements processors.Processor for routing tests.
type mockProcessor struct {
	id         string
	initDelay  time.Duration
	initErr    error
	processErr error

	mu        sync.Mutex
	received  [][]byte
	contexts  []context.Context
	processed int
}

var _ processors.Processor = (*mockProcessor)(nil)

func newMockProcessor(id string) *mockProcessor {
	return &mockProcessor{id: id}
}

func (m *mockProcessor) Initialize(ctx context.Context) error {
	if m.initDelay > 0 {
		select {
		case <-time.After(m.initDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return m.initErr
}

func (m *mockProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if m.processErr != nil {
		return nil, m.processErr
	}
	clone := make([]byte, len(raw))
	copy(clone, raw)
	m.mu.Lock()
	m.received = append(m.received, clone)
	m.contexts = append(m.contexts, ctx)
	m.processed++
	m.mu.Unlock()
	return clone, nil
}

func (m *mockProcessor) MessageTypeID() string { return m.id }

func (m *mockProcessor) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processed
}

func (m *mockProcessor) Received() [][]byte {
	m.mu.Lock()
	defer m.mu.Unlock()
	copies := make([][]byte, len(m.received))
	for i, payload := range m.received {
		clone := make([]byte, len(payload))
		copy(clone, payload)
		copies[i] = clone
	}
	return copies
}
