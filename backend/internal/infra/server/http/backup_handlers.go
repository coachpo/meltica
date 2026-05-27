package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/app/lambda/runtime"
	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/app/risk"
	"github.com/coachpo/meltica/internal/infra/config"
)

func (s *httpServer) handleContextBackupExport(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.buildContextBackup())
}

func (s *httpServer) handleContextBackupRestore(w http.ResponseWriter, r *http.Request) {
	limitRequestBody(w, r)
	defer func() { _ = r.Body.Close() }()
	var payload contextBackup
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&payload); err != nil {
		writeDecodeError(w, err)
		return
	}
	if err := s.applyContextBackup(r.Context(), payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "restored"})
}

func (s *httpServer) buildContextBackup() contextBackup {
	var result contextBackup
	if s.manager != nil {
		result.Risk = riskConfigFromLimits(s.manager.RiskLimits())
	}
	if s.providers != nil {
		sanitized := s.providers.SanitizedProviderSpecs()
		filtered := make([]config.ProviderSpec, 0, len(sanitized))
		for _, spec := range sanitized {
			if s.isBaselineProvider(spec.Name) {
				continue
			}
			filtered = append(filtered, spec)
		}
		sort.Slice(filtered, func(i, j int) bool {
			return filtered[i].Name < filtered[j].Name
		})
		result.Providers = filtered
	}
	if s.manager != nil {
		summaries := s.manager.Instances()
		lambdas := make([]config.LambdaSpec, 0, len(summaries))
		for _, summary := range summaries {
			if s.isBaselineLambda(summary.ID) {
				continue
			}
			if snapshot, ok := s.manager.Instance(summary.ID); ok {
				lambdas = append(lambdas, lambdaSpecFromSnapshot(snapshot))
			}
		}
		sort.Slice(lambdas, func(i, j int) bool {
			return lambdas[i].ID < lambdas[j].ID
		})
		result.Lambdas = lambdas
	}
	return result
}

type contextBackupRestorePlan struct {
	providers    []config.ProviderSpec
	providerKeys map[string]struct{}
	lambdas      []config.LambdaSpec
	lambdaKeys   map[string]struct{}
	applyRisk    bool
	riskLimits   risk.Limits
}

func (s *httpServer) applyContextBackup(ctx context.Context, payload contextBackup) error {
	plan, err := s.preflightContextBackupRestore(ctx, payload)
	if err != nil {
		return err
	}
	return s.applyContextBackupPlan(ctx, plan)
}

