package httpserver

import (
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/infra/config"
)

func (s *httpServer) listProviders(w http.ResponseWriter, _ *http.Request) {
	if s.providers == nil {
		writeJSON(w, http.StatusOK, map[string]any{"providers": []provider.RuntimeMetadata{}})
		return
	}
	metadata := s.providers.ProviderMetadataSnapshot()
	usage := s.providerUsage()
	for i := range metadata {
		nameKey := strings.ToLower(strings.TrimSpace(metadata[i].Name))
		if dependents, ok := usage[nameKey]; ok {
			metadata[i].DependentInstances = cloneStringSlice(dependents)
			metadata[i].DependentInstanceCount = len(dependents)
		} else {
			metadata[i].DependentInstances = []string{}
			metadata[i].DependentInstanceCount = 0
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": metadata})
}

func (s *httpServer) createProvider(w http.ResponseWriter, r *http.Request) {
	if s.providers == nil {
		writeError(w, http.StatusServiceUnavailable, "provider manager unavailable")
		return
	}
	limitRequestBody(w, r)
	payload, err := decodeProviderPayload(r)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	spec, enabled, err := buildProviderSpecFromPayload(payload)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := s.providers.Create(r.Context(), spec, false)
	if err != nil {
		s.writeProviderError(w, err)
		return
	}
	if enabled {
		if _, err := s.providers.StartProviderAsync(spec.Name); err != nil {
			s.writeProviderError(w, err)
			return
		}
	}
	location := providerDetailPrefix + spec.Name
	w.Header().Set("Location", location)
	writeJSON(w, http.StatusAccepted, detail)
}

func (s *httpServer) writeProviderDetail(w http.ResponseWriter, name string) {
	if name == "" {
		writeError(w, http.StatusNotFound, "provider name required")
		return
	}
	if s.providers == nil {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	meta, ok := s.providers.ProviderMetadataFor(name)
	if !ok {
		writeError(w, http.StatusNotFound, "provider not found")
		return
	}
	dependents := s.instancesUsingProvider(name)
	meta.DependentInstances = dependents
	meta.DependentInstanceCount = len(dependents)
	writeJSON(w, http.StatusOK, meta)
}

func (s *httpServer) handleProvider(w http.ResponseWriter, r *http.Request) {
	rest := strings.Trim(strings.TrimPrefix(r.URL.Path, providerDetailPrefix), "/")
	if rest == "" {
		writeError(w, http.StatusNotFound, "provider name required")
		return
	}

	name, action, hasAction := strings.Cut(rest, "/")
	name = strings.TrimSpace(name)
	if name == "" {
		writeError(w, http.StatusNotFound, "provider name required")
		return
	}

	if !hasAction {
		s.handleProviderResource(w, r, name)
		return
	}

	action = strings.TrimSpace(action)
	s.handleProviderAction(w, r, name, action)
}

func (s *httpServer) handleProviderResource(w http.ResponseWriter, r *http.Request, name string) {
	switch r.Method {
	case http.MethodGet:
		s.writeProviderDetail(w, name)
	case http.MethodPut:
		if s.providers == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager unavailable")
			return
		}
		limitRequestBody(w, r)
		payload, err := decodeProviderPayload(r)
		if err != nil {
			writeDecodeError(w, err)
			return
		}
		if strings.TrimSpace(payload.Name) == "" {
			payload.Name = name
		} else if !strings.EqualFold(strings.TrimSpace(payload.Name), name) {
			writeError(w, http.StatusBadRequest, "provider name mismatch")
			return
		}
		spec, enabled, err := buildProviderSpecFromPayload(payload)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		detail, err := s.providers.Update(r.Context(), spec, enabled)
		if err != nil {
			s.writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	case http.MethodDelete:
		if s.providers == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager unavailable")
			return
		}
		dependents := s.instancesUsingProvider(name)
		if len(dependents) > 0 {
			writeError(w, http.StatusConflict, fmt.Sprintf("provider %s is in use by instances: %s", name, strings.Join(dependents, ", ")))
			return
		}
		if err := s.providers.Remove(name); err != nil {
			s.writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "name": name})
	default:
		methodNotAllowed(w, http.MethodDelete, http.MethodGet, http.MethodPut)
	}
}

func (s *httpServer) handleProviderAction(w http.ResponseWriter, r *http.Request, name, action string) {
	switch action {
	case "start":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if s.providers == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager unavailable")
			return
		}
		detail, err := s.providers.StartProviderAsync(name)
		if err != nil {
			s.writeProviderError(w, err)
			return
		}
		location := providerDetailPrefix + name
		w.Header().Set("Location", location)
		writeJSON(w, http.StatusAccepted, detail)
	case "stop":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if s.providers == nil {
			writeError(w, http.StatusServiceUnavailable, "provider manager unavailable")
			return
		}
		detail, err := s.providers.StopProvider(name)
		if err != nil {
			s.writeProviderError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	case providerBalancesSuffix:
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleProviderBalances(w, r, name)
	default:
		writeError(w, http.StatusNotFound, "unsupported action")
	}
}

func (s *httpServer) listAdapters(w http.ResponseWriter, _ *http.Request) {
	if s.providers == nil {
		writeJSON(w, http.StatusOK, map[string]any{"adapters": []provider.AdapterMetadata{}})
		return
	}
	reg := s.providers.Registry()
	if reg == nil {
		writeJSON(w, http.StatusOK, map[string]any{"adapters": []provider.AdapterMetadata{}})
		return
	}
	metadata := reg.AdapterMetadataSnapshot()
	writeJSON(w, http.StatusOK, map[string]any{"adapters": metadata})
}

func (s *httpServer) getAdapter(w http.ResponseWriter, r *http.Request) {
	identifier := strings.Trim(strings.TrimPrefix(r.URL.Path, adapterDetailPrefix), "/")
	if identifier == "" {
		writeError(w, http.StatusNotFound, "adapter identifier required")
		return
	}
	if s.providers == nil {
		writeError(w, http.StatusNotFound, "adapter not found")
		return
	}
	reg := s.providers.Registry()
	if reg == nil {
		writeError(w, http.StatusNotFound, "adapter not found")
		return
	}
	meta, ok := reg.AdapterMetadata(identifier)
	if !ok {
		writeError(w, http.StatusNotFound, "adapter not found")
		return
	}
	writeJSON(w, http.StatusOK, meta)
}

func (s *httpServer) writeProviderError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, provider.ErrProviderExists):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, provider.ErrProviderNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, provider.ErrProviderRunning):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, provider.ErrProviderStarting):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, provider.ErrProviderNotRunning):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusBadRequest, err.Error())
	}
}

