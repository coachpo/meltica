package connection

import (
	"sync"
	"time"
)

type invalidTracker struct {
	window    time.Duration
	threshold uint32
	now       func() time.Time
	mu        sync.Mutex
	failures  []time.Time
}

func newInvalidTracker(window time.Duration, threshold uint32) *invalidTracker {
	if window <= 0 {
		window = time.Minute
	}
	return &invalidTracker{
		window:    window,
		threshold: threshold,
		now:       time.Now,
		failures:  make([]time.Time, 0, 16),
	}
}

func (t *invalidTracker) recordFailure() (uint32, bool) {
	if t == nil {
		return 0, false
	}
	now := t.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.pruneLocked(now)
	t.failures = append(t.failures, now)
	count := uint32(len(t.failures))
	exceeded := t.threshold > 0 && count >= t.threshold
	return count, exceeded
}

func (t *invalidTracker) reset() {
	if t == nil {
		return
	}
	t.mu.Lock()
	if len(t.failures) > 0 {
		t.failures = t.failures[:0]
	}
	t.mu.Unlock()
}

func (t *invalidTracker) pruneLocked(now time.Time) {
	if t.window <= 0 || len(t.failures) == 0 {
		return
	}
	cutoff := now.Add(-t.window)
	idx := 0
	for _, ts := range t.failures {
		if ts.After(cutoff) {
			break
		}
		idx++
	}
	if idx == 0 {
		return
	}
	newLen := copy(t.failures, t.failures[idx:])
	for i := newLen; i < len(t.failures); i++ {
		t.failures[i] = time.Time{}
	}
	t.failures = t.failures[:newLen]
}

func (t *invalidTracker) clock() time.Time {
	if t == nil || t.now == nil {
		return time.Now()
	}
	return t.now()
}
