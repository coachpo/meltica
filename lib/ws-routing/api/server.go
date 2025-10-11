package api

import (
	"net/http"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/lib/ws-routing/internal"
)

// Subscription represents an active routing subscription exposed via the admin API.
type Subscription struct {
	Symbol string `json:"symbol"`
	Stream string `json:"stream"`
}

// Status mirrors the session lifecycle statuses.
type Status = internal.Status

const (
	StatusStarting Status = internal.StatusStarting
	StatusRunning  Status = internal.StatusRunning
	StatusStopped  Status = internal.StatusStopped
)

// Inspector exposes session details required by the admin API.
type Inspector interface {
	Status() Status
	Subscriptions() []Subscription
}

// New constructs an HTTP handler implementing the admin API contract.
func New(inspector Inspector) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/admin/ws-routing/v1/health", healthHandler(inspector))
	mux.Handle("/admin/ws-routing/v1/subscriptions", subscriptionsHandler(inspector))
	return mux
}

func healthHandler(inspector Inspector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": mapStatus(inspector.Status())})
	})
}

func subscriptionsHandler(inspector Inspector) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		items := inspector.Subscriptions()
		copyOf := make([]Subscription, len(items))
		copy(copyOf, items)
		writeJSON(w, http.StatusOK, map[string]any{"items": copyOf})
	})
}

func mapStatus(status Status) string {
	if status == internal.StatusRunning {
		return "ok"
	}
	return string(status)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	buffer, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(buffer)
}