func (s *httpServer) handleProviderBalances(w http.ResponseWriter, r *http.Request, name string) {
	if s.orderStore == nil {
		writeError(w, http.StatusServiceUnavailable, "order store unavailable")
		return
	}
	values := r.URL.Query()
	limit, err := parseLimitParam(values.Get("limit"), defaultBalancesLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	asset := strings.TrimSpace(values.Get("asset"))
	records, err := s.orderStore.ListBalances(r.Context(), orderstore.BalanceQuery{
		Provider: strings.TrimSpace(name),
		Asset:    asset,
		Limit:    limit,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response := map[string]any{
		"balances": records,
		"count":    len(records),
	}
	writeJSON(w, http.StatusOK, response)
}

func decodeProviderPayload(r *http.Request) (providerPayload, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	var payload providerPayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode payload: %w", err)
	}
	return payload, nil
}

func decodeStrategyModulePayload(r *http.Request) (strategyModulePayload, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	var payload strategyModulePayload
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err != nil {
		return payload, fmt.Errorf("decode payload: %w", err)
	}
	return payload, nil
}

func buildProviderSpecFromPayload(payload providerPayload) (config.ProviderSpec, bool, error) {
	enabled := true
	if payload.Enabled != nil {
		enabled = *payload.Enabled
	}

	name := strings.TrimSpace(payload.Name)
	if name == "" {
		return config.ProviderSpec{}, false, fmt.Errorf("name required")
	}

	identifier := strings.TrimSpace(payload.Adapter.Identifier)
	if identifier == "" {
		return config.ProviderSpec{}, false, fmt.Errorf("adapter.identifier required")
	}

	adapterConfig := map[string]any{
		"identifier": identifier,
	}
	cleanConfig := sanitizeAdapterConfig(payload.Adapter.Config)
	if len(cleanConfig) > 0 {
		adapterConfig["config"] = cleanConfig
	}

	specs, err := config.BuildProviderSpecs(map[config.Provider]map[string]any{
		config.Provider(name): {
			"adapter": adapterConfig,
		},
	})
	if err != nil {
		return config.ProviderSpec{}, false, fmt.Errorf("build provider spec: %w", err)
	}
	if len(specs) == 0 {
		return config.ProviderSpec{}, false, fmt.Errorf("provider spec not generated")
	}
	return specs[0], enabled, nil
}

func sanitizeAdapterConfig(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	clean := make(map[string]any, len(input))
	for key, value := range input {
		if key == "" {
			continue
		}
		if sanitized, ok := sanitizeConfigValue(value); ok {
			clean[key] = sanitized
		}
	}
	if len(clean) == 0 {
		return nil
	}
	return clean
}

func sanitizeConfigValue(value any) (any, bool) {
	switch v := value.(type) {
	case nil:
		return nil, false
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, false
		}
		return trimmed, true
	case []any:
		clean, ok := sanitizeConfigSlice(v)
		if !ok {
			return nil, false
		}
		return clean, true
	case map[string]any:
		clean := sanitizeAdapterConfig(v)
		if len(clean) == 0 {
			return nil, false
		}
		return clean, true
	default:
		return v, true
	}
}

func sanitizeConfigSlice(values []any) ([]any, bool) {
	if len(values) == 0 {
		return nil, false
	}
	clean := make([]any, 0, len(values))
	for _, elem := range values {
		if sanitized, ok := sanitizeConfigValue(elem); ok {
			clean = append(clean, sanitized)
		}
	}
	if len(clean) == 0 {
		return nil, false
	}
	return clean, true
}

func (s *httpServer) providerUsage() map[string][]string {
	usage := make(map[string][]string)
	if s.manager == nil {
		return usage
	}
	appendProvider := func(list *[]string, seen map[string]struct{}, name string) {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			return
		}
		normalized := strings.ToLower(trimmed)
		if _, exists := seen[normalized]; exists {
			return
		}
		seen[normalized] = struct{}{}
		*list = append(*list, normalized)
	}
	summaries := s.manager.Instances()
	for _, summary := range summaries {
		if s.isBaselineLambda(summary.ID) {
			continue
		}
		normalizedProviders := make([]string, 0, len(summary.Providers))
		seen := make(map[string]struct{}, len(summary.Providers))
		for _, providerName := range summary.Providers {
			appendProvider(&normalizedProviders, seen, providerName)
		}
		if len(normalizedProviders) == 0 {
			if snapshot, ok := s.manager.Instance(summary.ID); ok {
				for _, providerName := range snapshot.Providers {
					appendProvider(&normalizedProviders, seen, providerName)
				}
				if len(normalizedProviders) == 0 && len(snapshot.ProviderSymbols) > 0 {
					providerNames := make([]string, 0, len(snapshot.ProviderSymbols))
					for name := range snapshot.ProviderSymbols {
						providerNames = append(providerNames, name)
					}
					sort.Strings(providerNames)
					for _, providerName := range providerNames {
						appendProvider(&normalizedProviders, seen, providerName)
					}
				}
			}
		}
		for _, key := range normalizedProviders {
			usage[key] = append(usage[key], summary.ID)
		}
	}
	for key := range usage {
		sort.Strings(usage[key])
	}
	return usage
}

func (s *httpServer) instancesUsingProvider(name string) []string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return []string{}
	}
	usage := s.providerUsage()
	list := usage[strings.ToLower(trimmed)]
	if len(list) == 0 {
		return []string{}
	}
	return cloneStringSlice(list)
}
