package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework"
)

func TestMetricsHandlerReturnsSnapshot(t *testing.T) {
	snapshot := &framework.MetricsSnapshot{
		Session:       "sess",
		WindowLength:  time.Second,
		MessagesTotal: 5,
		ErrorsTotal:   1,
		P50:           10 * time.Millisecond,
		P95:           20 * time.Millisecond,
		Allocated:     1024,
	}
	reporter := &stubMetricsReporter{sample: snapshot}
	engine := &stubEngine{reporter: reporter}
	handler := NewMetricsHandler(engine)
	require.NotNil(t, handler)
	router := NewRouter(RouterConfig{})
	handler.Register(router)
	req := httptest.NewRequest(http.MethodGet, "/v1/stream/sessions/sess/metrics", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &payload))
	require.Equal(t, float64(1), payload["windowSeconds"])
	require.Equal(t, float64(5), payload["messages"])
	require.Equal(t, float64(1), payload["errors"])
	require.Equal(t, float64(10), payload["p50LatencyMs"])
	require.Equal(t, float64(20), payload["p95LatencyMs"])
	require.Equal(t, float64(1024), payload["allocBytes"])
}

func TestMetricsHandlerReturnsNotFound(t *testing.T) {
	reporter := &stubMetricsReporter{}
	engine := &stubEngine{reporter: reporter}
	handler := NewMetricsHandler(engine)
	router := NewRouter(RouterConfig{})
	handler.Register(router)
	req := httptest.NewRequest(http.MethodGet, "/v1/stream/sessions/unknown/metrics", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusNotFound, resp.Code)
}

type stubEngine struct {
	reporter stream.MetricsReporter
}

func (s *stubEngine) RegisterHandler(reg stream.HandlerRegistration) error { return nil }

func (s *stubEngine) Dial(ctx context.Context) (stream.Session, error) { return nil, nil }

func (s *stubEngine) Metrics() stream.MetricsReporter { return s.reporter }

func (s *stubEngine) Close(ctx context.Context) error { return nil }

type stubMetricsReporter struct {
	sample stream.MetricsSample
}

func (s *stubMetricsReporter) Snapshot(sessionID string) (stream.MetricsSample, bool) {
	if s.sample == nil {
		return nil, false
	}
	if s.sample.SessionID() != sessionID {
		return nil, false
	}
	return s.sample, true
}

func (s *stubMetricsReporter) All() []stream.MetricsSample {
	if s.sample == nil {
		return nil
	}
	return []stream.MetricsSample{s.sample}
}
