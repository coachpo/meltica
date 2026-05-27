// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/coachpo/meltica/internal/app/lambda/core"
	"github.com/coachpo/meltica/internal/app/lambda/js"
	"github.com/coachpo/meltica/internal/app/lambda/strategies"
	"github.com/coachpo/meltica/internal/domain/schema"
	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/coachpo/meltica/internal/infra/telemetry"
)

// RefreshTargets narrows refresh operations to specific strategies or revision hashes.
type RefreshTargets struct {
	Strategies []string
	Hashes     []string
}

// RefreshResult captures the outcome of a targeted refresh operation.
type RefreshResult struct {
	Selector     string   `json:"selector"`
	Strategy     string   `json:"strategy"`
	Hash         string   `json:"hash"`
	PreviousHash string   `json:"previousHash,omitempty"`
	Instances    []string `json:"instances,omitempty"`
	Reason       string   `json:"reason"`
}

type refreshTargetFilter struct {
	all bool

	selectors   map[string]struct{}
	identifiers map[string]struct{}
	hashes      map[string]struct{}

	requestedSelectors []string
	requestedHashes    []string

	matchedSelectors map[string]bool
	matchedHashes    map[string]bool
}

type refreshMatch struct {
	matched       bool
	byHash        bool
	bySelector    bool
	byIdentifier  bool
	selectorKey   string
	identifierKey string
	hashKey       string
}

func (m *Manager) recordStrategyValidationFailure(err error) {
	if m == nil {
		return
	}
	diagErr, ok := js.AsDiagnosticError(err)
	if !ok {
		return
	}
	diagnostics := diagErr.Diagnostics()
	if m.logger != nil {
		if len(diagnostics) == 0 {
			m.logger.Printf("strategy validation failed: %v", diagErr)
		} else {
			for _, diag := range diagnostics {
				m.logger.Printf("strategy validation failed: stage=%s message=%s line=%d column=%d hint=%s",
					diag.Stage, diag.Message, diag.Line, diag.Column, diag.Hint)
			}
		}
	}
	if m.uploadValidationFailures == nil {
		return
	}
	env := telemetry.Environment()
	ctx := context.Background()
	if len(diagnostics) == 0 {
		m.uploadValidationFailures.Add(ctx, 1, metric.WithAttributes(
			attribute.String("environment", env),
			attribute.String("stage", "unknown"),
		))
		return
	}
	for _, diag := range diagnostics {
		stage := string(diag.Stage)
		if stage == "" {
			stage = "unknown"
		}
		m.uploadValidationFailures.Add(ctx, 1, metric.WithAttributes(
			attribute.String("environment", env),
			attribute.String("stage", stage),
		))
	}
}

func newRefreshTargetFilter(targets RefreshTargets) refreshTargetFilter {
	filter := refreshTargetFilter{
		all:                false,
		selectors:          make(map[string]struct{}),
		identifiers:        make(map[string]struct{}),
		hashes:             make(map[string]struct{}),
		requestedSelectors: make([]string, 0, len(targets.Strategies)),
		requestedHashes:    make([]string, 0, len(targets.Hashes)),
		matchedSelectors:   make(map[string]bool),
		matchedHashes:      make(map[string]bool),
	}

	for _, raw := range targets.Strategies {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			selector := normalizeSelector(trimmed)
			if selector != "" {
				filter.selectors[selector] = struct{}{}
				filter.requestedSelectors = append(filter.requestedSelectors, selector)
			}
			name := normalizeStrategyName(trimmed)
			if name != "" {
				filter.identifiers[name] = struct{}{}
			}
		}
	}

	for _, raw := range targets.Hashes {
		if trimmed := normalizeRevisionHash(raw); trimmed != "" {
			filter.hashes[trimmed] = struct{}{}
			filter.requestedHashes = append(filter.requestedHashes, trimmed)
		}
	}

	filter.all = len(filter.selectors) == 0 && len(filter.identifiers) == 0 && len(filter.hashes) == 0
	return filter
}

