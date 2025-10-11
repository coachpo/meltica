package observability

// Metrics provides counters, gauges, and histogram recording primitives.
type Metrics interface {
	IncCounter(name string, value float64, labels map[string]string)
	ObserveHistogram(name string, value float64, labels map[string]string)
	SetGauge(name string, value float64, labels map[string]string)
}

var defaultMetrics Metrics = noopMetrics{}

// SetMetrics overrides the global metrics implementation used by the system.
func SetMetrics(metrics Metrics) {
	if metrics == nil {
		defaultMetrics = noopMetrics{}
		return
	}
	defaultMetrics = metrics
}

// Telemetry returns the current global metrics collector.
func Telemetry() Metrics {
	return defaultMetrics
}

type noopMetrics struct{}

func (noopMetrics) IncCounter(string, float64, map[string]string)       {}
func (noopMetrics) ObserveHistogram(string, float64, map[string]string) {}
func (noopMetrics) SetGauge(string, float64, map[string]string)         {}
