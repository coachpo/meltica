package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

func TestHandlersHandlerRegistersHandler(t *testing.T) {
	stub := &stubRegistry{
		resolveFn: func(name, version string) (handler.Registration, bool) {
			return handler.Registration{Name: name, Version: version, Active: true}, true
		},
		setActiveFn: func(name, version string, active bool) error {
			require.Equal(t, "quotes", name)
			require.Equal(t, "1.0.0", version)
			require.True(t, active)
			return nil
		},
	}
	handler := NewHandlersHandler(nil, stub)
	router := NewRouter(RouterConfig{})
	handler.Register(router)
	payload := map[string]any{
		"name":         "quotes ",
		"version":      " 1.0.0",
		"outcomeTypes": []string{"Ack", "Error", "Ack"},
		"description":  "  spot quotes  ",
		"options": map[string]string{
			" team ": "  md  ",
		},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/stream/handlers", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusCreated, resp.Code)
	var response map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &response))
	require.Equal(t, "quotes", response["name"])
	require.Equal(t, "1.0.0", response["version"])
	require.Contains(t, response, "registeredAt")
	require.Equal(t, []any{"Ack", "Error"}, response["outcomeTypes"])
	require.Equal(t, "spot quotes", response["description"])
	options, ok := response["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "md", options["team"])
	require.Equal(t, "active", response["status"])
	require.Len(t, handler.records, 1)
}

func TestHandlersHandlerRejectsInvalidOutcomeType(t *testing.T) {
	handler := NewHandlersHandler(nil, nil)
	router := NewRouter(RouterConfig{})
	handler.Register(router)
	payload := map[string]any{
		"name":         "quotes",
		"version":      "1.0.0",
		"outcomeTypes": []string{"Ack", "Unknown"},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/stream/handlers", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

func TestHandlersHandlerRejectsUnknownHandler(t *testing.T) {
	stub := &stubRegistry{
		resolveFn: func(name, version string) (handler.Registration, bool) {
			return handler.Registration{}, false
		},
		setActiveFn: func(name, version string, active bool) error {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("should not be called"))
		},
	}
	handler := NewHandlersHandler(nil, stub)
	router := NewRouter(RouterConfig{})
	handler.Register(router)
	payload := map[string]any{
		"name":         "quotes",
		"version":      "1.0.0",
		"outcomeTypes": []string{"Ack"},
	}
	body, err := json.Marshal(payload)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/v1/stream/handlers", bytes.NewReader(body))
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusBadRequest, resp.Code)
}

type stubRegistry struct {
	resolveFn   func(name, version string) (handler.Registration, bool)
	setActiveFn func(name, version string, active bool) error
}

func (s *stubRegistry) Resolve(name, version string) (handler.Registration, bool) {
	return s.resolveFn(name, version)
}

func (s *stubRegistry) SetActive(name, version string, active bool) error {
	return s.setActiveFn(name, version, active)
}

func TestToOutcomeStringsStableOrder(t *testing.T) {
	values := []stream.OutcomeType{stream.OutcomeAck, stream.OutcomeError}
	result := toOutcomeStrings(values)
	require.Equal(t, []string{"Ack", "Error"}, result)
}

func TestEncodeHandlerResponseOmitsEmptyFields(t *testing.T) {
	record := handlerRecord{
		Name:         "quotes",
		Version:      "1.0.0",
		OutcomeTypes: []stream.OutcomeType{stream.OutcomeAck},
		RegisteredAt: time.Unix(0, 0).UTC(),
	}
	recorder := httptest.NewRecorder()
	require.NoError(t, encodeHandlerResponse(recorder, record))
	require.Equal(t, http.StatusCreated, recorder.Code)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))
	_, hasDescription := payload["description"]
	require.False(t, hasDescription)
	_, hasOptions := payload["options"]
	require.False(t, hasOptions)
}
