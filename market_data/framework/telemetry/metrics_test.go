package telemetry

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data/framework"
)

func TestMetricsAggregatorRecordsResults(t *testing.T) {
	snapshot := &framework.MetricsSnapshot{Session: "sess"}
	aggregator := NewMetricsAggregator(200*time.Millisecond, snapshot)
	aggregator.RecordResult(10*time.Millisecond, false, 128)
	aggregator.RecordResult(20*time.Millisecond, true, 256)
	aggregator.RecordFailure()
	snap := aggregator.Snapshot()
	require.Equal(t, uint64(2), snap.MessagesTotal)
	require.Equal(t, uint64(2), snap.ErrorsTotal)
	require.Equal(t, uint64(384), snap.Allocated)
	require.Equal(t, 200*time.Millisecond, snap.Window())
	require.True(t, snap.P50 >= 10*time.Millisecond)
	require.True(t, snap.P95 >= snap.P50)
}

func TestMetricsAggregatorPrunesWindow(t *testing.T) {
	snapshot := &framework.MetricsSnapshot{Session: "sess"}
	aggregator := NewMetricsAggregator(50*time.Millisecond, snapshot)
	aggregator.RecordResult(10*time.Millisecond, false, 64)
	time.Sleep(60 * time.Millisecond)
	aggregator.RecordResult(15*time.Millisecond, false, 64)
	snap := aggregator.Snapshot()
	require.Equal(t, uint64(2), snap.MessagesTotal)
	require.Equal(t, 15*time.Millisecond, snap.P50)
	require.Equal(t, 15*time.Millisecond, snap.P95)
}
