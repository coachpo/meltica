package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

var allowedOutcomeTypes = map[string]stream.OutcomeType{
	"Ack":       stream.OutcomeAck,
	"Transform": stream.OutcomeTransform,
	"Error":     stream.OutcomeError,
	"Drop":      stream.OutcomeDrop,
}

type handlerRegistry interface {
	Resolve(name, version string) (handler.Registration, bool)
	SetActive(name, version string, active bool) error
}

// HandlersHandler coordinates HTTP endpoints for handler lifecycle management.
type HandlersHandler struct {
	engine   stream.Engine
	registry handlerRegistry
	mu       sync.Mutex
	records  map[string]handlerRecord
}

type handlerRecord struct {
	Name         string
	Version      string
	OutcomeTypes []stream.OutcomeType
	Description  string
	Options      map[string]string
	RegisteredAt time.Time
}

// NewHandlersHandler constructs a HandlersHandler bound to the provided engine and registry.
func NewHandlersHandler(engine stream.Engine, registry handlerRegistry) *HandlersHandler {
	return &HandlersHandler{
		engine:   engine,
		registry: registry,
		records:  make(map[string]handlerRecord),
	}
}

// Register wires the handler endpoint into the router.
func (h *HandlersHandler) Register(router *Router) {
	if h == nil || router == nil {
		return
	}
	router.Handle(http.MethodPost, "/v1/stream/handlers", h.create)
}

func (h *HandlersHandler) create(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
	if h == nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("handler registration unavailable"))
	}
	defer req.Body.Close()
	payload, err := decodeHandlerRegistration(req.Body)
	if err != nil {
		return err
	}
	sanitized, err := h.normalize(payload)
	if err != nil {
		return err
	}
	if h.registry != nil {
		if _, ok := h.registry.Resolve(sanitized.Name, sanitized.Version); !ok {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("handler version not registered"))
		}
		if err := h.registry.SetActive(sanitized.Name, sanitized.Version, true); err != nil {
			return err
		}
	}
	h.store(sanitized)
	return encodeHandlerResponse(w, sanitized)
}

func (h *HandlersHandler) normalize(payload rawHandlerRegistration) (handlerRecord, error) {
	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return handlerRecord{}, errs.New("", errs.CodeInvalid, errs.WithMessage("name is required"))
	}
	version := strings.TrimSpace(payload.Version)
	if version == "" {
		return handlerRecord{}, errs.New("", errs.CodeInvalid, errs.WithMessage("version is required"))
	}
	if len(payload.OutcomeTypes) == 0 {
		return handlerRecord{}, errs.New("", errs.CodeInvalid, errs.WithMessage("at least one outcome type is required"))
	}
	unique := make(map[stream.OutcomeType]struct{}, len(payload.OutcomeTypes))
	outcomes := make([]stream.OutcomeType, 0, len(payload.OutcomeTypes))
	for _, raw := range payload.OutcomeTypes {
		trimmed := strings.TrimSpace(raw)
		mapped, ok := allowedOutcomeTypes[trimmed]
		if !ok {
			return handlerRecord{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid outcome type"))
		}
		if _, exists := unique[mapped]; exists {
			continue
		}
		unique[mapped] = struct{}{}
		outcomes = append(outcomes, mapped)
	}
	description := strings.TrimSpace(payload.Description)
	options := sanitizeOptions(payload.Options)
	return handlerRecord{
		Name:         name,
		Version:      version,
		OutcomeTypes: outcomes,
		Description:  description,
		Options:      options,
		RegisteredAt: time.Now().UTC(),
	}, nil
}

func (h *HandlersHandler) store(record handlerRecord) {
	h.mu.Lock()
	defer h.mu.Unlock()
	key := recordKey(record.Name, record.Version)
	h.records[key] = record
}

func recordKey(name, version string) string {
	return name + ":" + version
}

type rawHandlerRegistration struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	OutcomeTypes []string          `json:"outcomeTypes"`
	Description  string            `json:"description"`
	Options      map[string]string `json:"options"`
}

func decodeHandlerRegistration(body io.Reader) (rawHandlerRegistration, error) {
	var payload rawHandlerRegistration
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return rawHandlerRegistration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid handler payload"), errs.WithCause(err))
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return rawHandlerRegistration{}, errs.New("", errs.CodeInvalid, errs.WithMessage("invalid trailing data"))
	}
	return payload, nil
}

func sanitizeOptions(opts map[string]string) map[string]string {
	if len(opts) == 0 {
		return nil
	}
	clean := make(map[string]string, len(opts))
	for key, value := range opts {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			continue
		}
		clean[trimmedKey] = strings.TrimSpace(value)
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func encodeHandlerResponse(w http.ResponseWriter, record handlerRecord) error {
	payload := map[string]any{
		"name":         record.Name,
		"version":      record.Version,
		"outcomeTypes": toOutcomeStrings(record.OutcomeTypes),
		"registeredAt": record.RegisteredAt,
		"status":       "active",
	}
	if record.Description != "" {
		payload["description"] = record.Description
	}
	if len(record.Options) > 0 {
		payload["options"] = record.Options
	}
	writeJSON(w, http.StatusCreated, payload)
	return nil
}

func toOutcomeStrings(values []stream.OutcomeType) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}
