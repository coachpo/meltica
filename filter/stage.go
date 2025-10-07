package filter

import "context"

// StageResult represents the output channels produced by a pipeline stage.
type StageResult struct {
	Events <-chan EventEnvelope
	Errors <-chan error
}

// Stage defines a processing unit within the filter pipeline.
type Stage interface {
	Run(ctx context.Context, input StageResult) StageResult
	Name() string
}

// StageFunc wraps a function and implements Stage.
type StageFunc struct {
	name string
	fn   func(ctx context.Context, input StageResult) StageResult
}

// Run executes the wrapped function.
func (s StageFunc) Run(ctx context.Context, input StageResult) StageResult {
	if s.fn == nil {
		return StageResult{}
	}
	return s.fn(ctx, input)
}

// Name returns the configured stage name.
func (s StageFunc) Name() string {
	if s.name == "" {
		return "stage"
	}
	return s.name
}

// NewStageFunc constructs a Stage from a name and function.
func NewStageFunc(name string, fn func(ctx context.Context, input StageResult) StageResult) Stage {
	return StageFunc{name: name, fn: fn}
}
