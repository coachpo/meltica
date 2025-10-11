package mocks

import (
	"context"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/layers"
)

// Ensure interface compliance.
var _ layers.Business = (*MockBusiness)(nil)

// MockBusiness provides a configurable implementation of layers.Business.
type MockBusiness struct {
	mu sync.Mutex

	ProcessFn  func(context.Context, layers.NormalizedMessage) (layers.BusinessResult, error)
	ValidateFn func(context.Context, layers.BusinessRequest) error
	state      layers.BusinessState

	processCalls  []layers.NormalizedMessage
	validateCalls []layers.BusinessRequest
}

// NewMockBusiness constructs a business mock with sensible defaults.
func NewMockBusiness() *MockBusiness {
	return &MockBusiness{
		ProcessFn: func(context.Context, layers.NormalizedMessage) (layers.BusinessResult, error) {
			return layers.BusinessResult{Success: true}, nil
		},
		ValidateFn: func(context.Context, layers.BusinessRequest) error { return nil },
		state: layers.BusinessState{
			Status:     "idle",
			Metrics:    make(map[string]any),
			LastUpdate: time.Now().UTC(),
		},
	}
}

// Process records the call and delegates to the configured behaviour.
func (m *MockBusiness) Process(ctx context.Context, msg layers.NormalizedMessage) (layers.BusinessResult, error) {
	m.mu.Lock()
	m.processCalls = append(m.processCalls, msg)
	fn := m.ProcessFn
	m.mu.Unlock()
	return fn(ctx, msg)
}

// Validate records the call and delegates to the configured behaviour.
func (m *MockBusiness) Validate(ctx context.Context, req layers.BusinessRequest) error {
	m.mu.Lock()
	m.validateCalls = append(m.validateCalls, req)
	fn := m.ValidateFn
	m.mu.Unlock()
	return fn(ctx, req)
}

// GetState returns a snapshot of the current state.
func (m *MockBusiness) GetState() layers.BusinessState {
	m.mu.Lock()
	defer m.mu.Unlock()
	stateCopy := m.state
	if stateCopy.Metrics != nil {
		copyMetrics := make(map[string]any, len(stateCopy.Metrics))
		for k, v := range stateCopy.Metrics {
			copyMetrics[k] = v
		}
		stateCopy.Metrics = copyMetrics
	}
	return stateCopy
}

// SetState updates the internal state snapshot.
func (m *MockBusiness) SetState(state layers.BusinessState) {
	m.mu.Lock()
	m.state = state
	m.mu.Unlock()
}

// ProcessCalls returns the recorded Process inputs.
func (m *MockBusiness) ProcessCalls() []layers.NormalizedMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]layers.NormalizedMessage, len(m.processCalls))
	copy(calls, m.processCalls)
	return calls
}

// ValidateCalls returns the recorded Validate inputs.
func (m *MockBusiness) ValidateCalls() []layers.BusinessRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	calls := make([]layers.BusinessRequest, len(m.validateCalls))
	copy(calls, m.validateCalls)
	return calls
}
