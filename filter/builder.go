package filter

import (
	"context"
	"fmt"
	"strings"
)

// StageFactory constructs a Stage for the given request.
type StageFactory func(ctx context.Context, req FilterRequest, adapter Adapter) Stage

// Coordinator orchestrates the lifecycle of a filter pipeline.
type Coordinator struct {
	adapter Adapter
	auth    *AuthContext
}

// NewCoordinator creates a coordinator backed by the provided adapter.
// Authentication context can be provided for private streams and REST requests.
func NewCoordinator(adapter Adapter, auth *AuthContext) *Coordinator {
	return &Coordinator{adapter: adapter, auth: auth}
}

// Stream builds and runs a pipeline for the supplied request.
func (c *Coordinator) Stream(parent context.Context, req FilterRequest) (FilterStream, error) {
	ctx, cancel := context.WithCancel(parent)

	if err := c.validateRequest(req); err != nil {
		cancel()
		return FilterStream{}, err
	}

	result := StageResult{}
	stages, cache := c.buildStages(ctx, req)
	for _, stage := range stages {
		if stage == nil {
			continue
		}
		result = stage.Run(ctx, result)
	}

	stream := FilterStream{
		Events: result.Events,
		Errors: result.Errors,
		cancel: cancel,
		cache:  cache,
	}
	return stream, nil
}

// Close releases adapter resources.
func (c *Coordinator) Close() {
	if c.adapter != nil {
		c.adapter.Close()
	}
}

func (c *Coordinator) buildStages(ctx context.Context, req FilterRequest) ([]Stage, *snapshotCache) {
	stages := []Stage{}
	cache := newSnapshotCache(req.EnableSnapshots)

	if c.adapter != nil {
		// Use multi-source stage for all channel types
		if stage := multiSourceStage(c.adapter, req, c.auth); stage != nil {
			stages = append(stages, stage, newNormalizeStage())

			if throttle := newThrottleStage(req.MinEmitInterval); throttle != nil {
				stages = append(stages, throttle)
			}
			if agg := newAggregationStage(cache, req.BookDepth); agg != nil {
				stages = append(stages, agg)
			}
			if vwap := newVWAPStage(req.EnableVWAP); vwap != nil {
				stages = append(stages, vwap)
			}

			stages = append(stages, newReliabilityStage())

			if obs := newObserverStage(req.Observer); obs != nil {
				stages = append(stages, obs)
			}
		}
	}

	if req.SamplingInterval > 0 {
		stages = append(stages, newSamplingStage(req.SamplingInterval))
	}

	// Dispatch stage ensures result channels are initialized even if no previous stages ran.
	stages = append(stages, dispatchStage())

	return stages, cache
}

func (c *Coordinator) validateRequest(req FilterRequest) error {
	if c.adapter == nil {
		if req.Feeds.Books || req.Feeds.Trades || req.Feeds.Tickers || req.EnablePrivate || len(req.RESTRequests) > 0 {
			return fmt.Errorf("filter: adapter unavailable")
		}
		return nil
	}

	caps := c.adapter.Capabilities()
	var missing []string
	if req.Feeds.Books && !caps.Books {
		missing = append(missing, "books")
	}
	if req.Feeds.Trades && !caps.Trades {
		missing = append(missing, "trades")
	}
	if req.Feeds.Tickers && !caps.Tickers {
		missing = append(missing, "tickers")
	}
	if req.EnablePrivate && !caps.PrivateStreams {
		missing = append(missing, "private_streams")
	}
	if len(req.RESTRequests) > 0 && !caps.RESTEndpoints {
		missing = append(missing, "rest_endpoints")
	}
	if len(missing) > 0 {
		return fmt.Errorf("filter: adapter missing capabilities: %s", strings.Join(missing, ", "))
	}

	// Validate auth context for private streams
	if req.EnablePrivate && c.auth == nil {
		return fmt.Errorf("filter: authentication required for private streams")
	}

	return nil
}
