package transport

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"
)

type fakeNetError struct {
	timeout bool
}

func (f fakeNetError) Error() string   { return "fake" }
func (f fakeNetError) Timeout() bool   { return f.timeout }
func (f fakeNetError) Temporary() bool { return true }

func TestNextBackoffRespectsBounds(t *testing.T) {
	policy := RetryPolicy{BaseDelay: time.Millisecond, MaxDelay: 8 * time.Millisecond}
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{-1, time.Millisecond},
		{0, time.Millisecond},
		{1, 2 * time.Millisecond},
		{3, 8 * time.Millisecond},
		{5, 8 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := nextBackoff(policy, tc.attempt); got != tc.want {
			t.Fatalf("attempt %d: expected %v, got %v", tc.attempt, tc.want, got)
		}
	}
}

func TestWaitWithContextHonoursCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitWithContext(ctx, time.Second); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if err := waitWithContext(context.Background(), 0); err != nil {
		t.Fatalf("expected immediate return, got %v", err)
	}
}

func TestIsRetryableNetworkError(t *testing.T) {
	if isRetryableNetworkError(nil) {
		t.Fatalf("nil should not be retryable")
	}
	if isRetryableNetworkError(context.Canceled) {
		t.Fatalf("context cancellation should not be retryable")
	}
	if !isRetryableNetworkError(fakeNetError{timeout: true}) {
		t.Fatalf("timeout net errors should retry")
	}
	if !isRetryableNetworkError(fakeNetError{timeout: false}) {
		t.Fatalf("temporary net errors should retry")
	}
}

func TestShouldRetryStatus(t *testing.T) {
	if !shouldRetryStatus(http.StatusTooManyRequests) {
		t.Fatalf("expected retry for 429")
	}
	if !shouldRetryStatus(http.StatusInternalServerError) {
		t.Fatalf("expected retry for 500")
	}
	if shouldRetryStatus(http.StatusBadRequest) {
		t.Fatalf("expected no retry for 400")
	}
}

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC)
	hdr := http.Header{"Retry-After": []string{"3"}}
	if wait, ok := parseRetryAfter(hdr, now); !ok || wait != 3*time.Second {
		t.Fatalf("expected delta seconds, got wait=%v ok=%v", wait, ok)
	}
	hdr.Set("Retry-After", now.Add(2*time.Second).Format(http.TimeFormat))
	if wait, ok := parseRetryAfter(hdr, now); !ok || wait != 2*time.Second {
		t.Fatalf("expected http date, got wait=%v ok=%v", wait, ok)
	}
	hdr.Set("Retry-After", "bad")
	if _, ok := parseRetryAfter(hdr, now); ok {
		t.Fatalf("expected parse failure")
	}
}
