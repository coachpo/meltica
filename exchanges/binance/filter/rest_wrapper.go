package filter

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/exchanges/shared/infra/transport"
	"github.com/coachpo/meltica/exchanges/shared/routing"
	mdfilter "github.com/coachpo/meltica/filter"
)

// RESTWrapper provides retry and rate limiting capabilities for REST requests
type RESTWrapper struct {
	router       routing.RESTDispatcher
	limiter      *transport.TokenBucketLimiter
	defaultRetry *mdfilter.RetryPolicy
}

// NewRESTWrapper creates a new wrapper with rate limiting and retry capabilities
func NewRESTWrapper(router routing.RESTDispatcher, requestsPerSecond float64, defaultRetry *mdfilter.RetryPolicy) *RESTWrapper {
	var limiter *transport.TokenBucketLimiter
	if requestsPerSecond > 0 {
		// Allow burst up to 2x the rate for short periods
		burstCapacity := requestsPerSecond * 2
		if burstCapacity < 1 {
			burstCapacity = 1
		}
		limiter = transport.NewTokenBucketLimiter(burstCapacity, requestsPerSecond)
	}

	return &RESTWrapper{
		router:       router,
		limiter:      limiter,
		defaultRetry: defaultRetry,
	}
}

// Dispatch implements routing.RESTDispatcher with rate limiting and retry logic
func (w *RESTWrapper) Dispatch(ctx context.Context, msg routing.RESTMessage, out any) error {
	// Apply rate limiting if configured
	if w.limiter != nil {
		if err := w.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait: %w", err)
		}
	}

	// Execute the request through the underlying router
	return w.router.Dispatch(ctx, msg, out)
}

// DefaultRetryPolicy returns a sensible default retry policy for Binance
func DefaultRetryPolicy() *mdfilter.RetryPolicy {
	return &mdfilter.RetryPolicy{
		MaxAttempts:          3,
		BaseDelay:            mdfilter.Duration(100 * time.Millisecond),
		MaxDelay:             mdfilter.Duration(5 * time.Second),
		BackoffMultiplier:    2.0,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
		RetryableErrors: []string{
			"timeout",
			"network",
			"rate limit",
			"connection reset",
			"EOF",
		},
	}
}

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations
func AggressiveRetryPolicy() *mdfilter.RetryPolicy {
	return &mdfilter.RetryPolicy{
		MaxAttempts:          5,
		BaseDelay:            mdfilter.Duration(50 * time.Millisecond),
		MaxDelay:             mdfilter.Duration(10 * time.Second),
		BackoffMultiplier:    1.5,
		RetryableStatusCodes: []int{429, 500, 502, 503, 504},
		RetryableErrors: []string{
			"timeout",
			"network",
			"rate limit",
			"connection reset",
			"EOF",
			"temporary",
		},
	}
}

// ConservativeRetryPolicy returns a conservative retry policy for non-critical operations
func ConservativeRetryPolicy() *mdfilter.RetryPolicy {
	return &mdfilter.RetryPolicy{
		MaxAttempts:          2,
		BaseDelay:            mdfilter.Duration(500 * time.Millisecond),
		MaxDelay:             mdfilter.Duration(2 * time.Second),
		BackoffMultiplier:    1.2,
		RetryableStatusCodes: []int{429, 503},
		RetryableErrors: []string{
			"rate limit",
			"temporary",
		},
	}
}

// WithRetryPolicy creates a new InteractionRequest with the specified retry policy
func WithRetryPolicy(req mdfilter.InteractionRequest, policy *mdfilter.RetryPolicy) mdfilter.InteractionRequest {
	req.RetryPolicy = policy
	return req
}

// WithTimeout creates a new InteractionRequest with the specified timeout
func WithTimeout(req mdfilter.InteractionRequest, timeout time.Duration) mdfilter.InteractionRequest {
	req.Timeout = mdfilter.Duration(timeout)
	return req
}

// WithQueryParams creates a new InteractionRequest with additional query parameters
func WithQueryParams(req mdfilter.InteractionRequest, params map[string]string) mdfilter.InteractionRequest {
	if req.QueryParams == nil {
		req.QueryParams = make(map[string]string)
	}
	for k, v := range params {
		req.QueryParams[k] = v
	}
	return req
}

// WithHeaders creates a new InteractionRequest with additional headers
func WithHeaders(req mdfilter.InteractionRequest, headers map[string]string) mdfilter.InteractionRequest {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	for k, v := range headers {
		req.Headers[k] = v
	}
	return req
}

// WithSigningHint creates a new InteractionRequest with the specified signing hint
func WithSigningHint(req mdfilter.InteractionRequest, hint mdfilter.SigningHint) mdfilter.InteractionRequest {
	req.SigningHint = hint
	return req
}
