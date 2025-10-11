package api

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
	apiwsrouting "github.com/coachpo/meltica/market_data/framework/api/wsrouting"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

const handlersMetadataKey = "handlers"

// SessionHandler exposes lifecycle endpoints for streaming sessions.
type SessionHandler struct {
	manager *apiwsrouting.Manager
}

// NewSessionHandler constructs a SessionHandler bound to the provided manager.
func NewSessionHandler(manager *apiwsrouting.Manager) *SessionHandler {
	if manager == nil {
		return nil
	}
	return &SessionHandler{manager: manager}
}

// Register wires the handler into the router.
func (h *SessionHandler) Register(router *Router) {
	if h == nil || router == nil || h.manager == nil {
		return
	}
	router.Handle(http.MethodPost, "/v1/stream/sessions", h.create)
}

func (h *SessionHandler) create(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	if h == nil || h.manager == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session handler not configured"))
	}
	defer req.Body.Close()
	payload, err := decodeSessionRequest(req.Body)
	if err != nil {
		return err
	}
	normalized, err := normalizeSessionRequest(payload)
	if err != nil {
		return err
	}
	result, err := h.manager.CreateSession(ctx, toSessionConfig(normalized))
	if err != nil {
		return err
	}
	return encodeSessionResponse(w, result.Stream)
}

type rawSessionRequest struct {
	Endpoint         string              `json:"endpoint"`
	Protocols        []string            `json:"protocols"`
	AuthToken        string              `json:"authToken"`
	Handlers         []rawHandlerBinding `json:"handlers"`
	InvalidThreshold *uint32             `json:"invalidThreshold"`
}

type rawHandlerBinding struct {
	HandlerName    string   `json:"handlerName"`
	Channels       []string `json:"channels"`
	MaxConcurrency int      `json:"maxConcurrency"`
}

type sessionRequest struct {
	Endpoint         string
	Protocols        []string
	AuthToken        string
	Handlers         []rawHandlerBinding
	InvalidThreshold uint32
	Metadata         map[string]string
	Bindings         []handler.Binding
}

func decodeSessionRequest(body io.Reader) (rawSessionRequest, error) {
	var payload rawSessionRequest
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return rawSessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid session payload"), errs.WithCause(err))
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return rawSessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid trailing data"))
	}
	return payload, nil
}

func normalizeSessionRequest(payload rawSessionRequest) (sessionRequest, error) {
	endpoint := strings.TrimSpace(payload.Endpoint)
	if endpoint == "" {
		return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("endpoint is required"))
	}
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid endpoint"), errs.WithCause(err))
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "ws" && scheme != "wss" {
		return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("endpoint must use ws or wss scheme"))
	}
	if parsed.Host == "" {
		return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("endpoint host is required"))
	}
	protocols := make([]string, 0, len(payload.Protocols))
	for _, proto := range payload.Protocols {
		p := strings.TrimSpace(proto)
		if p == "" {
			return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("protocol entries cannot be empty"))
		}
		protocols = append(protocols, strings.ToUpper(p))
	}
	if len(payload.Handlers) == 0 {
		return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("at least one handler binding is required"))
	}
	sanitizedHandlers := make([]rawHandlerBinding, 0, len(payload.Handlers))
	bindings := make([]handler.Binding, 0, len(payload.Handlers))
	for _, binding := range payload.Handlers {
		name := strings.TrimSpace(binding.HandlerName)
		if name == "" {
			return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handlerName is required"))
		}
		if len(binding.Channels) == 0 {
			return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("handler channels are required"))
		}
		channels := make([]string, 0, len(binding.Channels))
		for _, ch := range binding.Channels {
			trimmed := strings.TrimSpace(ch)
			if trimmed == "" {
				return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("channel identifiers cannot be empty"))
			}
			channels = append(channels, strings.ToUpper(trimmed))
		}
		maxConcurrency := binding.MaxConcurrency
		if maxConcurrency < 0 {
			return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("maxConcurrency must be non-negative"))
		}
		if maxConcurrency == 0 {
			maxConcurrency = 1
		}
		sanitizedHandlers = append(sanitizedHandlers, rawHandlerBinding{
			HandlerName:    name,
			Channels:       channels,
			MaxConcurrency: maxConcurrency,
		})
		bindings = append(bindings, handler.Binding{
			Name:           name,
			Channels:       channels,
			MaxConcurrency: maxConcurrency,
		})
	}
	threshold := uint32(0)
	if payload.InvalidThreshold != nil {
		threshold = *payload.InvalidThreshold
	}
	metadata := make(map[string]string)
	if len(sanitizedHandlers) > 0 {
		encoded, err := json.Marshal(sanitizedHandlers)
		if err != nil {
			return sessionRequest{}, errs.New("", errs.CodeInvalid, errs.WithMessage("failed to encode handler bindings"), errs.WithCause(err))
		}
		metadata[handlersMetadataKey] = string(encoded)
	}
	return sessionRequest{
		Endpoint:         parsed.String(),
		Protocols:        protocols,
		AuthToken:        strings.TrimSpace(payload.AuthToken),
		Handlers:         sanitizedHandlers,
		InvalidThreshold: threshold,
		Metadata:         metadata,
		Bindings:         bindings,
	}, nil
}

func toSessionConfig(req sessionRequest) apiwsrouting.SessionConfig {
	bindingsCopy := make([]handler.Binding, 0, len(req.Bindings))
	for _, binding := range req.Bindings {
		channels := make([]string, len(binding.Channels))
		copy(channels, binding.Channels)
		bindingsCopy = append(bindingsCopy, handler.Binding{
			Name:           binding.Name,
			Channels:       channels,
			MaxConcurrency: binding.MaxConcurrency,
		})
	}
	metadataCopy := make(map[string]string, len(req.Metadata))
	for k, v := range req.Metadata {
		metadataCopy[k] = v
	}
	protocolsCopy := make([]string, len(req.Protocols))
	copy(protocolsCopy, req.Protocols)
	return apiwsrouting.SessionConfig{
		Endpoint:         req.Endpoint,
		Protocols:        protocolsCopy,
		AuthToken:        req.AuthToken,
		InvalidThreshold: req.InvalidThreshold,
		Metadata:         metadataCopy,
		Bindings:         bindingsCopy,
	}
}

func encodeSessionResponse(w http.ResponseWriter, sess stream.Session) error {
	if sess == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("session unavailable"))
	}
	base := map[string]any{
		"sessionId":    sess.ID(),
		"status":       string(sess.Status()),
		"connectedAt":  time.Now().UTC(),
		"invalidCount": 0,
	}
	if concrete, ok := sess.(*framework.ConnectionSession); ok {
		base["connectedAt"] = concrete.ConnectedAt
		base["invalidCount"] = concrete.InvalidCount
		if len(concrete.Protocols) > 0 {
			base["protocols"] = concrete.Protocols
		}
		if snap := concrete.Throughput; snap != nil {
			window := snap.Window()
			base["throughput"] = map[string]any{
				"windowSeconds": int64(window / time.Second),
				"messages":      snap.Messages(),
				"errors":        snap.Errors(),
				"p50LatencyMs":  float64(snap.P50Latency()) / float64(time.Millisecond),
				"p95LatencyMs":  float64(snap.P95Latency()) / float64(time.Millisecond),
				"allocBytes":    snap.AllocBytes(),
			}
		}
	}
	writeJSON(w, http.StatusCreated, base)
	return nil
}
