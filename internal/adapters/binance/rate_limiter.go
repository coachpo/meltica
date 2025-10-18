package binance

import (
	"sync"
	"time"
)

// RateLimiter implements token bucket algorithm for rate limiting.
// Binance allows 5 messages per second per connection.
type RateLimiter struct {
	rate       int           // messages per second
	tokens     int           // current available tokens
	maxTokens  int           // maximum tokens (burst)
	lastRefill time.Time     // last token refill time
	mu         sync.Mutex
}

// NewRateLimiter creates a rate limiter with the specified messages per second.
func NewRateLimiter(messagesPerSecond int) *RateLimiter {
	//nolint:exhaustruct // mu is zero-value initialized
	return &RateLimiter{
		rate:       messagesPerSecond,
		tokens:     messagesPerSecond,
		maxTokens:  messagesPerSecond,
		lastRefill: time.Now(),
	}
}

// Allow checks if a message can be sent and consumes a token if so.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens based on time elapsed
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill)
	tokensToAdd := int(elapsed.Seconds() * float64(rl.rate))

	if tokensToAdd > 0 {
		rl.tokens += tokensToAdd
		if rl.tokens > rl.maxTokens {
			rl.tokens = rl.maxTokens
		}
		rl.lastRefill = now
	}

	// Check if we have tokens available
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// Wait blocks until a token is available.
func (rl *RateLimiter) Wait() {
	for !rl.Allow() {
		time.Sleep(100 * time.Millisecond)
	}
}

// Reset resets the rate limiter to full capacity.
func (rl *RateLimiter) Reset() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.tokens = rl.maxTokens
	rl.lastRefill = time.Now()
}
