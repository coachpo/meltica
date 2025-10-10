package connection

import (
	"testing"
	"time"
)

func TestInvalidTrackerRecordsFailuresAndThreshold(t *testing.T) {
	base := time.Unix(0, 0)
	tracker := newInvalidTracker(2*time.Second, 2)
	tracker.now = func() time.Time { return base }

	count, exceeded := tracker.recordFailure()
	if count != 1 || exceeded {
		t.Fatalf("expected count=1 exceeded=false, got count=%d exceeded=%v", count, exceeded)
	}

	tracker.now = func() time.Time { return base.Add(time.Second) }
	count, exceeded = tracker.recordFailure()
	if count != 2 || !exceeded {
		t.Fatalf("expected second failure to reach threshold, got count=%d exceeded=%v", count, exceeded)
	}
}

func TestInvalidTrackerPrunesAndResets(t *testing.T) {
	base := time.Unix(0, 0)
	tracker := newInvalidTracker(time.Second, 10)
	tracker.now = func() time.Time { return base }
	tracker.recordFailure()
	tracker.recordFailure()

	tracker.now = func() time.Time { return base.Add(3 * time.Second) }
	if count, _ := tracker.recordFailure(); count != 1 {
		t.Fatalf("expected prune to remove stale entries, got count=%d", count)
	}

	tracker.reset()
	if len(tracker.failures) != 0 {
		t.Fatalf("expected reset to clear failures, got %d entries", len(tracker.failures))
	}
}
