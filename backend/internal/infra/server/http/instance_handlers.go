package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/app/lambda/runtime"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/infra/config"
)

func (s *httpServer) listInstances(w http.ResponseWriter, _ *http.Request) {
	instances := s.manager.Instances()
	responses := make([]instanceSummaryResponse, 0, len(instances))
	for _, summary := range instances {
		responses = append(responses, instanceSummaryResponse{
			InstanceSummary: summary,
			Links:           s.buildInstanceLinksFromSummary(summary),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"instances": responses})
}

func (s *httpServer) createInstance(w http.ResponseWriter, r *http.Request) {
	limitRequestBody(w, r)
	spec, err := decodeInstanceSpec(r)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	if _, err := s.manager.Create(spec); err != nil {
		s.writeManagerError(w, err)
		return
	}
	snapshot, _ := s.manager.Instance(spec.ID)
	response := instanceSnapshotResponse{
		InstanceSnapshot: snapshot,
		Links:            s.buildInstanceLinksFromSnapshot(snapshot),
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *httpServer) handleInstance(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, instanceDetailPrefix), "/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "instance id required")
		return
	}

	id, action, hasAction := strings.Cut(rest, "/")
	id = strings.TrimSpace(id)
	if id == "" {
		writeError(w, http.StatusNotFound, "instance id required")
		return
	}

	if !hasAction {
		s.handleInstanceResource(w, r, id)
		return
	}

	action = strings.TrimSpace(action)
	s.handleInstanceAction(w, r, id, action)
}

func (s *httpServer) handleInstanceResource(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		snapshot, ok := s.manager.Instance(id)
		if !ok {
			writeError(w, http.StatusNotFound, "strategy instance not found")
			return
		}
		response := instanceSnapshotResponse{
			InstanceSnapshot: snapshot,
			Links:            s.buildInstanceLinksFromSnapshot(snapshot),
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodPut:
		limitRequestBody(w, r)
		spec, err := decodeInstanceSpec(r)
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		if spec.ID != "" && spec.ID != id {
			writeError(w, http.StatusBadRequest, "instance id mismatch")
			return
		}
		spec.ID = id
		if err := s.manager.Update(r.Context(), spec); err != nil {
			s.writeManagerError(w, err)
			return
		}
		snapshot, _ := s.manager.Instance(id)
		response := instanceSnapshotResponse{
			InstanceSnapshot: snapshot,
			Links:            s.buildInstanceLinksFromSnapshot(snapshot),
		}
		writeJSON(w, http.StatusOK, response)
	case http.MethodDelete:
		if err := s.manager.Remove(id); err != nil {
			s.writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "id": id})
	default:
		methodNotAllowed(w, http.MethodDelete, http.MethodGet, http.MethodPut)
	}
}

func (s *httpServer) handleInstanceAction(w http.ResponseWriter, r *http.Request, id, action string) {
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if s.manager == nil {
			writeError(w, http.StatusServiceUnavailable, "lambda manager unavailable")
			return
		}
		if err := s.manager.Start(r.Context(), id); err != nil {
			s.writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id, "action": action})
	case "stop":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if s.manager == nil {
			writeError(w, http.StatusServiceUnavailable, "lambda manager unavailable")
			return
		}
		if err := s.manager.Stop(id); err != nil {
			s.writeManagerError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "id": id, "action": action})
	case instanceOrdersSuffix:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleInstanceOrders(w, r, id)
	case instanceExecutionsSuffix:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleInstanceExecutions(w, r, id)
	default:
		writeError(w, http.StatusNotFound, "unsupported action")
	}
}

func (s *httpServer) handleInstanceOrders(w http.ResponseWriter, r *http.Request, id string) {
	if s.orderStore == nil {
		writeError(w, http.StatusServiceUnavailable, "order store unavailable")
		return
	}
	values := r.URL.Query()
	limit, err := parseLimitParam(values.Get("limit"), defaultOrdersLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	states := values["state"]
	for i, state := range states {
		states[i] = strings.TrimSpace(state)
	}
	provider := strings.TrimSpace(values.Get("provider"))
	records, err := s.orderStore.ListOrders(r.Context(), orderstore.OrderQuery{
		StrategyInstance: id,
		Provider:         provider,
		States:           states,
		Limit:            limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := map[string]any{
		"orders": records,
		"count":  len(records),
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *httpServer) handleInstanceExecutions(w http.ResponseWriter, r *http.Request, id string) {
	if s.orderStore == nil {
		writeError(w, http.StatusServiceUnavailable, "order store unavailable")
		return
	}
	values := r.URL.Query()
	limit, err := parseLimitParam(values.Get("limit"), defaultExecutionsLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	provider := strings.TrimSpace(values.Get("provider"))
	orderID := strings.TrimSpace(values.Get("orderId"))
	records, err := s.orderStore.ListExecutions(r.Context(), orderstore.ExecutionQuery{
		StrategyInstance: id,
		Provider:         provider,
		OrderID:          orderID,
		Limit:            limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := map[string]any{
		"executions": records,
		"count":      len(records),
	}
	writeJSON(w, http.StatusOK, response)
}

func parseLimitParam(raw string, fallback int) (int, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid limit")
	}
	if value > maxListLimit {
		return maxListLimit, nil
	}
	return value, nil
}

func (s *httpServer) writeManagerError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, runtime.ErrInstanceExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, runtime.ErrInstanceAlreadyRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, runtime.ErrInstanceNotRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, runtime.ErrInstanceNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func decodeInstanceSpec(r *http.Request) (config.LambdaSpec, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	var spec config.LambdaSpec
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&spec); err != nil {
		return spec, fmt.Errorf("decode payload: %w", err)
	}
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Strategy.Normalize()
	if len(spec.ProviderSymbols) > 0 {
		symbolSets := make(map[string]config.ProviderSymbols, len(spec.ProviderSymbols))
		for name, symbolSpec := range spec.ProviderSymbols {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			symbolSpec.Normalize()
			symbolSets[trimmed] = symbolSpec
		}
		spec.ProviderSymbols = symbolSets
	}
	spec.RefreshProviders()
	if spec.ID == "" {
		return spec, fmt.Errorf("id required")
	}
	if spec.Strategy.Identifier == "" {
		return spec, fmt.Errorf("strategy required")
	}
	if len(spec.AllSymbols()) == 0 {
		return spec, fmt.Errorf("symbols required")
	}
	return spec, nil
}