func (f *refreshTargetFilter) matchSpec(spec config.LambdaSpec) refreshMatch {
	if f == nil {
		var empty refreshMatch
		return empty
	}
	if f.all {
		var match refreshMatch
		match.matched = true
		return match
	}

	var match refreshMatch

	hash := normalizeRevisionHash(spec.Strategy.Hash)
	if hash != "" {
		if _, ok := f.hashes[hash]; ok {
			match.matched = true
			match.byHash = true
			match.hashKey = hash
		}
	}

	selector := normalizeSelector(spec.Strategy.Selector)
	if selector != "" {
		if _, ok := f.selectors[selector]; ok {
			match.matched = true
			match.bySelector = true
			match.selectorKey = selector
		}
	}

	identifier := normalizeStrategyName(spec.Strategy.Identifier)
	if identifier != "" {
		if _, ok := f.identifiers[identifier]; ok {
			match.matched = true
			match.byIdentifier = true
			match.identifierKey = identifier
		}
	}

	if match.bySelector && match.selectorKey == "" {
		match.selectorKey = selector
	}
	if match.byIdentifier && match.identifierKey == "" {
		match.identifierKey = identifier
	}

	return match
}

func (f *refreshTargetFilter) recordMatch(match refreshMatch) {
	if f == nil || f.all || !match.matched {
		return
	}
	if match.byHash && match.hashKey != "" {
		if f.matchedHashes == nil {
			f.matchedHashes = make(map[string]bool, len(f.hashes))
		}
		f.matchedHashes[match.hashKey] = true
	}
	if match.bySelector && match.selectorKey != "" {
		if f.matchedSelectors == nil {
			f.matchedSelectors = make(map[string]bool, len(f.selectors)+len(f.identifiers))
		}
		f.matchedSelectors[match.selectorKey] = true
	}
	if match.byIdentifier && match.identifierKey != "" {
		if f.matchedSelectors == nil {
			f.matchedSelectors = make(map[string]bool, len(f.selectors)+len(f.identifiers))
		}
		f.matchedSelectors[match.identifierKey] = true
	}
}

func (f refreshTargetFilter) unmatchedHashTargets() []string {
	if f.all || len(f.requestedHashes) == 0 {
		return nil
	}
	var out []string
	for _, hash := range f.requestedHashes {
		if f.matchedHashes != nil && f.matchedHashes[hash] {
			continue
		}
		out = append(out, hash)
	}
	return out
}

func (f refreshTargetFilter) unmatchedSelectors() []string {
	if f.all || len(f.requestedSelectors) == 0 {
		return nil
	}
	var out []string
	for _, selector := range f.requestedSelectors {
		if f.matchedSelectors != nil && f.matchedSelectors[selector] {
			continue
		}
		out = append(out, selector)
	}
	return out
}

func ensureRefreshResult(results map[string]*RefreshResult, id string, spec config.LambdaSpec) *RefreshResult {
	if results == nil {
		return nil
	}
	if existing, ok := results[id]; ok {
		return existing
	}
	selector := spec.Strategy.Selector
	if selector == "" {
		selector = spec.Strategy.Identifier
	}
	result := &RefreshResult{
		Selector:     selector,
		Strategy:     spec.Strategy.Identifier,
		Hash:         spec.Strategy.Hash,
		PreviousHash: spec.Strategy.Hash,
		Instances:    []string{id},
		Reason:       "",
	}
	results[id] = result
	return result
}

func pickRefreshReason(current, candidate string) string {
	switch candidate {
	case "":
		return current
	case "retired":
		return "retired"
	case "refreshed":
		if current == "" || current == "alreadyPinned" {
			return "refreshed"
		}
		return current
	case "alreadyPinned":
		if current == "" {
			return "alreadyPinned"
		}
		return current
	default:
		if current == "" {
			return candidate
		}
		return current
	}
}

func buildUnmatchedRefreshResults(filter refreshTargetFilter) []RefreshResult {
	if filter.all {
		return nil
	}
	var out []RefreshResult
	for _, hash := range filter.unmatchedHashTargets() {
		out = append(out, RefreshResult{
			Selector:     hash,
			Strategy:     "",
			Hash:         hash,
			PreviousHash: hash,
			Instances:    nil,
			Reason:       "retired",
		})
	}
	for _, selector := range filter.unmatchedSelectors() {
		out = append(out, RefreshResult{
			Selector:     selector,
			Strategy:     "",
			Hash:         "",
			PreviousHash: "",
			Instances:    nil,
			Reason:       "retired",
		})
	}
	return out
}

