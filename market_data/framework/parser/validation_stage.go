package parser

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
)

const defaultValidationWindow = time.Minute

// Validator defines a rule that inspects and optionally rejects an envelope.
//
// Deprecated: Implement processors.Processor validation within Process instead
// of using standalone validator stages.
type Validator interface {
	Validate(*framework.MessageEnvelope) error
}

// ValidatorFunc adapts a function into a Validator.
//
// Deprecated: Prefer inline validation in processor implementations.
type ValidatorFunc func(*framework.MessageEnvelope) error

// Validate executes the wrapped function.
func (fn ValidatorFunc) Validate(env *framework.MessageEnvelope) error {
	if fn == nil {
		return nil
	}
	return fn(env)
}

// ValidationResult captures the outcome of a validation cycle.
//
// Deprecated: Processors should return typed errors directly and rely on
// routing metrics rather than ValidationResult tracking.
type ValidationResult struct {
	Valid             bool
	InvalidCount      uint32
	ThresholdExceeded bool
	Err               error
}

// ValidationStage coordinates validator execution and invalid message tracking.
//
// Deprecated: Validation should be performed within processors registered to
// the routing framework instead of a separate validation stage.
type ValidationStage struct {
	validators []Validator
	window     time.Duration
	threshold  uint32
	now        func() time.Time

	mu       sync.Mutex
	failures []time.Time
}

// ValidationOption configures ValidationStage behavior.
//
// Deprecated: Configure validation logic directly on processors.
type ValidationOption func(*ValidationStage)

// WithValidators registers validator implementations for the stage.
//
// Deprecated: Add validation logic inside processors instead of using
// ValidationStage options.
func WithValidators(validators ...Validator) ValidationOption {
	return func(stage *ValidationStage) {
		for _, v := range validators {
			if v == nil {
				continue
			}
			stage.validators = append(stage.validators, v)
		}
	}
}

// WithValidationWindow sets the sliding window used to report invalid counts.
//
// Deprecated: Utilize routing metrics to observe error rates instead of
// ValidationStage windows.
func WithValidationWindow(window time.Duration) ValidationOption {
	return func(stage *ValidationStage) {
		stage.window = window
	}
}

// WithInvalidThreshold configures the threshold after which callers may take action.
//
// Deprecated: Use router metrics and processor error handling for thresholding.
func WithInvalidThreshold(threshold uint32) ValidationOption {
	return func(stage *ValidationStage) {
		stage.threshold = threshold
	}
}

// WithClock overrides the clock source used for window calculations.
//
// Deprecated: Provided for backward compatibility only.
func WithClock(clock func() time.Time) ValidationOption {
	return func(stage *ValidationStage) {
		if clock != nil {
			stage.now = clock
		}
	}
}

// NewValidationStage constructs a validation pipeline with optional configuration.
//
// Deprecated: Register processors with the router and perform validation there
// instead of constructing validation stages.
func NewValidationStage(opts ...ValidationOption) *ValidationStage {
	stage := &ValidationStage{
		window: defaultValidationWindow,
		now:    time.Now,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(stage)
		}
	}
	return stage
}

// Validate executes registered validators and tracks invalid payloads.
//
// Deprecated: Perform validation inside processor Process methods.
func (s *ValidationStage) Validate(env *framework.MessageEnvelope) ValidationResult {
	if env == nil {
		return ValidationResult{
			Valid: false,
			Err:   errs.New("", errs.CodeInvalid, errs.WithMessage("message envelope required")),
		}
	}

	now := s.clock()
	s.prune(now)
	env.SetValidated(false)

	var validationErrs []error
	for _, validator := range s.validators {
		if validator == nil {
			continue
		}
		if err := validator.Validate(env); err != nil {
			env.AppendError(err)
			validationErrs = append(validationErrs, err)
		}
	}

	if len(validationErrs) == 0 {
		env.SetValidated(true)
		return ValidationResult{
			Valid:        true,
			InvalidCount: s.currentCount(),
		}
	}

	count := s.recordFailure(now)
	msg := "message validation failed"
	if s.threshold > 0 {
		msg = fmt.Sprintf("message validation failed (%d recent violations)", count)
	}

	var cause error
	if len(validationErrs) == 1 {
		cause = validationErrs[0]
	} else {
		cause = errors.Join(validationErrs...)
	}

	return ValidationResult{
		Valid:             false,
		InvalidCount:      count,
		ThresholdExceeded: s.threshold > 0 && count >= s.threshold,
		Err: errs.New(
			"",
			errs.CodeInvalid,
			errs.WithMessage(msg),
			errs.WithCause(cause),
		),
	}
}

func (s *ValidationStage) recordFailure(now time.Time) uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failures = append(s.failures, now)
	return uint32(len(s.failures))
}

func (s *ValidationStage) prune(now time.Time) {
	if s.window <= 0 {
		return
	}
	cutoff := now.Add(-s.window)
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := 0
	for _, ts := range s.failures {
		if ts.After(cutoff) {
			break
		}
		idx++
	}
	if idx == 0 {
		return
	}
	copy(s.failures, s.failures[idx:])
	newLen := len(s.failures) - idx
	for i := newLen; i < len(s.failures); i++ {
		s.failures[i] = time.Time{}
	}
	s.failures = s.failures[:newLen]
}

func (s *ValidationStage) currentCount() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return uint32(len(s.failures))
}

func (s *ValidationStage) clock() time.Time {
	if s == nil || s.now == nil {
		return time.Now()
	}
	return s.now()
}
