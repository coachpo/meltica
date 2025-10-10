package bridge

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/exchanges/shared/infra/transport"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

// Wrapper provides rate limiting around a Dispatcher.
type Wrapper struct {
	dispatcher    Dispatcher
	limiter       *transport.TokenBucketLimiter
	defaultPolicy *mdfilter.RetryPolicy
}

// NewWrapper constructs a dispatcher with optional rate limiting and retry policy metadata.
func NewWrapper(dispatcher Dispatcher, requestsPerSecond float64, defaultRetry *mdfilter.RetryPolicy) *Wrapper {
	var limiter *transport.TokenBucketLimiter
	if requestsPerSecond > 0 {
		burst := requestsPerSecond * 2
		if burst < 1 {
			burst = 1
		}
		limiter = transport.NewTokenBucketLimiter(burst, requestsPerSecond)
	}
	return &Wrapper{
		dispatcher:    dispatcher,
		limiter:       limiter,
		defaultPolicy: defaultRetry,
	}
}

// Dispatch enforces rate limiting before delegating to the underlying dispatcher.
func (w *Wrapper) Dispatch(ctx context.Context, req mdfilter.InteractionRequest, out any) error {
	if w.limiter != nil {
		if err := w.limiter.Wait(ctx); err != nil {
			return fmt.Errorf("rate limit wait: %w", err)
		}
	}
	if req.RetryPolicy == nil && w.defaultPolicy != nil {
		req.RetryPolicy = w.defaultPolicy
	}
	return w.dispatcher.Dispatch(ctx, req, out)
}

// DefaultRetryPolicy returns a sensible default retry policy for Binance REST calls.
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

// AggressiveRetryPolicy returns a more aggressive retry policy for critical operations.
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

// ConservativeRetryPolicy returns a conservative retry policy for non-critical operations.
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

// WithRetryPolicy applies a retry policy to an InteractionRequest.
func WithRetryPolicy(req mdfilter.InteractionRequest, policy *mdfilter.RetryPolicy) mdfilter.InteractionRequest {
	req.RetryPolicy = policy
	return req
}

// WithTimeout sets a timeout on an InteractionRequest.
func WithTimeout(req mdfilter.InteractionRequest, timeout time.Duration) mdfilter.InteractionRequest {
	req.Timeout = mdfilter.Duration(timeout)
	return req
}

// WithQueryParams adds query parameters to an InteractionRequest.
func WithQueryParams(req mdfilter.InteractionRequest, params map[string]string) mdfilter.InteractionRequest {
	if req.QueryParams == nil {
		req.QueryParams = make(map[string]string)
	}
	for k, v := range params {
		req.QueryParams[k] = v
	}
	return req
}

// WithHeaders adds headers to an InteractionRequest.
func WithHeaders(req mdfilter.InteractionRequest, headers map[string]string) mdfilter.InteractionRequest {
	if req.Headers == nil {
		req.Headers = make(map[string]string)
	}
	for k, v := range headers {
		req.Headers[k] = v
	}
	return req
}

// WithSigningHint sets the signing hint on an InteractionRequest.
func WithSigningHint(req mdfilter.InteractionRequest, hint mdfilter.SigningHint) mdfilter.InteractionRequest {
	req.SigningHint = hint
	return req
}