// RefreshJavaScriptStrategies reloads JavaScript modules and restarts affected instances.
func (m *Manager) RefreshJavaScriptStrategies(ctx context.Context) error {
	var targets RefreshTargets
	_, err := m.refreshJavaScriptStrategies(ctx, targets)
	return err
}

// RefreshJavaScriptStrategiesWithTargets performs a filtered refresh limited to the supplied targets.
func (m *Manager) RefreshJavaScriptStrategiesWithTargets(ctx context.Context, targets RefreshTargets) ([]RefreshResult, error) {
	return m.refreshJavaScriptStrategies(ctx, targets)
}

func (m *Manager) refreshJavaScriptStrategies(ctx context.Context, targets RefreshTargets) ([]RefreshResult, error) {
	if _, err := m.installJavaScriptStrategies(ctx); err != nil {
		return nil, err
	}
	selections := m.snapshotStrategySelections()
	filter := newRefreshTargetFilter(targets)
	if len(selections) == 0 {
		results := buildUnmatchedRefreshResults(filter)
		return results, nil
	}

	dynamicSet := m.currentDynamicSet()
	updates := make(map[string]config.LambdaSpec)
	restartIDs := make([]string, 0)
	stopOnly := make([]string, 0)
	resultsByInstance := make(map[string]*RefreshResult)

	for id, selection := range selections {
		spec := selection.Spec
		name := strings.ToLower(strings.TrimSpace(spec.Strategy.Identifier))
		if _, ok := dynamicSet[name]; !ok && spec.Strategy.Hash == "" {
			continue
		}
		match := filter.matchSpec(spec)
		if !match.matched {
			continue
		}
		filter.recordMatch(match)

		result := ensureRefreshResult(resultsByInstance, id, spec)
		selector := spec.Strategy.Selector
		if selector == "" {
			selector = spec.Strategy.Identifier
		}
		if selector == "" || m.jsLoader == nil {
			result.Reason = pickRefreshReason(result.Reason, "retired")
			stopOnly = append(stopOnly, id)
			continue
		}
		resolution, err := m.jsLoader.ResolveReference(selector)
		if err != nil {
			result.Reason = pickRefreshReason(result.Reason, "retired")
			stopOnly = append(stopOnly, id)
			continue
		}

		oldHash := spec.Strategy.Hash
		spec.Strategy.Identifier = resolution.Name
		spec.Strategy.Hash = resolution.Hash
		spec.Strategy.Tag = resolution.Tag
		spec.Strategy.Selector = canonicalSelector(selector, resolution)
		updates[id] = spec

		result.Hash = resolution.Hash
		result.Selector = spec.Strategy.Selector
		result.Strategy = spec.Strategy.Identifier
		result.Reason = pickRefreshReason(result.Reason, "alreadyPinned")
		if oldHash != resolution.Hash {
			result.Reason = pickRefreshReason(result.Reason, "refreshed")
		}

		if selection.Running && oldHash != resolution.Hash {
			restartIDs = append(restartIDs, id)
		}
	}

	if len(updates) > 0 {
		m.mu.Lock()
		for id, updated := range updates {
			strategy, hash, _ := revisionSignatureForSpec(updated)
			m.ensureRevisionUsageLocked(strategy, hash)
			m.specs[id] = cloneSpec(updated)
		}
		m.mu.Unlock()
	}

	for _, id := range restartIDs {
		if err := m.Stop(id); err != nil && !errors.Is(err, ErrInstanceNotRunning) {
			if m.logger != nil {
				m.logger.Printf("stop strategy %s: %v", id, err)
			}
		}
	}
	for _, id := range restartIDs {
		if err := m.Start(ctx, id); err != nil && m.logger != nil {
			if !errors.Is(err, ErrInstanceAlreadyRunning) {
				m.logger.Printf("restart strategy %s: %v", id, err)
			}
		}
	}
	for _, id := range stopOnly {
		if err := m.Stop(id); err != nil && !errors.Is(err, ErrInstanceNotRunning) {
			if m.logger != nil {
				m.logger.Printf("stop strategy %s: %v", id, err)
			}
		}
	}

	results := make([]RefreshResult, 0, len(resultsByInstance))
	for _, res := range resultsByInstance {
		if res.Reason == "" {
			res.Reason = "alreadyPinned"
		}
		results = append(results, *res)
	}

	unmatched := buildUnmatchedRefreshResults(filter)
	results = append(results, unmatched...)

	sort.Slice(results, func(i, j int) bool {
		if results[i].Selector != results[j].Selector {
			return results[i].Selector < results[j].Selector
		}
		return results[i].Strategy < results[j].Strategy
	})

	return results, nil
}

