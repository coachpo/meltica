package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
)

// MetricsHandler exposes endpoints for session telemetry data.
type MetricsHandler struct {
	engine stream.Engine
}

// NewMetricsHandler constructs a MetricsHandler bound to the provided engine.
func NewMetricsHandler(engine stream.Engine) *MetricsHandler {
	if engine == nil {
		return nil
	}
	return &MetricsHandler{engine: engine}
}

// Register wires the metrics endpoint into the router.
func (h *MetricsHandler) Register(router *Router) {
	if h == nil || router == nil {
		return
	}
	router.Handle(http.MethodGet, "/v1/stream/sessions/:sessionId/metrics", h.get)
}

func (h *MetricsHandler) get(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	if h == nil || h.engine == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("metrics handler not configured"))
	}
	sessionID, ok := Param(req, "sessionId")
	if !ok {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("sessionId is required"))
	}
	id := strings.TrimSpace(sessionID)
	if id == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("sessionId is required"))
	}
	reporter := h.engine.Metrics()
	if reporter == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("metrics reporter unavailable"))
	}
	sample, ok := reporter.Snapshot(id)
	if !ok {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session not found"), errs.WithHTTP(http.StatusNotFound))
	}
	payload := map[string]any{
		"windowSeconds": int64(sample.Window() / time.Second),
		"messages":      sample.Messages(),
		"errors":        sample.Errors(),
		"p50LatencyMs":  durationMillis(sample.P50Latency()),
		"p95LatencyMs":  durationMillis(sample.P95Latency()),
		"allocBytes":    sample.AllocBytes(),
	}
	writeJSON(w, http.StatusOK, payload)
	return nil
}

func durationMillis(d time.Duration) float64 {
	if d <= 0 {
		return 0
	}
	return float64(d) / float64(time.Millisecond)
}