func (s *httpServer) preflightContextBackupRestore(ctx context.Context, payload contextBackup) (contextBackupRestorePlan, error) {
	var plan contextBackupRestorePlan
	plan.providerKeys = make(map[string]struct{}, len(payload.Providers))
	plan.lambdaKeys = make(map[string]struct{}, len(payload.Lambdas))
	var emptyRiskLimits risk.Limits
	plan.providers = nil
	plan.lambdas = nil
	plan.applyRisk = false
	plan.riskLimits = emptyRiskLimits
	if s.providers == nil || s.manager == nil {
		return plan, fmt.Errorf("runtime managers unavailable")
	}
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return plan, fmt.Errorf("context error: %w", err)
		}
	}
	if hasRiskConfig(payload.Risk) {
		limits, err := parseRiskConfigLimits(payload.Risk)
		if err != nil {
			return plan, fmt.Errorf("risk: %w", err)
		}
		plan.applyRisk = true
		plan.riskLimits = limits
	}
	seenProviders := make(map[string]struct{}, len(payload.Providers))
	for _, spec := range payload.Providers {
		sanitized := provider.SanitizeProviderSpec(spec)
		sanitized.Name = strings.TrimSpace(sanitized.Name)
		sanitized.Adapter = strings.TrimSpace(sanitized.Adapter)
		if sanitized.Name == "" {
			return plan, fmt.Errorf("provider name required")
		}
		if sanitized.Adapter == "" {
			return plan, fmt.Errorf("provider %s adapter required", sanitized.Name)
		}
		key := strings.ToLower(sanitized.Name)
		if _, exists := seenProviders[key]; exists {
			return plan, fmt.Errorf("duplicate provider name %s", sanitized.Name)
		}
		seenProviders[key] = struct{}{}
		if s.isBaselineProvider(sanitized.Name) {
			continue
		}
		plan.providerKeys[key] = struct{}{}
		plan.providers = append(plan.providers, sanitized)
	}

	seenLambdas := make(map[string]struct{}, len(payload.Lambdas))
	for _, spec := range payload.Lambdas {
		copied := normalizeContextBackupLambdaSpec(spec)
		if copied.ID == "" {
			return plan, fmt.Errorf("lambda id required")
		}
		key := strings.ToLower(copied.ID)
		if _, exists := seenLambdas[key]; exists {
			return plan, fmt.Errorf("duplicate lambda id %s", copied.ID)
		}
		seenLambdas[key] = struct{}{}
		plan.lambdaKeys[key] = struct{}{}
		plan.lambdas = append(plan.lambdas, copied)
	}
	if err := s.validateContextBackupProviderReferences(plan.lambdas, plan.providerKeys); err != nil {
		return plan, err
	}
	if err := s.validateContextBackupProviderRemovals(plan.providerKeys, plan.lambdas); err != nil {
		return plan, err
	}
	if err := s.validateContextBackupProviderApply(ctx, plan.providers); err != nil {
		return plan, err
	}
	if err := s.validateContextBackupLambdaApply(plan.lambdas); err != nil {
		return plan, err
	}
	return plan, nil
}

func normalizeContextBackupLambdaSpec(spec config.LambdaSpec) config.LambdaSpec {
	copied := config.LambdaSpec{
		ID: strings.TrimSpace(spec.ID),
		Strategy: config.LambdaStrategySpec{
			Identifier: strings.TrimSpace(spec.Strategy.Identifier),
			Config:     cloneAnyMap(spec.Strategy.Config),
			Selector:   strings.TrimSpace(spec.Strategy.Selector),
			Tag:        strings.TrimSpace(spec.Strategy.Tag),
			Hash:       strings.TrimSpace(spec.Strategy.Hash),
		},
		ProviderSymbols: cloneProviderSymbolsMap(spec.ProviderSymbols),
		Providers:       cloneStringSlice(spec.Providers),
	}
	copied.Strategy.Normalize()
	copied.RefreshProviders()
	return copied
}

func (s *httpServer) validateContextBackupProviderReferences(specs []config.LambdaSpec, targetProviders map[string]struct{}) error {
	for _, spec := range specs {
		if len(spec.Providers) == 0 {
			return fmt.Errorf("lambda %s requires at least one provider", spec.ID)
		}
		for _, providerName := range spec.Providers {
			trimmed := strings.TrimSpace(providerName)
			if trimmed == "" {
				return fmt.Errorf("lambda %s requires at least one provider", spec.ID)
			}
			if s.isBaselineProvider(trimmed) {
				continue
			}
			if _, ok := targetProviders[strings.ToLower(trimmed)]; !ok {
				return fmt.Errorf("lambda %s references unknown provider %s", spec.ID, trimmed)
			}
		}
	}
	return nil
}

func (s *httpServer) validateContextBackupProviderRemovals(targetProviders map[string]struct{}, targetLambdas []config.LambdaSpec) error {
	targetUsage := s.contextBackupTargetProviderUsage(targetLambdas)
	existing := s.providers.SanitizedProviderSpecs()
	for _, spec := range existing {
		name := strings.TrimSpace(spec.Name)
		if name == "" || s.isBaselineProvider(name) {
			continue
		}
		key := strings.ToLower(name)
		if _, ok := targetProviders[key]; ok {
			continue
		}
		dependents := targetUsage[key]
		if len(dependents) > 0 {
			return fmt.Errorf("provider %s is in use by instances: %s", spec.Name, strings.Join(dependents, ", "))
		}
	}
	return nil
}