func normalizeStrategyDefinition(def StrategyDefinition) (StrategyDefinition, error) {
	name := strings.ToLower(strings.TrimSpace(def.meta.Name))
	if name == "" {
		return StrategyDefinition{}, fmt.Errorf("strategy name required")
	}
	if def.factory == nil {
		return StrategyDefinition{}, fmt.Errorf("strategy %s missing factory", name)
	}
	def.meta.Name = name

	if len(def.meta.Events) == 0 {
		strat, err := def.factory(map[string]any{})
		if err == nil && strat != nil {
			def.meta.Events = append([]schema.EventType(nil), strat.SubscribedEvents()...)
			closeStrategy(strat)
		}
	}
	def.meta.Events = append([]schema.EventType(nil), def.meta.Events...)

	fields := make([]strategies.ConfigField, len(def.meta.Config))
	copy(fields, def.meta.Config)
	sort.Slice(fields, func(i, j int) bool { return fields[i].Name < fields[j].Name })
	def.meta.Config = fields

	return def, nil
}

func (m *Manager) installJavaScriptStrategies(ctx context.Context) (map[string]struct{}, error) {
	if m.jsLoader == nil {
		return nil, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := m.jsLoader.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("load javascript strategies: %w", err)
	}

	summaries := m.jsLoader.List()
	definitions := make(map[string]StrategyDefinition, len(summaries))
	for _, summary := range summaries {
		module, err := m.jsLoader.Get(summary.Name)
		if err != nil {
			return nil, fmt.Errorf("load strategy %s: %w", summary.Name, err)
		}
		mod := module
		def := StrategyDefinition{
			meta: strategies.CloneMetadata(summary.Metadata),
			factory: func(cfg map[string]any) (core.TradingStrategy, error) {
				return js.NewStrategy(mod, cfg, m.logger)
			},
		}
		normalized, err := normalizeStrategyDefinition(def)
		if err != nil {
			return nil, fmt.Errorf("strategy %s: %w", summary.Name, err)
		}
		definitions[normalized.meta.Name] = normalized
	}

	m.mu.Lock()
	for name := range m.dynamic {
		delete(m.strategies, name)
		if baseDef, ok := m.base[name]; ok {
			m.strategies[name] = baseDef
		}
	}
	m.dynamic = make(map[string]struct{}, len(definitions))
	for name, def := range definitions {
		m.strategies[name] = def
		m.dynamic[name] = struct{}{}
	}
	m.mu.Unlock()

	return copyStringSet(m.dynamic), nil
}

func (m *Manager) currentDynamicSet() map[string]struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return copyStringSet(m.dynamic)
}

// StrategyCatalog returns all available strategy metadata.
func (m *Manager) StrategyCatalog() []strategies.Metadata {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]strategies.Metadata, 0, len(m.strategies))
	for _, def := range m.strategies {
		out = append(out, def.Metadata())
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// StrategyDetail returns metadata for a specific strategy by name.
func (m *Manager) StrategyDetail(name string) (strategies.Metadata, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	def, ok := m.strategies[strings.ToLower(strings.TrimSpace(name))]
	if !ok {
		var empty strategies.Metadata
		return empty, false
	}
	return def.Metadata(), true
}

type strategySelection struct {
	Spec    config.LambdaSpec
	Running bool
}

func (m *Manager) snapshotStrategySelections() map[string]strategySelection {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]strategySelection, len(m.specs))
	for id, spec := range m.specs {
		_, running := m.instances[id]
		out[id] = strategySelection{
			Spec:    cloneSpec(spec),
			Running: running,
		}
	}
	return out
}

