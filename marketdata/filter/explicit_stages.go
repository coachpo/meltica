package filter

import (
	"context"
	"time"
)

// StageType represents different types of pipeline stages
type StageType string

const (
	StageTypeAuth        StageType = "auth"
	StageTypeRequest     StageType = "request"
	StageTypeResponse    StageType = "response"
	StageTypeReliability StageType = "reliability"
	StageTypeThrottle    StageType = "throttle"
	StageTypeAggregation StageType = "aggregation"
	StageTypeAnalytics   StageType = "analytics"
	StageTypeObserver    StageType = "observer"
)

// StageConfig contains configuration for pipeline stages
type StageConfig struct {
	Type              StageType
	Channel           ChannelType
	Config            interface{}
	Enabled           bool
	ConcurrencyLimit  int
	RateLimitPerSecond float64
}

// AuthStage handles authentication and session management
type AuthStage struct {
	name    string
	adapter Adapter
	config  *StageConfig
}

// NewAuthStage creates a new authentication stage
func NewAuthStage(adapter Adapter, config *StageConfig) *AuthStage {
	return &AuthStage{
		name:    "auth",
		adapter: adapter,
		config:  config,
	}
}

func (s *AuthStage) Run(ctx context.Context, input StageResult) StageResult {
	// Auth stage doesn't process events, it manages sessions
	// For now, just pass through input
	return input
}

func (s *AuthStage) Name() string {
	return s.name
}

// RequestStage handles REST request scheduling and backpressure
type RequestStage struct {
	name   string
	config *StageConfig
}

// NewRequestStage creates a new request stage
func NewRequestStage(config *StageConfig) *RequestStage {
	return &RequestStage{
		name:   "request",
		config: config,
	}
}

func (s *RequestStage) Run(ctx context.Context, input StageResult) StageResult {
	// Request stage manages REST request scheduling
	// For now, just pass through input
	return input
}

func (s *RequestStage) Name() string {
	return s.name
}

// ResponseStage handles REST response normalization
type ResponseStage struct {
	name   string
	config *StageConfig
}

// NewResponseStage creates a new response stage
func NewResponseStage(config *StageConfig) *ResponseStage {
	return &ResponseStage{
		name:   "response",
		config: config,
	}
}

func (s *ResponseStage) Run(ctx context.Context, input StageResult) StageResult {
	// Response stage normalizes REST responses
	// For now, just pass through input
	return input
}

func (s *ResponseStage) Name() string {
	return s.name
}

// PrivateReliabilityStage handles retry semantics for private feeds
type PrivateReliabilityStage struct {
	name   string
	config *StageConfig
}

// NewPrivateReliabilityStage creates a new private reliability stage
func NewPrivateReliabilityStage(config *StageConfig) *PrivateReliabilityStage {
	return &PrivateReliabilityStage{
		name:   "private_reliability",
		config: config,
	}
}

func (s *PrivateReliabilityStage) Run(ctx context.Context, input StageResult) StageResult {
	// Private reliability stage handles retries for private streams
	// For now, just pass through input
	return input
}

func (s *PrivateReliabilityStage) Name() string {
	return s.name
}

// ChannelThrottleStage provides per-channel throttling
type ChannelThrottleStage struct {
	name   string
	config *StageConfig
}

// NewChannelThrottleStage creates a new channel throttle stage
func NewChannelThrottleStage(config *StageConfig) *ChannelThrottleStage {
	return &ChannelThrottleStage{
		name:   "channel_throttle",
		config: config,
	}
}

func (s *ChannelThrottleStage) Run(ctx context.Context, input StageResult) StageResult {
	// Channel throttle stage provides per-channel rate limiting
	// For now, just pass through input
	return input
}

func (s *ChannelThrottleStage) Name() string {
	return s.name
}

// StageBuilder builds pipeline stages based on configuration
type StageBuilder struct {
	adapter Adapter
	auth    *AuthContext
}

// NewStageBuilder creates a new stage builder
func NewStageBuilder(adapter Adapter, auth *AuthContext) *StageBuilder {
	return &StageBuilder{
		adapter: adapter,
		auth:    auth,
	}
}

