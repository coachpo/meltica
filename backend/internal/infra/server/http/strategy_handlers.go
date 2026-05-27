package httpserver

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/app/lambda/js"
	"github.com/coachpo/meltica/internal/app/lambda/runtime"
)

func (s *httpServer) getStrategies(w http.ResponseWriter, _ *http.Request) {
	catalog := s.manager.StrategyCatalog()
	writeJSON(w, http.StatusOK, map[string]any{"strategies": catalog})
}

func (s *httpServer) getStrategy(w http.ResponseWriter, r *http.Request) {
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, strategyDetailPrefix), "/")
	if name == "" {
		writeError(w, http.StatusNotFound, "strategy name required")
		return
	}
	meta, ok := s.manager.StrategyDetail(name)
	if !ok {
		writeError(w, http.StatusNotFound, "strategy not found")
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *httpServer) listStrategyModules(w http.ResponseWriter, r *http.Request) {
	modules := []js.ModuleSummary{}
	if s.manager != nil {
		modules = s.manager.StrategyModules()
	}
	strategyDirectory := ""
	if s.manager != nil {
		strategyDirectory = s.manager.StrategyDirectory()
	}
	values := r.URL.Query()
	if raw := values.Get("runningOnly"); raw != "" {
		writeError(w, http.StatusBadRequest, "runningOnly is no longer supported; use /strategies/modules/{selector}/usage")
		return
	}
	filtered, total, offset, limit, err := filterModuleSummaries(modules, values)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	for i := range filtered {
		filtered[i].Running = nil
	}
	response := map[string]any{
		"modules":           filtered,
		"total":             total,
		"offset":            offset,
		"strategyDirectory": strategyDirectory,
	}
	if limit >= 0 {
		response["limit"] = limit
	}
	writeJSON(w, http.StatusOK, response)
}

func filterModuleSummaries(modules []js.ModuleSummary, values url.Values) ([]js.ModuleSummary, int, int, int, error) {
	if len(modules) == 0 {
		return modules, 0, 0, -1, nil
	}

	strategyFilter := strings.TrimSpace(values.Get("strategy"))
	hashFilter := strings.TrimSpace(values.Get("hash"))

	limit := -1
	if raw := values.Get("limit"); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil || val < 0 {
			return nil, 0, 0, 0, fmt.Errorf("limit must be a non-negative integer")
		}
		limit = val
	}
	offset := 0
	if raw := values.Get("offset"); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil || val < 0 {
			return nil, 0, 0, 0, fmt.Errorf("offset must be a non-negative integer")
		}
		offset = val
	}

	filtered := make([]js.ModuleSummary, 0, len(modules))
	for _, module := range modules {
		if strategyFilter != "" && !strings.EqualFold(module.Name, strategyFilter) {
			continue
		}
		if filteredModule, include := applyModuleFilters(module, hashFilter); include {
			filtered = append(filtered, filteredModule)
		}
	}

	total := len(filtered)
	if total == 0 {
		return filtered, 0, offset, limit, nil
	}

	if offset > total {
		offset = total
	}
	end := total
	if limit >= 0 && offset+limit < end {
		end = offset + limit
	}
	paged := filtered[offset:end]
	return paged, total, offset, limit, nil
}

func applyModuleFilters(module js.ModuleSummary, hashFilter string) (js.ModuleSummary, bool) {
	filtered := module
	filtered.Revisions = filterModuleRevisions(module.Revisions, hashFilter)

	if strings.TrimSpace(hashFilter) != "" {
		if len(filtered.Revisions) == 0 {
			var empty js.ModuleSummary
			return empty, false
		}
	}
	return filtered, true
}

func filterModuleRevisions(revisions []js.ModuleRevision, hashFilter string) []js.ModuleRevision {
	if len(revisions) == 0 {
		return nil
	}
	normalized := strings.TrimSpace(hashFilter)
	out := make([]js.ModuleRevision, 0, len(revisions))
	for _, revision := range revisions {
		if normalized != "" && !strings.EqualFold(revision.Hash, normalized) {
			continue
		}
		revCopy := revision
		out = append(out, revCopy)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (s *httpServer) buildInstanceLinksFromSummary(summary runtime.InstanceSummary) instanceLinks {
	selector := buildUsageSelector(summary.StrategySelector, summary.StrategyIdentifier, summary.StrategyHash)
	return instanceLinks{
		Self:  instanceDetailPrefix + url.PathEscape(summary.ID),
		Usage: buildModuleUsageURL(selector),
	}
}

func (s *httpServer) buildInstanceLinksFromSnapshot(snapshot runtime.InstanceSnapshot) instanceLinks {
	selector := buildUsageSelector(snapshot.Strategy.Selector, snapshot.Strategy.Identifier, snapshot.Strategy.Hash)
	return instanceLinks{
		Self:  instanceDetailPrefix + url.PathEscape(snapshot.ID),
		Usage: buildModuleUsageURL(selector),
	}
}

func buildUsageSelector(selector, identifier, hash string) string {
	trimmed := strings.TrimSpace(selector)
	if trimmed != "" {
		return trimmed
	}
	name := strings.ToLower(strings.TrimSpace(identifier))
	normalizedHash := strings.TrimSpace(hash)
	if name == "" || normalizedHash == "" {
		return ""
	}
	return fmt.Sprintf("%s@%s", name, normalizedHash)
}

func buildModuleUsageURL(selector string) string {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		return ""
	}
	return strategyModulePrefix + url.PathEscape(trimmed) + strategyUsageSuffix
}

func (s *httpServer) createStrategyModule(w http.ResponseWriter, r *http.Request) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	limitRequestBody(w, r)
	payload, err := decodeStrategyModulePayload(r)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	if strings.TrimSpace(payload.Source) == "" {
		writeError(w, http.StatusBadRequest, "source required")
		return
	}
	opts := js.ModuleWriteOptions{
		Filename:      "",
		Tag:           "",
		Aliases:       nil,
		ReassignTags:  nil,
		PromoteLatest: true,
	}
	resolution, err := s.manager.UpsertStrategy([]byte(payload.Source), opts)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"status":            "pending_refresh",
		"strategyDirectory": s.manager.StrategyDirectory(),
		"module":            moduleResolutionPayload(resolution),
	})
}