func canonicalSelector(raw string, res js.ModuleResolution) string {
	name := strings.ToLower(strings.TrimSpace(res.Name))
	if name == "" {
		name = strings.ToLower(strings.TrimSpace(raw))
	}
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return name
	}
	if strings.Contains(trimmed, "@") && res.Hash != "" {
		return fmt.Sprintf("%s@%s", name, res.Hash)
	}
	if strings.Contains(trimmed, ":") && res.Tag != "" {
		return fmt.Sprintf("%s:%s", name, res.Tag)
	}
	return name
}

// StrategyModules returns metadata for the currently loaded JavaScript strategy modules.
func (m *Manager) StrategyModules() []js.ModuleSummary {
	if m == nil || m.jsLoader == nil {
		return nil
	}
	usage := convertModuleUsageSnapshots(m.RevisionUsageSnapshot())
	return m.jsLoader.ListWithUsage(usage)
}

// StrategyModule returns module metadata for a specific strategy.
func (m *Manager) StrategyModule(name string) (js.ModuleSummary, error) {
	if m == nil || m.jsLoader == nil {
		return js.ModuleSummary{}, js.ErrModuleNotFound
	}
	usage := convertModuleUsageSnapshots(m.RevisionUsageSnapshot())
	summary, err := m.jsLoader.ModuleWithUsage(name, usage)
	if err != nil {
		return js.ModuleSummary{}, fmt.Errorf("strategy module %q: %w", name, err)
	}
	return summary, nil
}

// ResolveStrategySelector resolves a module selector into the corresponding revision.
func (m *Manager) ResolveStrategySelector(selector string) (js.ModuleResolution, error) {
	if m == nil || m.jsLoader == nil {
		var empty js.ModuleResolution
		return empty, fmt.Errorf("strategy loader unavailable")
	}
	resolution, err := m.jsLoader.ResolveReference(selector)
	if err != nil {
		var empty js.ModuleResolution
		return empty, fmt.Errorf("resolve selector %q: %w", selector, err)
	}
	return resolution, nil
}

// RegistryExport returns the registry manifest alongside usage summaries.
func (m *Manager) RegistryExport() (js.RegistrySnapshot, []RevisionUsageSummary, error) {
	if m == nil || m.jsLoader == nil {
		return nil, nil, fmt.Errorf("strategy loader unavailable")
	}
	snapshot, err := m.jsLoader.RegistrySnapshot()
	if err != nil {
		return nil, nil, fmt.Errorf("registry snapshot: %w", err)
	}
	usage := m.RevisionUsageSnapshot()
	return snapshot, usage, nil
}

// StrategySource retrieves the raw JavaScript source for the named strategy.
func (m *Manager) StrategySource(name string) ([]byte, error) {
	if m == nil || m.jsLoader == nil {
		return nil, js.ErrModuleNotFound
	}
	source, err := m.jsLoader.Read(name)
	if err != nil {
		return nil, fmt.Errorf("strategy source %q: %w", name, err)
	}
	return source, nil
}

// UpsertStrategy writes or replaces a JavaScript strategy module.
func (m *Manager) UpsertStrategy(source []byte, opts js.ModuleWriteOptions) (js.ModuleResolution, error) {
	if m == nil || m.jsLoader == nil {
		return js.ModuleResolution{Name: "", Hash: "", Tag: "", Alias: "", Module: nil}, fmt.Errorf("strategy loader unavailable")
	}
	resolution, err := m.jsLoader.Store(source, opts)
	if err == nil {
		return resolution, nil
	}
	m.recordStrategyValidationFailure(err)
	if !errors.Is(err, js.ErrRegistryUnavailable) {
		return js.ModuleResolution{Name: "", Hash: "", Tag: "", Alias: "", Module: nil}, fmt.Errorf("strategy upsert: %w", err)
	}
	if err := m.jsLoader.Write(source); err != nil {
		m.recordStrategyValidationFailure(err)
		return js.ModuleResolution{Name: "", Hash: "", Tag: "", Alias: "", Module: nil}, fmt.Errorf("strategy upsert: %w", err)
	}
	return js.ModuleResolution{Name: "", Hash: "", Tag: "", Alias: "", Module: nil}, nil
}

