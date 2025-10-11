package api

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/errs"
	apiwsrouting "github.com/coachpo/meltica/market_data/framework/api/wsrouting"
)

// HandlerFunc represents a request handler returning an error for centralized encoding.
type HandlerFunc func(context.Context, http.ResponseWriter, *http.Request) error

// Middleware wraps an http.Handler with cross-cutting behavior.
type Middleware func(http.Handler) http.Handler

// ErrorEncoder converts handler errors into HTTP responses.
type ErrorEncoder func(context.Context, http.ResponseWriter, error)

// RouterConfig configures router behavior.
type RouterConfig struct {
	Middlewares    []Middleware
	ErrorEncoder   ErrorEncoder
	SessionManager *apiwsrouting.Manager
}

type route struct {
	pattern    string
	segments   []string
	paramNames []string
	methods    map[string]http.Handler
}

// Router is a minimal HTTP router with centralized error handling.
type Router struct {
	routes       map[string]*route
	paramRoutes  []*route
	middlewares  []Middleware
	errorEncoder ErrorEncoder
	mu           sync.RWMutex
}

// NewRouter constructs a Router using the provided configuration.
func NewRouter(cfg RouterConfig) *Router {
	r := &Router{
		routes:      make(map[string]*route),
		paramRoutes: make([]*route, 0),
		middlewares: append([]Middleware(nil), cfg.Middlewares...),
	}
	if cfg.ErrorEncoder != nil {
		r.errorEncoder = cfg.ErrorEncoder
	} else {
		r.errorEncoder = defaultErrorEncoder
	}
	if cfg.SessionManager != nil {
		if handler := NewSessionHandler(cfg.SessionManager); handler != nil {
			handler.Register(r)
		}
	}
	return r
}

// Use appends runtime middleware.
func (r *Router) Use(m Middleware) {
	if m == nil {
		return
	}
	r.mu.Lock()
	r.middlewares = append(r.middlewares, m)
	r.mu.Unlock()
}

// Handle registers a handler for the given method and pattern.
func (r *Router) Handle(method, pattern string, handler HandlerFunc) {
	if handler == nil {
		panic("api: nil handler")
	}
	m := strings.ToUpper(strings.TrimSpace(method))
	if m == "" {
		panic("api: empty method")
	}
	p := strings.TrimSpace(pattern)
	if p == "" {
		panic("api: empty pattern")
	}
	wrapped := r.wrap(handler)
	r.mu.Lock()
	defer r.mu.Unlock()
	segments, params := parsePattern(p)
	if len(params) == 0 {
		rt, ok := r.routes[p]
		if !ok {
			rt = &route{
				pattern:  p,
				segments: segments,
				methods:  make(map[string]http.Handler),
			}
			r.routes[p] = rt
		}
		rt.methods[m] = wrapped
		return
	}
	rt := r.findParamRoute(p)
	if rt == nil {
		rt = &route{
			pattern:    p,
			segments:   segments,
			paramNames: params,
			methods:    make(map[string]http.Handler),
		}
		r.paramRoutes = append(r.paramRoutes, rt)
	}
	rt.methods[m] = wrapped
}

func parsePattern(pattern string) ([]string, []string) {
	trimmed := strings.Trim(pattern, "/")
	if trimmed == "" {
		return nil, nil
	}
	parts := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(parts))
	params := make([]string, 0)
	for _, part := range parts {
		segments = append(segments, part)
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			params = append(params, part[1:])
		}
	}
	return segments, params
}

func (r *Router) findParamRoute(pattern string) *route {
	for _, rt := range r.paramRoutes {
		if rt.pattern == pattern {
			return rt
		}
	}
	return nil
}

// ServeHTTP dispatches requests to registered handlers.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	path := req.URL.Path
	r.mu.RLock()
	rt, ok := r.routes[path]
	r.mu.RUnlock()
	if !ok {
		handler, params, allowed := r.matchParameterized(path, req.Method)
		if handler != nil {
			ctx := context.WithValue(req.Context(), paramsKey{}, params)
			handler.ServeHTTP(w, req.WithContext(ctx))
			return
		}
		if len(allowed) > 0 {
			w.Header().Set("Allow", strings.Join(allowed, ", "))
			r.writeStatus(w, http.StatusMethodNotAllowed, "method not allowed", nil)
			return
		}
		r.writeStatus(w, http.StatusNotFound, "resource not found", nil)
		return
	}
	method := strings.ToUpper(req.Method)
	if handler, ok := rt.methods[method]; ok {
		handler.ServeHTTP(w, req)
		return
	}
	allowed := rt.allowed()
	if method == http.MethodOptions {
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		w.WriteHeader(http.StatusNoContent)
		return
	}
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	r.writeStatus(w, http.StatusMethodNotAllowed, "method not allowed", nil)
}

