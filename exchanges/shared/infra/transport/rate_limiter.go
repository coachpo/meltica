package transport

import (
	"context"
	"sync"
	"time"
)

// TokenBucketLimiter is a simple token bucket RateLimiter.
// capacity: maximum number of tokens.
// refillPerSecond: tokens added per second (can be fractional).
type TokenBucketLimiter struct {
	mu              sync.Mutex
	capacity        float64
	tokens          float64
	refillPerSecond float64
	lastRefill      time.Time
}

func NewTokenBucketLimiter(capacity, refillPerSecond float64) *TokenBucketLimiter {
	return &TokenBucketLimiter{
		capacity:        capacity,
		tokens:          capacity,
		refillPerSecond: refillPerSecond,
		lastRefill:      time.Now(),
	}
}

func (t *TokenBucketLimiter) Wait(ctx context.Context) error {
	for {
		// Try to take a token; if not available, wait until next token
		delay := t.tryTake()
		if delay <= 0 {
			return nil
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (t *TokenBucketLimiter) tryTake() time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()
	// Refill based on time elapsed
	now := time.Now()
	elapsed := now.Sub(t.lastRefill).Seconds()
	if elapsed > 0 {
		refill := elapsed * t.refillPerSecond
		t.tokens += refill
		if t.tokens > t.capacity {
			t.tokens = t.capacity
		}
		t.lastRefill = now
	}
	if t.tokens >= 1 {
		t.tokens -= 1
		return 0
	}
	needed := 1 - t.tokens
	seconds := needed / t.refillPerSecond
	if seconds < 0 {
		seconds = 0
	}
	return time.Duration(seconds * float64(time.Second))
}
