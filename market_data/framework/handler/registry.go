package handler

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
)

// Registration captures sanitized metadata for an active handler version.
type Registration struct {
	Name         string
	Version      string
	Channels     []string
	Factory      stream.HandlerFactory
	Middleware   []stream.Middleware
	Description  string
	Active       bool
	RegisteredAt time.Time
}

// Instantiate constructs a handler instance wrapped with middleware.
func (r Registration) Instantiate() stream.Handler {
	if r.Factory == nil {
		return nil
	}
	base := r.Factory()
	if base == nil {
		return nil
	}
	wrapped := base
	for i := len(r.Middleware) - 1; i >= 0; i-- {
		mw := r.Middleware[i]
		if mw == nil {
			continue
		}
		wrapped = mw(wrapped)
	}
	return wrapped
}

type registrationRecord struct {
	registration Registration
}

func (r registrationRecord) clone() Registration {
	clone := r.registration
	clone.Channels = append([]string(nil), r.registration.Channels...)
	clone.Middleware = append([]stream.Middleware(nil), r.registration.Middleware...)
	return clone
}

// Registry manages handler registrations and their lifecycle by version.
type Registry struct {
	mu     sync.RWMutex
	items  map[string]map[string]*registrationRecord
	order  map[string][]string
	latest map[string]string
}

// NewRegistry constructs an empty handler registry.
func NewRegistry() *Registry {
	return &Registry{
		items:  make(map[string]map[string]*registrationRecord),
		order:  make(map[string][]string),
		latest: make(map[string]string),
	}
}

// Register stores a new handler version. Duplicate name/version pairs are rejected.
func (r *Registry) Register(in stream.HandlerRegistration) (Registration, error) {
	entry, err := sanitizeRegistration(in)
	if err != nil {
		return Registration{}, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket, ok := r.items[entry.Name]
	if !ok {
		bucket = make(map[string]*registrationRecord)
		r.items[entry.Name] = bucket
	}
	if _, exists := bucket[entry.Version]; exists {
		return Registration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handler version already registered"))
	}
	record := &registrationRecord{registration: entry}
	bucket[entry.Version] = record
	r.order[entry.Name] = append(r.order[entry.Name], entry.Version)
	r.refreshLatestLocked(entry.Name)
	return record.clone(), nil
}

// Resolve looks up an active registration by name and version. When version is empty, the latest active version is returned.
func (r *Registry) Resolve(name, version string) (Registration, bool) {
	key := strings.TrimSpace(name)
	if key == "" {
		return Registration{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	bucket, ok := r.items[key]
	if !ok {
		return Registration{}, false
	}
	ver := strings.TrimSpace(version)
	if ver == "" {
		ver = r.latest[key]
	}
	record, ok := bucket[ver]
	if !ok || !record.registration.Active {
		return Registration{}, false
	}
	return record.clone(), true
}

// SetActive toggles the availability of a registered handler version.
func (r *Registry) SetActive(name, version string, active bool) error {
	key := strings.TrimSpace(name)
	ver := strings.TrimSpace(version)
	if key == "" || ver == "" {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("handler name and version required"))
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	bucket, ok := r.items[key]
	if !ok {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("handler not registered"))
	}
	record, ok := bucket[ver]
	if !ok {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("handler version not registered"))
	}
	record.registration.Active = active
	r.refreshLatestLocked(key)
	return nil
}

// Versions returns the known versions for a handler name, sorted by registration order.
func (r *Registry) Versions(name string) []string {
	key := strings.TrimSpace(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	versions := append([]string(nil), r.order[key]...)
	return versions
}

// LatestVersion returns the most recently active version for the handler.
func (r *Registry) LatestVersion(name string) (string, bool) {
	key := strings.TrimSpace(name)
	r.mu.RLock()
	defer r.mu.RUnlock()
	ver, ok := r.latest[key]
	return ver, ok
}

// Active returns a snapshot of all active registrations.
func (r *Registry) Active() []Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var results []Registration
	for _, bucket := range r.items {
		for _, record := range bucket {
			if !record.registration.Active {
				continue
			}
			results = append(results, record.clone())
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Name == results[j].Name {
			return results[i].RegisteredAt.Before(results[j].RegisteredAt)
		}
		return results[i].Name < results[j].Name
	})
	return results
}

// All returns a snapshot of every registration regardless of lifecycle state.
func (r *Registry) All() []Registration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var results []Registration
	for _, bucket := range r.items {
		for _, record := range bucket {
			results = append(results, record.clone())
		}
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Name == results[j].Name {
			if results[i].RegisteredAt.Equal(results[j].RegisteredAt) {
				return results[i].Version < results[j].Version
			}
			return results[i].RegisteredAt.Before(results[j].RegisteredAt)
		}
		return results[i].Name < results[j].Name
	})
	return results
}

func (r *Registry) refreshLatestLocked(name string) {
	versions := r.order[name]
	bucket := r.items[name]
	for i := len(versions) - 1; i >= 0; i-- {
		ver := versions[i]
		record := bucket[ver]
		if record != nil && record.registration.Active {
			r.latest[name] = ver
			return
		}
	}
	delete(r.latest, name)
}

func sanitizeRegistration(in stream.HandlerRegistration) (Registration, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return Registration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handler name is required"))
	}
	version := strings.TrimSpace(in.Version)
	if version == "" {
		return Registration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handler version is required"))
	}
	if in.Factory == nil {
		return Registration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handler factory is required"))
	}
	channels := sanitizeChannels(in.Channels)
	if len(channels) == 0 {
		return Registration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("at least one channel is required"))
	}
	middlewares := sanitizeMiddlewares(in.Middleware)
	desc := strings.TrimSpace(in.Description)
	return Registration{
		Name:         name,
		Version:      version,
		Channels:     channels,
		Factory:      in.Factory,
		Middleware:   middlewares,
		Description:  desc,
		Active:       true,
		RegisteredAt: time.Now().UTC(),
	}, nil
}

func sanitizeChannels(channels []string) []string {
	if len(channels) == 0 {
		return nil
	}
	result := make([]string, 0, len(channels))
	seen := make(map[string]struct{}, len(channels))
	for _, channel := range channels {
		trimmed := strings.TrimSpace(channel)
		if trimmed == "" {
			return nil
		}
		upper := strings.ToUpper(trimmed)
		if _, ok := seen[upper]; ok {
			continue
		}
		seen[upper] = struct{}{}
		result = append(result, upper)
	}
	return result
}

func sanitizeMiddlewares(mw []stream.Middleware) []stream.Middleware {
	if len(mw) == 0 {
		return nil
	}
	result := make([]stream.Middleware, 0, len(mw))
	for _, middleware := range mw {
		if middleware == nil {
			continue
		}
		result = append(result, middleware)
	}
	return result
}