func (s *httpServer) contextBackupTargetProviderUsage(specs []config.LambdaSpec) map[string][]string {
	usage := make(map[string][]string)
	for _, spec := range specs {
		if s.isBaselineLambda(spec.ID) {
			continue
		}
		seen := make(map[string]struct{}, len(spec.Providers))
		for _, providerName := range spec.Providers {
			trimmed := strings.TrimSpace(providerName)
			if trimmed == "" || s.isBaselineProvider(trimmed) {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			usage[key] = append(usage[key], spec.ID)
		}
	}
	for key := range usage {
		sort.Strings(usage[key])
	}
	return usage
}

func (s *httpServer) validateContextBackupProviderApply(ctx context.Context, specs []config.ProviderSpec) error {
	if ctx != nil {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("context error: %w", err)
		}
	}
	for _, spec := range specs {
		detail, ok := s.providers.ProviderMetadataFor(spec.Name)
		if !ok {
			continue
		}
		if detail.Status == provider.StatusStarting {
			return fmt.Errorf("provider %s is starting", spec.Name)
		}
	}
	return nil
}

func (s *httpServer) validateContextBackupLambdaApply(specs []config.LambdaSpec) error {
	for _, spec := range specs {
		if s.isBaselineLambda(spec.ID) {
			continue
		}
		if spec.Strategy.Identifier == "" {
			return fmt.Errorf("lambda %s strategy required", spec.ID)
		}
		if len(spec.AllSymbols()) == 0 {
			return fmt.Errorf("lambda %s: instrument symbols required", spec.ID)
		}
		lookupName := contextBackupStrategyLookupName(spec.Strategy.Identifier)
		if lookupName == "" {
			return fmt.Errorf("lambda %s strategy required", spec.ID)
		}
		if _, ok := s.manager.StrategyDetail(lookupName); !ok {
			return fmt.Errorf("lambda %s strategy %q not registered", spec.ID, spec.Strategy.Identifier)
		}
	}
	return nil
}

func contextBackupStrategyLookupName(identifier string) string {
	trimmed := strings.TrimSpace(identifier)
	if before, _, ok := strings.Cut(trimmed, ":"); ok {
		return strings.TrimSpace(before)
	}
	if before, _, ok := strings.Cut(trimmed, "@"); ok {
		return strings.TrimSpace(before)
	}
	return trimmed
}

func (s *httpServer) applyContextBackupPlan(ctx context.Context, plan contextBackupRestorePlan) error {
	summaries := s.manager.Instances()
	for _, summary := range summaries {
		if s.isBaselineLambda(summary.ID) {
			continue
		}
		if _, ok := plan.lambdaKeys[strings.ToLower(summary.ID)]; !ok {
			if err := s.manager.Remove(summary.ID); err != nil && !errors.Is(err, runtime.ErrInstanceNotFound) {
				return fmt.Errorf("remove lambda %s: %w", summary.ID, err)
			}
		}
	}

	existing := s.providers.SanitizedProviderSpecs()
	for _, spec := range existing {
		if s.isBaselineProvider(spec.Name) {
			continue
		}
		if _, ok := plan.providerKeys[strings.ToLower(spec.Name)]; !ok {
			if err := s.providers.Remove(spec.Name); err != nil && !errors.Is(err, provider.ErrProviderNotFound) {
				return fmt.Errorf("remove provider %s: %w", spec.Name, err)
			}
		}
	}

	for _, spec := range plan.providers {
		if _, exists := s.providers.ProviderMetadataFor(spec.Name); exists {
			if _, err := s.providers.StopProvider(spec.Name); err != nil && !errors.Is(err, provider.ErrProviderNotRunning) {
				return fmt.Errorf("stop provider %s: %w", spec.Name, err)
			}
			if _, err := s.providers.Update(ctx, spec, false); err != nil {
				return fmt.Errorf("update provider %s: %w", spec.Name, err)
			}
		} else {
			if _, err := s.providers.Create(ctx, spec, false); err != nil {
				return fmt.Errorf("create provider %s: %w", spec.Name, err)
			}
		}
	}

	restored := make([]config.LambdaSpec, 0, len(plan.lambdas))
	for _, spec := range plan.lambdas {
		if s.isBaselineLambda(spec.ID) {
			continue
		}
		if err := s.manager.Remove(spec.ID); err != nil && !errors.Is(err, runtime.ErrInstanceNotFound) {
			return fmt.Errorf("prepare lambda %s: %w", spec.ID, err)
		}
		restored = append(restored, spec)
	}

	for _, spec := range restored {
		if _, err := s.manager.Create(spec); err != nil {
			return fmt.Errorf("restore lambda %s: %w", spec.ID, err)
		}
	}

	if plan.applyRisk {
		s.manager.UpdateRiskLimits(plan.riskLimits)
	}
	return nil
}