func (s *httpServer) handleStrategyModule(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.Trim(r.URL.Path, "/")
	if rawPath == "" {
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
		return
	}
	segments := stripModuleRoutePrefix(splitPathSegments(rawPath))
	if len(segments) == 0 {
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
		return
	}
	name := strings.TrimSpace(segments[0])
	if name == "" {
		writeError(w, http.StatusNotFound, "module identifier required")
		return
	}
	if len(segments) >= 3 && strings.EqualFold(segments[1], "tags") {
		tag := strings.TrimSpace(segments[2])
		if tag == "" {
			writeError(w, http.StatusNotFound, "tag identifier required")
			return
		}
		if len(segments) != 3 {
			writeError(w, http.StatusNotFound, "invalid module tag path")
			return
		}
		switch r.Method {
		case http.MethodPut:
			s.assignStrategyModuleTag(w, r, name, tag)
		case http.MethodDelete:
			s.deleteStrategyModuleTag(w, r, name, tag)
		default:
			methodNotAllowed(w, http.MethodPut, http.MethodDelete)
		}
		return
	}
	if len(segments) == 2 {
		switch segments[1] {
		case strings.TrimPrefix(strategySourceSuffix, "/"):
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			s.getStrategyModuleSource(w, r, name)
			return
		case strings.TrimPrefix(strategyUsageSuffix, "/"):
			if r.Method != http.MethodGet {
				methodNotAllowed(w, http.MethodGet)
				return
			}
			s.getStrategyModuleUsage(w, r, name)
			return
		default:
			writeError(w, http.StatusNotFound, "invalid module path")
			return
		}
	}
	if len(segments) != 1 {
		writeError(w, http.StatusNotFound, "invalid module path")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getStrategyModule(w, name)
	case http.MethodPut:
		s.updateStrategyModule(w, r)
	case http.MethodDelete:
		s.deleteStrategyModule(w, name)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func splitPathSegments(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "/")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return out
}

func stripModuleRoutePrefix(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}
	idx := 0
	if strings.EqualFold(segments[0], "strategies") {
		idx++
	}
	if len(segments) > idx && strings.EqualFold(segments[idx], "modules") {
		idx++
	}
	if idx > 0 && idx <= len(segments) {
		return segments[idx:]
	}
	return segments
}

func (s *httpServer) getStrategyModule(w http.ResponseWriter, name string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	summary, err := s.manager.StrategyModule(name)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	summary.Running = nil
	writeJSON(w, http.StatusOK, summary)
}

func (s *httpServer) getStrategyModuleUsage(w http.ResponseWriter, r *http.Request, selector string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}

	query := r.URL.Query()

	includeStopped := false
	if raw := query.Get("includeStopped"); raw != "" {
		val, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "includeStopped must be a boolean")
			return
		}
		includeStopped = val
	}

	limit := -1
	if raw := query.Get("limit"); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil || val < 0 {
			writeError(w, http.StatusBadRequest, "limit must be a non-negative integer")
			return
		}
		limit = val
	}

	offset := 0
	if raw := query.Get("offset"); raw != "" {
		val, err := strconv.Atoi(raw)
		if err != nil || val < 0 {
			writeError(w, http.StatusBadRequest, "offset must be a non-negative integer")
			return
		}
		offset = val
	}

	usage, canonical, instances, err := s.manager.RevisionUsageDetail(selector, includeStopped)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}

	total := len(instances)
	if offset > total {
		offset = total
	}
	end := total
	if limit >= 0 && offset+limit < end {
		end = offset + limit
	}
	sliced := instances[offset:end]
	responseInstances := make([]instanceSummaryResponse, 0, len(sliced))
	for _, summary := range sliced {
		responseInstances = append(responseInstances, instanceSummaryResponse{
			InstanceSummary: summary,
			Links:           s.buildInstanceLinksFromSummary(summary),
		})
	}

	var limitValue any
	if limit >= 0 {
		limitValue = limit
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"selector":  canonical,
		"strategy":  usage.Strategy,
		"hash":      usage.Hash,
		"usage":     usage,
		"instances": responseInstances,
		"total":     total,
		"offset":    offset,
		"limit":     limitValue,
	})
}

