package pipeline

import "context"

// PipelineStepResult represents the output channels produced by a pipeline step.
type PipelineStepResult struct {
	Events <-chan ClientEvent
	Errors <-chan error
}

// PipelineStep defines a processing unit within the filter pipeline.
type PipelineStep interface {
	Run(ctx context.Context, input PipelineStepResult) PipelineStepResult
	Name() string
}

// PipelineStepFunc wraps a function and implements PipelineStep.
type PipelineStepFunc struct {
	name string
	fn   func(ctx context.Context, input PipelineStepResult) PipelineStepResult
}

// Run executes the wrapped function.
func (s PipelineStepFunc) Run(ctx context.Context, input PipelineStepResult) PipelineStepResult {
	if s.fn == nil {
		return PipelineStepResult{}
	}
	return s.fn(ctx, input)
}

// Name returns the configured step name.
func (s PipelineStepFunc) Name() string {
	if s.name == "" {
		return "step"
	}
	return s.name
}

// NewPipelineStepFunc constructs a PipelineStep from a name and function.
func NewPipelineStepFunc(name string, fn func(ctx context.Context, input PipelineStepResult) PipelineStepResult) PipelineStep {
	return PipelineStepFunc{name: name, fn: fn}
}
