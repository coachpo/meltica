package layers

import "context"

// Filter represents a transformation stage in a market-data pipeline.
//
// Contracts:
//   - Filters should be composable and manage internal state safely.
//   - Apply must return promptly when the context is cancelled.
//   - Close releases any resources allocated by Apply.
type Filter interface {
	// Apply wires the filter into the supplied event stream and returns the
	// transformed output stream.
	Apply(ctx context.Context, events <-chan Event) (<-chan Event, error)

	// Name identifies the filter for logging and diagnostics.
	Name() string

	// Close tears down internal resources associated with the filter.
	Close() error
}

// Pipeline coordinates sources and filter stages end to end.
type Pipeline interface {
	// AddSource registers an event source for the pipeline.
	AddSource(source DataSource) error

	// AddFilter appends a filter stage to the pipeline execution graph.
	AddFilter(filter Filter) error

	// Start begins processing events; callers must drain the returned channel
	// until the context is cancelled or an error occurs.
	Start(ctx context.Context) (<-chan Event, error)

	// Stop gracefully halts the pipeline and frees resources.
	Stop() error

	// OnError registers a handler for asynchronous pipeline errors.
	OnError(handler func(error))
}
