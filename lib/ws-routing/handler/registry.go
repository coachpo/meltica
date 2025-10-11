package handler

import (
	"context"
	"strings"
	"sync"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
)

// Handler processes an event for a specific routing type.
type Handler func(context.Context, internal.Event) error

// Registry stores handlers grouped by event type.
type Registry struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
}

// NewRegistry constructs an empty handler registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[string][]Handler)}
}

// Register associates the handler with the supplied event type.
func (r *Registry) Register(eventType string, h Handler) error {
	key := strings.TrimSpace(eventType)
	if key == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("event type required"))
	}
	if h == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("handler required"))
	}
	r.mu.Lock()
	r.handlers[key] = append(r.handlers[key], h)
	r.mu.Unlock()
	return nil
}

// HandlersFor returns handlers configured for the event type.
func (r *Registry) HandlersFor(eventType string) []Handler {
	key := strings.TrimSpace(eventType)
	r.mu.RLock()
	snapshot := append([]Handler(nil), r.handlers[key]...)
	r.mu.RUnlock()
	return snapshot
}

// HandlersSnapshot returns the current handler list without copying.
// Callers MUST treat the returned slice as read-only and avoid retaining it
// across mutations triggered by Register or Clear.
func (r *Registry) HandlersSnapshot(eventType string) []Handler {
	key := strings.TrimSpace(eventType)
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.handlers[key]
}

// Clear removes all handlers associated with an event type.
func (r *Registry) Clear(eventType string) {
	key := strings.TrimSpace(eventType)
	if key == "" {
		return
	}
	r.mu.Lock()
	delete(r.handlers, key)
	r.mu.Unlock()
}
