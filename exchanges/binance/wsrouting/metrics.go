package wsrouting

import (
	"sort"
	"sync"
	"time"
)

const latencySampleLimit = 1024

// RoutingMetrics captures counters and latency samples for routing operations.
type RoutingMetrics struct {
	mu sync.RWMutex

	MessagesRouted        map[string]uint64
	RoutingErrors         uint64
	ProcessorInvocations  map[string]*ProcessorInvocationMetrics
	ConversionDurations   map[string]*LatencyHistogram
	ProcessorInitFailures uint64
	BackpressureEvents    uint64
	ChannelDepth          map[string]uint64
}

// ProcessorInvocationMetrics tracks successes and failures for a processor.
type ProcessorInvocationMetrics struct {
	Successes uint64
	Errors    uint64
}

// LatencyHistogram stores recent duration samples with percentile calculations.
type LatencyHistogram struct {
	Samples []time.Duration
	P50     time.Duration
	P95     time.Duration
	P99     time.Duration
}

// NewRoutingMetrics constructs an initialized metrics container.
func NewRoutingMetrics() *RoutingMetrics {
	return &RoutingMetrics{
		MessagesRouted:       make(map[string]uint64),
		ProcessorInvocations: make(map[string]*ProcessorInvocationMetrics),
		ConversionDurations:  make(map[string]*LatencyHistogram),
		ChannelDepth:         make(map[string]uint64),
	}
}

// RecordRoute increments the routed message counter for a message type.
func (m *RoutingMetrics) RecordRoute(messageTypeID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.MessagesRouted[messageTypeID]++
	m.mu.Unlock()
}

// RecordError increments the routing error counter.
func (m *RoutingMetrics) RecordError() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.RoutingErrors++
	m.mu.Unlock()
}

// RecordProcessing updates invocation counters and latency histogram for a processor.
func (m *RoutingMetrics) RecordProcessing(messageTypeID string, duration time.Duration, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	metrics := m.ensureInvocation(messageTypeID)
	if err != nil {
		metrics.Errors++
	} else {
		metrics.Successes++
	}
	m.trackLatency(messageTypeID, duration)
	m.mu.Unlock()
}

// Snapshot returns a deep copy of the current metrics.
func (m *RoutingMetrics) Snapshot() *RoutingMetrics {
	if m == nil {
		return nil
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	copyMetrics := &RoutingMetrics{
		MessagesRouted:        make(map[string]uint64, len(m.MessagesRouted)),
		RoutingErrors:         m.RoutingErrors,
		ProcessorInvocations:  make(map[string]*ProcessorInvocationMetrics, len(m.ProcessorInvocations)),
		ConversionDurations:   make(map[string]*LatencyHistogram, len(m.ConversionDurations)),
		ProcessorInitFailures: m.ProcessorInitFailures,
		BackpressureEvents:    m.BackpressureEvents,
		ChannelDepth:          make(map[string]uint64, len(m.ChannelDepth)),
	}

	for key, count := range m.MessagesRouted {
		copyMetrics.MessagesRouted[key] = count
	}
	for key, original := range m.ProcessorInvocations {
		copyMetrics.ProcessorInvocations[key] = &ProcessorInvocationMetrics{
			Successes: original.Successes,
			Errors:    original.Errors,
		}
	}
	for key, hist := range m.ConversionDurations {
		clone := &LatencyHistogram{
			Samples: append([]time.Duration(nil), hist.Samples...),
			P50:     hist.P50,
			P95:     hist.P95,
			P99:     hist.P99,
		}
		copyMetrics.ConversionDurations[key] = clone
	}
	for key, depth := range m.ChannelDepth {
		copyMetrics.ChannelDepth[key] = depth
	}
	return copyMetrics
}

// RecordBackpressureStart increments channel depth and event counters.
func (m *RoutingMetrics) RecordBackpressureStart(messageTypeID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.BackpressureEvents++
	m.ChannelDepth[messageTypeID] = m.ChannelDepth[messageTypeID] + 1
	m.mu.Unlock()
}

// RecordBackpressureEnd decrements channel depth gauge for a message type.
func (m *RoutingMetrics) RecordBackpressureEnd(messageTypeID string) {
	if m == nil {
		return
	}
	m.mu.Lock()
	if current, ok := m.ChannelDepth[messageTypeID]; ok {
		if current <= 1 {
			delete(m.ChannelDepth, messageTypeID)
		} else {
			m.ChannelDepth[messageTypeID] = current - 1
		}
	}
	m.mu.Unlock()
}

func (m *RoutingMetrics) ensureInvocation(messageTypeID string) *ProcessorInvocationMetrics {
	inv, ok := m.ProcessorInvocations[messageTypeID]
	if !ok {
		inv = &ProcessorInvocationMetrics{}
		m.ProcessorInvocations[messageTypeID] = inv
	}
	return inv
}

func (m *RoutingMetrics) trackLatency(messageTypeID string, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	hist, ok := m.ConversionDurations[messageTypeID]
	if !ok {
		hist = &LatencyHistogram{Samples: make([]time.Duration, 0, latencySampleLimit)}
		m.ConversionDurations[messageTypeID] = hist
	}
	if len(hist.Samples) == latencySampleLimit {
		copy(hist.Samples[0:], hist.Samples[1:])
		hist.Samples[len(hist.Samples)-1] = duration
	} else {
		hist.Samples = append(hist.Samples, duration)
	}
	calculatePercentiles(hist)
}

func calculatePercentiles(hist *LatencyHistogram) {
	if hist == nil || len(hist.Samples) == 0 {
		hist.P50, hist.P95, hist.P99 = 0, 0, 0
		return
	}
	ordered := append([]time.Duration(nil), hist.Samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	hist.P50 = percentile(ordered, 50)
	hist.P95 = percentile(ordered, 95)
	hist.P99 = percentile(ordered, 99)
}

func percentile(values []time.Duration, pct int) time.Duration {
	if len(values) == 0 {
		return 0
	}
	if pct <= 0 {
		return values[0]
	}
	if pct >= 100 {
		return values[len(values)-1]
	}
	index := (len(values) - 1) * pct / 100
	return values[index]
}
