package transport

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"
)

// nextBackoff returns the exponential backoff duration for the given attempt.
// Attempt 0 represents the initial retry (after the first failure).
func nextBackoff(policy RetryPolicy, attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	base := policy.BaseDelay
	if base <= 0 {
		return 0
	}
	delay := base << attempt
	if policy.MaxDelay > 0 && delay > policy.MaxDelay {
		return policy.MaxDelay
	}
	return delay
}

// waitWithContext sleeps for the provided duration or terminates early if the
// context is cancelled.
func waitWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			return nil
		}
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// isRetryableNetworkError reports whether the error is a transient network failure.
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary()
	}
	return false
}

// shouldRetryStatus reports whether the HTTP status should trigger a retry.
func shouldRetryStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500
}

// parseRetryAfter interprets Retry-After header values. It supports both delta
// seconds and HTTP-date formats. When parsing fails the second return value is
// false.
func parseRetryAfter(header http.Header, now time.Time) (time.Duration, bool) {
	value := header.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(value); err == nil {
		if secs < 0 {
			secs = 0
		}
		return time.Duration(secs) * time.Second, true
	}
	if when, err := http.ParseTime(value); err == nil {
		if when.Before(now) {
			return 0, true
		}
		return when.Sub(now), true
	}
	return 0, false
}