func hasRiskConfig(cfg config.RiskConfig) bool {
	return strings.TrimSpace(cfg.MaxPositionSize) != "" ||
		strings.TrimSpace(cfg.MaxNotionalValue) != "" ||
		strings.TrimSpace(cfg.NotionalCurrency) != "" ||
		cfg.OrderThrottle != 0 ||
		cfg.OrderBurst != 0 ||
		cfg.MaxConcurrentOrders != 0 ||
		cfg.PriceBandPercent != 0 ||
		len(cfg.AllowedOrderTypes) > 0 ||
		cfg.KillSwitchEnabled ||
		cfg.MaxRiskBreaches != 0 ||
		cfg.CircuitBreaker.Enabled ||
		cfg.CircuitBreaker.Threshold != 0 ||
		strings.TrimSpace(cfg.CircuitBreaker.Cooldown) != ""
}

func lambdaSpecFromSnapshot(snapshot runtime.InstanceSnapshot) config.LambdaSpec {
	return config.LambdaSpec{
		ID: snapshot.ID,
		Strategy: config.LambdaStrategySpec{
			Identifier: snapshot.Strategy.Identifier,
			Config:     cloneAnyMap(snapshot.Strategy.Config),
			Selector:   snapshot.Strategy.Selector,
			Tag:        snapshot.Strategy.Tag,
			Hash:       snapshot.Strategy.Hash,
		},
		ProviderSymbols: cloneProviderSymbolsMap(snapshot.ProviderSymbols),
		Providers:       cloneStringSlice(snapshot.Providers),
	}
}

func cloneProviderSymbolsMap(input map[string]config.ProviderSymbols) map[string]config.ProviderSymbols {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]config.ProviderSymbols, len(input))
	for key, symbols := range input {
		out[key] = config.ProviderSymbols{Symbols: cloneStringSlice(symbols.Symbols)}
	}
	return out
}

func cloneStringSlice(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, len(values))
	copy(out, values)
	return out
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneAnyMap(typed)
		case []any:
			out[key] = cloneAnySlice(typed)
		default:
			out[key] = typed
		}
	}
	return out
}

func cloneAnySlice(input []any) []any {
	if len(input) == 0 {
		return nil
	}
	out := make([]any, 0, len(input))
	for _, value := range input {
		switch typed := value.(type) {
		case map[string]any:
			out = append(out, cloneAnyMap(typed))
		case []any:
			out = append(out, cloneAnySlice(typed))
		default:
			out = append(out, typed)
		}
	}
	return out
}

func (s *httpServer) isBaselineProvider(name string) bool {
	if name == "" {
		return false
	}
	_, ok := s.baseProviders[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

func (s *httpServer) isBaselineLambda(id string) bool {
	if id == "" || s.manager == nil {
		return false
	}
	return s.manager.IsBaseline(id)
}