type paramsKey struct{}

func (r *Router) matchParameterized(path, method string) (http.Handler, map[string]string, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, rt := range r.paramRoutes {
		params, ok := matchRoute(rt, path)
		if !ok {
			continue
		}
		handler, ok := rt.methods[strings.ToUpper(method)]
		if ok {
			return handler, params, nil
		}
		return nil, nil, rt.allowed()
	}
	return nil, nil, nil
}

// Handler returns the router as an http.Handler.
func (r *Router) Handler() http.Handler {
	return r
}

func (r *Router) wrap(handler HandlerFunc) http.Handler {
	base := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		err := handler(req.Context(), w, req)
		if err != nil {
			r.errorEncoder(req.Context(), w, err)
		}
	})
	var wrapped http.Handler = base
	r.mu.RLock()
	middlewares := append([]Middleware(nil), r.middlewares...)
	r.mu.RUnlock()
	for i := len(middlewares) - 1; i >= 0; i-- {
		wrapped = middlewares[i](wrapped)
	}
	return wrapped
}

func (rt *route) allowed() []string {
	methods := make([]string, 0, len(rt.methods))
	for method := range rt.methods {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	return methods
}

func matchRoute(rt *route, path string) (map[string]string, bool) {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) == 1 && segments[0] == "" {
		segments = nil
	}
	if len(segments) != len(rt.segments) {
		return nil, false
	}
	params := make(map[string]string)
	for idx, segment := range rt.segments {
		if strings.HasPrefix(segment, ":") {
			name := segment[1:]
			params[name] = segments[idx]
			continue
		}
		if segment != segments[idx] {
			return nil, false
		}
	}
	return params, true
}

// Param retrieves a route parameter from the request context.
func Param(req *http.Request, key string) (string, bool) {
	if req == nil {
		return "", false
	}
	params, _ := req.Context().Value(paramsKey{}).(map[string]string)
	value, ok := params[key]
	return value, ok
}

func defaultErrorEncoder(ctx context.Context, w http.ResponseWriter, err error) {
	status, payload := mapError(err)
	writeJSON(w, status, payload)
}

func mapError(err error) (int, map[string]any) {
	var e *errs.E
	if errors.As(err, &e) {
		status := statusFromCode(e.Code)
		if e.HTTP > 0 {
			status = e.HTTP
		}
		message := strings.TrimSpace(e.Message)
		if message == "" {
			message = "request failed"
		}
		payload := map[string]any{
			"code":    string(e.Code),
			"message": message,
		}
		if e.Canonical != "" {
			payload["canonical"] = string(e.Canonical)
		}
		if len(e.VenueMetadata) > 0 {
			payload["metadata"] = e.VenueMetadata
		}
		return status, payload
	}
	message := strings.TrimSpace(err.Error())
	if message == "" {
		message = "internal server error"
	}
	return http.StatusInternalServerError, map[string]any{
		"code":    "internal_error",
		"message": message,
	}
}

func statusFromCode(code errs.Code) int {
	switch code {
	case errs.CodeInvalid:
		return http.StatusBadRequest
	case errs.CodeAuth:
		return http.StatusUnauthorized
	case errs.CodeRateLimited:
		return http.StatusConflict
	case errs.CodeNetwork:
		return http.StatusServiceUnavailable
	default:
		return http.StatusInternalServerError
	}
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	data, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (r *Router) writeStatus(w http.ResponseWriter, status int, msg string, meta map[string]any) {
	payload := map[string]any{
		"code":    "invalid_request",
		"message": msg,
	}
	for k, v := range meta {
		payload[k] = v
	}
	writeJSON(w, status, payload)
}

var _ http.Handler = (*Router)(nil)