// AssignStrategyTag re-points the supplied tag alias to the provided revision hash.
func (m *Manager) AssignStrategyTag(ctx context.Context, name, tag, hash string, refresh bool) (string, error) {
	if m == nil || m.jsLoader == nil {
		return "", fmt.Errorf("strategy loader unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	previous, err := m.jsLoader.AssignTag(name, tag, hash)
	if err != nil {
		return "", fmt.Errorf("assign tag %s:%s: %w", name, tag, err)
	}
	if m.logger != nil {
		m.logger.Printf("strategy tag %s:%s moved from %s to %s", name, tag, previous, hash)
	}
	if refresh && !strings.EqualFold(previous, hash) {
		_, refreshErr := m.RefreshJavaScriptStrategiesWithTargets(ctx, RefreshTargets{Strategies: []string{name}, Hashes: nil})
		if refreshErr != nil {
			return previous, fmt.Errorf("refresh after tag move: %w", refreshErr)
		}
	}
	if m.tagAssignmentCounter != nil && !strings.EqualFold(previous, hash) {
		env := telemetry.Environment()
		m.tagAssignmentCounter.Add(ctx, 1, metric.WithAttributes(
			attribute.String("environment", env),
			attribute.String("strategy", strings.ToLower(strings.TrimSpace(name))),
			attribute.String("tag", strings.ToLower(strings.TrimSpace(tag))),
		))
	}
	return previous, nil
}

// DeleteStrategyTag removes a tag alias while honoring guard rails.
func (m *Manager) DeleteStrategyTag(name, tag string, allowOrphan bool) (string, error) {
	if m == nil || m.jsLoader == nil {
		return "", fmt.Errorf("strategy loader unavailable")
	}
	opts := js.TagDeleteOptions{AllowOrphan: allowOrphan}
	hash, err := m.jsLoader.DeleteTagWithOptions(name, tag, opts)
	if err != nil {
		return "", fmt.Errorf("delete tag %s:%s: %w", name, tag, err)
	}
	if m.logger != nil {
		m.logger.Printf("strategy tag %s:%s removed (hash %s)", name, tag, hash)
	}
	if m.tagDeleteCounter != nil {
		env := telemetry.Environment()
		m.tagDeleteCounter.Add(context.Background(), 1, metric.WithAttributes(
			attribute.String("environment", env),
			attribute.String("strategy", strings.ToLower(strings.TrimSpace(name))),
			attribute.String("tag", strings.ToLower(strings.TrimSpace(tag))),
			attribute.Bool("allowOrphan", allowOrphan),
		))
	}
	return hash, nil
}

// RemoveStrategy deletes the JavaScript strategy file by name.
func (m *Manager) RemoveStrategy(name string) error {
	if m == nil || m.jsLoader == nil {
		return js.ErrModuleNotFound
	}

	selector := strings.TrimSpace(name)
	if selector == "" {
		return fmt.Errorf("strategy remove: selector required")
	}

	var (
		inUseErr error
	)
	if strings.ContainsAny(selector, "@:") {
		resolution, err := m.jsLoader.ResolveReference(selector)
		if err != nil {
			return fmt.Errorf("strategy remove %q: %w", selector, err)
		}
		if resolution.Hash != "" && m.hashInUse(resolution.Hash) {
			inUseErr = fmt.Errorf("strategy revision %s is in use", resolution.Hash)
		}
	} else if m.strategyInUse(selector) {
		inUseErr = fmt.Errorf("strategy %s is in use by running instances", selector)
	}
	if inUseErr != nil {
		return inUseErr
	}

	if err := m.jsLoader.Delete(selector); err != nil {
		return fmt.Errorf("strategy remove %q: %w", name, err)
	}
	return nil
}

// StrategyDirectory returns the filesystem directory backing JavaScript strategies.
func (m *Manager) StrategyDirectory() string {
	if m == nil {
		return ""
	}
	return m.strategyDir
}

func (m *Manager) strategyInUse(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(name))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, spec := range m.specs {
		if strings.EqualFold(spec.Strategy.Identifier, lower) {
			return true
		}
	}
	return false
}

func (m *Manager) hashInUse(hash string) bool {
	if hash == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(hash))
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, spec := range m.specs {
		if strings.EqualFold(spec.Strategy.Hash, normalized) {
			return true
		}
	}
	return false
}