func (s *httpServer) exportStrategyRegistry(w http.ResponseWriter, _ *http.Request) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	registry, usage, err := s.manager.RegistryExport()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"registry": registry,
		"usage":    usage,
	})
}

func (s *httpServer) updateStrategyModule(w http.ResponseWriter, r *http.Request) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	limitRequestBody(w, r)
	payload, err := decodeStrategyModulePayload(r)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	source := payload.Source
	if source == "" {
		writeError(w, http.StatusBadRequest, "source required")
		return
	}
	opts := js.ModuleWriteOptions{
		Filename:      "",
		Tag:           "",
		Aliases:       nil,
		ReassignTags:  nil,
		PromoteLatest: true,
	}
	resolution, err := s.manager.UpsertStrategy([]byte(source), opts)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":            "pending_refresh",
		"strategyDirectory": s.manager.StrategyDirectory(),
		"module":            moduleResolutionPayload(resolution),
	})
}

func (s *httpServer) deleteStrategyModule(w http.ResponseWriter, name string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	if err := s.manager.RemoveStrategy(name); err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *httpServer) assignStrategyModuleTag(w http.ResponseWriter, r *http.Request, name, tag string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	limitRequestBody(w, r)
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var payload strategyTagPayload
	if err := decoder.Decode(&payload); err != nil {
		writeDecodeError(w, err)
		return
	}
	hash := strings.TrimSpace(payload.Hash)
	if hash == "" {
		writeError(w, http.StatusBadRequest, "hash required")
		return
	}
	refresh := true
	if payload.Refresh != nil {
		refresh = *payload.Refresh
	}
	previous, err := s.manager.AssignStrategyTag(r.Context(), name, tag, hash, refresh)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":       "tag_assigned",
		"strategy":     name,
		"tag":          tag,
		"hash":         hash,
		"previousHash": previous,
		"refresh":      refresh,
	})
}

func (s *httpServer) deleteStrategyModuleTag(w http.ResponseWriter, r *http.Request, name, tag string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	allowOrphan := false
	if raw := r.URL.Query().Get("allowOrphan"); raw != "" {
		val, err := strconv.ParseBool(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "allowOrphan must be a boolean")
			return
		}
		allowOrphan = val
	}
	hash, err := s.manager.DeleteStrategyTag(name, tag, allowOrphan)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":      "tag_deleted",
		"strategy":    name,
		"tag":         tag,
		"hash":        hash,
		"allowOrphan": allowOrphan,
	})
}

func (s *httpServer) getStrategyModuleSource(w http.ResponseWriter, _ *http.Request, name string) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	source, err := s.manager.StrategySource(name)
	if err != nil {
		s.writeStrategyModuleError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(source)
}

func (s *httpServer) refreshStrategies(w http.ResponseWriter, r *http.Request) {
	if s.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "strategy manager unavailable")
		return
	}
	limitRequestBody(w, r)
	defer func() { _ = r.Body.Close() }()

	var payload strategyRefreshPayload
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
			writeDecodeError(w, err)
			return
		}
	}

	if len(payload.Hashes) == 0 && len(payload.Strategies) == 0 {
		if err := s.manager.RefreshJavaScriptStrategies(r.Context()); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "refreshed"})
		return
	}

	results, err := s.manager.RefreshJavaScriptStrategiesWithTargets(r.Context(), runtime.RefreshTargets{
		Hashes:     payload.Hashes,
		Strategies: payload.Strategies,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "partial_refresh",
		"results": results,
	})
}

func moduleResolutionPayload(res js.ModuleResolution) map[string]any {
	if res.Name == "" && res.Hash == "" {
		return nil
	}
	payload := map[string]any{
		"name": res.Name,
		"hash": res.Hash,
		"tag":  res.Tag,
	}
	if res.Alias != "" {
		payload["alias"] = res.Alias
	}
	if res.Module != nil {
		payload["file"] = res.Module.Filename
		payload["path"] = res.Module.Path
	}
	return payload
}

func (s *httpServer) writeStrategyModuleError(w http.ResponseWriter, err error) {
	if diagErr, ok := js.AsDiagnosticError(err); ok {
		diagnostics := diagErr.Diagnostics()
		payload := map[string]any{
			"status":  "error",
			"error":   "strategy_validation_failed",
			"message": diagErr.Error(),
		}
		if len(diagnostics) > 0 {
			payload["diagnostics"] = diagnostics
		}
		if payload["message"] == "" {
			payload["message"] = "strategy validation failed"
		}
		writeJSON(w, http.StatusUnprocessableEntity, payload)
		return
	}
	switch {
	case errors.Is(err, js.ErrModuleNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}
