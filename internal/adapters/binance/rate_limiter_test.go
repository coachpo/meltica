package binance

import (
	"sync"
	"testing"
	"time"
)

func TestRateLimiter_TokenRefill(t *testing.T) {
	rl := NewRateLimiter(2)
	
	rl.Allow()
	rl.Allow()
	
	if rl.Allow() {
		t.Error("should be denied before refill")
	}
	
	time.Sleep(1100 * time.Millisecond)
	
	if !rl.Allow() {
		t.Error("should be allowed after refill period")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(1)
	
	rl.Allow()
	
	start := time.Now()
	rl.Wait()
	elapsed := time.Since(start)
	
	if elapsed < 100*time.Millisecond {
		t.Errorf("expected wait to take at least 100ms, took %v", elapsed)
	}
}

func TestRateLimiter_Concurrent(t *testing.T) {
	rl := NewRateLimiter(10)
	
	var wg sync.WaitGroup
	allowed := 0
	denied := 0
	var mu sync.Mutex
	
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if rl.Allow() {
				mu.Lock()
				allowed++
				mu.Unlock()
			} else {
				mu.Lock()
				denied++
				mu.Unlock()
			}
		}()
	}
	
	wg.Wait()
	
	if allowed != 10 {
		t.Errorf("expected 10 allowed, got %d", allowed)
	}
	if denied != 10 {
		t.Errorf("expected 10 denied, got %d", denied)
	}
}

func TestRateLimiter_BurstCapacity(t *testing.T) {
	rl := NewRateLimiter(5)
	
	time.Sleep(2 * time.Second)
	
	count := 0
	for i := 0; i < 10; i++ {
		if rl.Allow() {
			count++
		}
	}
	
	if count != 5 {
		t.Errorf("expected burst of 5, got %d", count)
	}
}

func TestRateLimiter_ZeroRate(t *testing.T) {
	rl := NewRateLimiter(0)
	
	if rl.Allow() {
		t.Error("expected all requests to be denied with zero rate")
	}
}

func TestRateLimiter_HighRate(t *testing.T) {
	rl := NewRateLimiter(1000)
	
	count := 0
	for i := 0; i < 1000; i++ {
		if rl.Allow() {
			count++
		}
	}
	
	if count != 1000 {
		t.Errorf("expected 1000 allowed immediately, got %d", count)
	}
}
