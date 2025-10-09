package telemetry

import (
	"math"
	"sort"
	"sync"
	"time"

	"github.com/coachpo/meltica/market_data/framework"
)

const defaultMetricsWindow = time.Minute

type latencySample struct {
	when  time.Time
	value time.Duration
}

// MetricsAggregator tracks rolling latency and allocation metrics for a session.
type MetricsAggregator struct {
	window   time.Duration
	snapshot *framework.MetricsSnapshot

	mu        sync.Mutex
	samples   []latencySample
	messages  uint64
	errors    uint64
	allocated uint64
}

// NewMetricsAggregator constructs an aggregator backed by the provided snapshot.
func NewMetricsAggregator(window time.Duration, snapshot *framework.MetricsSnapshot) *MetricsAggregator {
	if window <= 0 {
		window = defaultMetricsWindow
	}
	if snapshot == nil {
		snapshot = &framework.MetricsSnapshot{WindowLength: window}
	}
	snapshot.WindowLength = window
	return &MetricsAggregator{
		window:   window,
		snapshot: snapshot,
		samples:  make([]latencySample, 0, 128),
	}
}

// RecordResult records a processed message with associated latency and allocation statistics.
func (m *MetricsAggregator) RecordResult(latency time.Duration, errored bool, allocBytes uint64) {
	now := time.Now().UTC()
	m.mu.Lock()
	m.messages++
	if errored {
		m.errors++
	}
	m.allocated += allocBytes
	m.samples = append(m.samples, latencySample{when: now, value: latency})
	m.prune(now)
	m.recomputeLocked()
	m.mu.Unlock()
}

// RecordFailure increments error counters for failures outside the normal dispatch cycle.
func (m *MetricsAggregator) RecordFailure() {
	m.mu.Lock()
	m.errors++
	m.recomputeLocked()
	m.mu.Unlock()
}

// Snapshot returns a copy of the latest metrics snapshot.
func (m *MetricsAggregator) Snapshot() framework.MetricsSnapshot {
	m.mu.Lock()
	copy := *m.snapshot
	m.mu.Unlock()
	return copy
}

func (m *MetricsAggregator) prune(now time.Time) {
	if m.window <= 0 || len(m.samples) == 0 {
		return
	}
	cutoff := now.Add(-m.window)
	i := 0
	for _, sample := range m.samples {
		if sample.when.After(cutoff) {
			break
		}
		i++
	}
	if i == 0 {
		return
	}
	copy(m.samples, m.samples[i:])
	m.samples = m.samples[:len(m.samples)-i]
}

func (m *MetricsAggregator) recomputeLocked() {
	m.snapshot.WindowLength = m.window
	m.snapshot.MessagesTotal = m.messages
	m.snapshot.ErrorsTotal = m.errors
	m.snapshot.Allocated = m.allocated
	if len(m.samples) == 0 {
		m.snapshot.P50 = 0
		m.snapshot.P95 = 0
		return
	}
	durations := make([]time.Duration, 0, len(m.samples))
	for _, sample := range m.samples {
		durations = append(durations, sample.value)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	m.snapshot.P50 = percentile(durations, 0.50)
	m.snapshot.P95 = percentile(durations, 0.95)
}

func percentile(values []time.Duration, quantile float64) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if quantile <= 0 {
		return values[0]
	}
	if quantile >= 1 {
		return values[len(values)-1]
	}
	index := int(math.Ceil(quantile*float64(len(values)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