// BuildStages builds the pipeline stages based on the filter request
func (b *StageBuilder) BuildStages(ctx context.Context, req FilterRequest) []Stage {
	stages := []Stage{}

	// Add authentication stage if private streams are enabled
	if req.EnablePrivate && b.auth != nil {
		authConfig := &StageConfig{
			Type:    StageTypeAuth,
			Channel: ChannelPrivateWS,
			Enabled: true,
		}
		stages = append(stages, NewAuthStage(b.adapter, authConfig))
	}

	// Add request stage if REST requests are present
	if len(req.RESTRequests) > 0 {
		requestConfig := &StageConfig{
			Type:    StageTypeRequest,
			Channel: ChannelREST,
			Enabled: true,
		}
		stages = append(stages, NewRequestStage(requestConfig))
	}

	// Add multi-source stage for all channel types
	if stage := multiSourceStage(b.adapter, req, b.auth); stage != nil {
		stages = append(stages, stage)
	}

	// Add response stage for REST
	if len(req.RESTRequests) > 0 {
		responseConfig := &StageConfig{
			Type:    StageTypeResponse,
			Channel: ChannelREST,
			Enabled: true,
		}
		stages = append(stages, NewResponseStage(responseConfig))
	}

	// Add normalization stage
	stages = append(stages, newNormalizeStage())

	// Add private reliability stage if private streams are enabled
	if req.EnablePrivate {
		reliabilityConfig := &StageConfig{
			Type:    StageTypeReliability,
			Channel: ChannelPrivateWS,
			Enabled: true,
		}
		stages = append(stages, NewPrivateReliabilityStage(reliabilityConfig))
	}

	// Add throttling stages
	if throttle := newThrottleStage(req.MinEmitInterval); throttle != nil {
		stages = append(stages, throttle)
	}

	// Add channel-specific throttling
	channelThrottleConfig := &StageConfig{
		Type:    StageTypeThrottle,
		Channel: ChannelHybrid,
		Enabled: true,
	}
	stages = append(stages, NewChannelThrottleStage(channelThrottleConfig))

	// Add aggregation stage
	if agg := newAggregationStage(nil, req.BookDepth); agg != nil {
		stages = append(stages, agg)
	}

	// Add analytics stage
	if vwap := newVWAPStage(req.EnableVWAP); vwap != nil {
		stages = append(stages, vwap)
	}

	// Add reliability stage
	stages = append(stages, newReliabilityStage())

	// Add observer stage
	if obs := newObserverStage(req.Observer); obs != nil {
		stages = append(stages, obs)
	}

	// Add sampling stage if configured
	if req.SamplingInterval > 0 {
		stages = append(stages, newSamplingStage(req.SamplingInterval))
	}

	// Add dispatch stage to ensure result channels are initialized
	stages = append(stages, dispatchStage())

	return stages
}

// StageCoordinator coordinates the execution of pipeline stages
type StageCoordinator struct {
	stages []Stage
	cache  *snapshotCache
}

// NewStageCoordinator creates a new stage coordinator
func NewStageCoordinator(stages []Stage, cache *snapshotCache) *StageCoordinator {
	return &StageCoordinator{
		stages: stages,
		cache:  cache,
	}
}

// Execute runs all stages in sequence
func (c *StageCoordinator) Execute(ctx context.Context, initial StageResult) StageResult {
	result := initial
	for _, stage := range c.stages {
		if stage == nil {
			continue
		}
		result = stage.Run(ctx, result)
	}
	return result
}

// StageMetrics contains metrics for pipeline stages
type StageMetrics struct {
	StageName     string
	StageType     StageType
	ProcessedCount int64
	ErrorCount    int64
	Latency       time.Duration
}

// StageRegistry manages stage instances and their configurations
type StageRegistry struct {
	stages   map[string]Stage
	configs  map[string]*StageConfig
	metrics  map[string]*StageMetrics
}

// NewStageRegistry creates a new stage registry
func NewStageRegistry() *StageRegistry {
	return &StageRegistry{
		stages:  make(map[string]Stage),
		configs: make(map[string]*StageConfig),
		metrics: make(map[string]*StageMetrics),
	}
}

// RegisterStage registers a stage with the registry
func (r *StageRegistry) RegisterStage(name string, stage Stage, config *StageConfig) {
	r.stages[name] = stage
	r.configs[name] = config
	r.metrics[name] = &StageMetrics{
		StageName: name,
		StageType: config.Type,
	}
}

// GetStage returns a stage by name
func (r *StageRegistry) GetStage(name string) (Stage, *StageConfig, bool) {
	stage, ok := r.stages[name]
	if !ok {
		return nil, nil, false
	}
	config, ok := r.configs[name]
	if !ok {
		return stage, nil, true
	}
	return stage, config, true
}

// UpdateMetrics updates metrics for a stage
func (r *StageRegistry) UpdateMetrics(name string, processed int64, errors int64, latency time.Duration) {
	if metrics, ok := r.metrics[name]; ok {
		metrics.ProcessedCount += processed
		metrics.ErrorCount += errors
		metrics.Latency = latency
	}
}

// GetMetrics returns all stage metrics
func (r *StageRegistry) GetMetrics() map[string]*StageMetrics {
	return r.metrics
}

// DefaultStageConfigs returns default configurations for all stage types
func DefaultStageConfigs() map[StageType]*StageConfig {
	return map[StageType]*StageConfig{
		StageTypeAuth: {
			Type:    StageTypeAuth,
			Channel: ChannelPrivateWS,
			Enabled: true,
		},
		StageTypeRequest: {
			Type:    StageTypeRequest,
			Channel: ChannelREST,
			Enabled: true,
			ConcurrencyLimit: 10,
			RateLimitPerSecond: 10.0,
		},
		StageTypeResponse: {
			Type:    StageTypeResponse,
			Channel: ChannelREST,
			Enabled: true,
		},
		StageTypeReliability: {
			Type:    StageTypeReliability,
			Channel: ChannelPrivateWS,
			Enabled: true,
		},
		StageTypeThrottle: {
			Type:    StageTypeThrottle,
			Channel: ChannelHybrid,
			Enabled: true,
		},
	}
}