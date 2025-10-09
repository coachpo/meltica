package infra_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	transport "github.com/coachpo/meltica/exchanges/shared/infra/transport"
)

func TestTokenBucketLimiterRespectsRefillRate(t *testing.T) {
	limiter := transport.NewTokenBucketLimiter(1, 100) // 1 token every 10ms
	ctx := context.Background()

	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("first wait returned error: %v", err)
	}

	start := time.Now()
	if err := limiter.Wait(ctx); err != nil {
		t.Fatalf("second wait returned error: %v", err)
	}
	delay := time.Since(start)
	if delay < 5*time.Millisecond {
		t.Fatalf("expected throttling delay, got %s", delay)
	}
	if delay > 50*time.Millisecond {
		t.Fatalf("unexpectedly long throttling delay %s", delay)
	}
}

func TestTokenBucketLimiterHonorsContextCancellation(t *testing.T) {
	limiter := transport.NewTokenBucketLimiter(0, 0.5) // one token every 2 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := limiter.Wait(ctx)
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Fatalf("context cancellation took too long: %s", elapsed)
	}
}

type recordingLimiter struct {
	calls int
	err   error
}

func (r *recordingLimiter) Wait(ctx context.Context) error {
	r.calls++
	if r.err != nil {
		return r.err
	}
	return nil
}

func TestClientRateLimiterIntegration(t *testing.T) {
	t.Run("wait invoked", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		t.Cleanup(server.Close)

		limiter := &recordingLimiter{}
		waits := 0

		client := &transport.Client{
			HTTP:        server.Client(),
			RateLimiter: limiter,
			BaseURL:     server.URL,
			OnRLWait: func(wait time.Duration) {
				waits++
				if wait != 0 {
					t.Fatalf("expected zero wait duration from rate limiter, got %s", wait)
				}
			},
		}

		_, status, err := client.DoWithHeaders(context.Background(), http.MethodGet, "/", nil, nil, false, http.Header{}, nil)
		if err != nil {
			t.Fatalf("DoWithHeaders returned error: %v", err)
		}
		if status != http.StatusOK {
			t.Fatalf("unexpected status %d", status)
		}
		if limiter.calls != 1 {
			t.Fatalf("expected 1 limiter invocation, got %d", limiter.calls)
		}
		if waits != 1 {
			t.Fatalf("expected OnRLWait to run once, got %d", waits)
		}
	})

	t.Run("limiter error propagates", func(t *testing.T) {
		limiter := &recordingLimiter{err: context.Canceled}
		client := &transport.Client{
			HTTP:        &http.Client{},
			RateLimiter: limiter,
			BaseURL:     "https://example.com",
		}

		_, _, err := client.DoWithHeaders(context.Background(), http.MethodGet, "/resource", nil, nil, false, http.Header{}, nil)
		if err == nil {
			t.Fatal("expected limiter error to propagate")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
		if limiter.calls != 1 {
			t.Fatalf("expected limiter to be invoked exactly once, got %d", limiter.calls)
		}
	})
}
