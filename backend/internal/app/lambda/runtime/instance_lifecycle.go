// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"

	"github.com/coachpo/meltica/internal/app/dispatcher"
	"github.com/coachpo/meltica/internal/app/lambda/core"
	"github.com/coachpo/meltica/internal/app/lambda/js"
	"github.com/coachpo/meltica/internal/domain/schema"
	"github.com/coachpo/meltica/internal/infra/config"
)

// SetLifecycleContext configures the parent context used to run lambda instances.
func (m *Manager) SetLifecycleContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	m.lifecycleMu.Lock()
	m.lifecycleCtx = ctx
	m.lifecycleMu.Unlock()
}

func (m *Manager) parentContext() context.Context {
	m.lifecycleMu.RLock()
	ctx := m.lifecycleCtx
	m.lifecycleMu.RUnlock()
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

// Create creates a new lambda instance from the specification.
func (m *Manager) Create(spec config.LambdaSpec) (*core.BaseLambda, error) {
	spec = sanitizeSpec(spec)
	if spec.ID == "" || len(spec.Providers) == 0 || spec.Strategy.Identifier == "" {
		return nil, fmt.Errorf("strategy instance requires id, providers, and strategy")
	}
	if len(spec.AllSymbols()) == 0 {
		return nil, fmt.Errorf("strategy %s: instrument symbols required", spec.ID)
	}
	if err := m.ensureSpec(&spec, false); err != nil {
		return nil, fmt.Errorf("ensure spec %s: %w", spec.ID, err)
	}
	m.setBaselineInstance(spec.ID, false)
	m.setDynamicInstance(spec.ID, true)
	m.persistStrategy(spec.ID)
	return nil, nil
}

func (m *Manager) ensureSpec(spec *config.LambdaSpec, allowReplace bool) error {
	if spec == nil {
		return fmt.Errorf("lambda spec required")
	}
	if spec.Strategy.Config == nil {
		spec.Strategy.Config = make(map[string]any)
	}

	rawIdentifier := strings.TrimSpace(spec.Strategy.Identifier)
	baseName := strings.ToLower(rawIdentifier)
	requireResolution := strings.ContainsAny(rawIdentifier, ":@")
	if !requireResolution {
		if current := m.currentDynamicSet(); len(current) > 0 {
			if _, ok := current[baseName]; ok {
				requireResolution = true
			}
		}
	}

	if requireResolution {
		if m.jsLoader == nil {
			return fmt.Errorf("strategy loader unavailable")
		}
		res, err := m.jsLoader.ResolveReference(rawIdentifier)
		if err != nil {
			return fmt.Errorf("resolve strategy %q: %w", rawIdentifier, err)
		}
		spec.Strategy.Identifier = res.Name
		spec.Strategy.Hash = res.Hash
		spec.Strategy.Tag = res.Tag
		spec.Strategy.Selector = canonicalSelector(rawIdentifier, res)
	} else {
		spec.Strategy.Identifier = strings.ToLower(rawIdentifier)
		spec.Strategy.Selector = spec.Strategy.Identifier
		spec.Strategy.Hash = ""
		spec.Strategy.Tag = ""
	}

	name := strings.ToLower(strings.TrimSpace(spec.Strategy.Identifier))
	if _, ok := m.strategies[name]; !ok {
		return fmt.Errorf("strategy %q not registered", spec.Strategy.Identifier)
	}

	m.mu.Lock()
	if _, exists := m.specs[spec.ID]; exists && !allowReplace {
		m.mu.Unlock()
		return ErrInstanceExists
	}
	strategy, hash, _ := revisionSignatureForSpec(*spec)
	m.ensureRevisionUsageLocked(strategy, hash)
	m.specs[spec.ID] = cloneSpec(*spec)
	m.mu.Unlock()

	m.persistStrategy(spec.ID)
	return nil
}

// Start starts a lambda instance by ID.
func (m *Manager) Start(ctx context.Context, id string) error {
	spec, err := m.specForID(id)
	if err != nil {
		return err
	}
	m.mu.Lock()
	if _, running := m.instances[spec.ID]; running {
		m.mu.Unlock()
		return ErrInstanceAlreadyRunning
	}
	m.mu.Unlock()

	_, _, _, err = m.launch(ctx, spec, true)
	return err
}

func (m *Manager) launch(ctx context.Context, spec config.LambdaSpec, registerNow bool) (*core.BaseLambda, []string, []dispatcher.RouteDeclaration, error) {
	providers := spec.Providers
	if len(providers) == 0 {
		return nil, nil, nil, fmt.Errorf("strategy %s: providers required", spec.ID)
	}
	resolvedProviders := make([]string, 0, len(providers))
	for _, name := range providers {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := m.providers.Provider(name); !ok {
			return nil, nil, nil, fmt.Errorf("provider %q unavailable", name)
		}
		resolvedProviders = append(resolvedProviders, name)
	}
	if len(resolvedProviders) == 0 {
		return nil, nil, nil, fmt.Errorf("strategy %s: no valid providers resolved", spec.ID)
	}

	strategy, err := m.buildStrategy(spec.Strategy)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("strategy %s: %w", spec.ID, err)
	}
	if strategy != nil && len(resolvedProviders) > 1 && !strategy.WantsCrossProviderEvents() {
		return nil, nil, nil, fmt.Errorf("strategy %s does not support cross-provider feeds", spec.Strategy.Identifier)
	}

	routes := buildRouteDeclarations(strategy, spec)
	var registered bool
	if registerNow && m.registrar != nil && len(routes) > 0 {
		if err := m.registrar.RegisterLambda(ctx, spec.ID, resolvedProviders, routes); err != nil {
			return nil, nil, nil, fmt.Errorf("strategy %s: register routes: %w", spec.ID, err)
		}
		registered = true
	}

	orderRouter := &providerOrderRouter{catalog: m.providers}
	dryRun := true
	if raw, ok := spec.Strategy.Config["dry_run"]; ok {
		if val, ok := raw.(bool); ok {
			dryRun = val
		}
	}
	baseCfg := core.Config{Providers: resolvedProviders, ProviderSymbols: spec.ProviderSymbolMap(), DryRun: dryRun}
	base := core.NewBaseLambda(spec.ID, baseCfg, m.bus, orderRouter, m.pools, strategy, m.riskManager, m.orderStore)
	bindStrategy(strategy, base, m.logger)

	runCtx, cancel := context.WithCancel(m.parentContext())
	errs, err := base.Start(runCtx)
	if err != nil {
		cancel()
		if registered && m.registrar != nil {
			_ = m.registrar.UnregisterLambda(ctx, spec.ID)
		}
		return nil, nil, nil, fmt.Errorf("start strategy %s: %w", spec.ID, err)
	}

	m.mu.Lock()
	revisionKey := m.markInstanceRunningLocked(spec, spec.ID)
	m.instances[spec.ID] = &lambdaInstance{base: base, cancel: cancel, errs: errs, strat: strategy, revKey: revisionKey}
	m.mu.Unlock()

	go m.observe(runCtx, spec.ID, errs, strategy)
	m.persistStrategy(spec.ID)
	return base, resolvedProviders, routes, nil
}

func (m *Manager) specForID(id string) (config.LambdaSpec, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		var empty config.LambdaSpec
		return empty, ErrInstanceNotFound
	}
	m.mu.RLock()
	spec, ok := m.specs[id]
	m.mu.RUnlock()
	if !ok {
		var empty config.LambdaSpec
		return empty, ErrInstanceNotFound
	}
	return cloneSpec(spec), nil
}

// Stop stops a running lambda instance by ID.
func (m *Manager) Stop(id string) error {
	id = strings.TrimSpace(id)
	m.mu.Lock()
	inst, running := m.instances[id]
	if !running {
		if _, exists := m.specs[id]; !exists {
			m.mu.Unlock()
			return ErrInstanceNotFound
		}
		m.mu.Unlock()
		return ErrInstanceNotRunning
	}
	revKey := inst.revKey
	delete(m.instances, id)
	m.markInstanceStoppedLocked(revKey, id)
	m.mu.Unlock()

	inst.cancel()
	if m.registrar != nil {
		_ = m.registrar.UnregisterLambda(context.Background(), id)
	}
	closeStrategy(inst.strat)
	m.persistStrategy(id)
	return nil
}

// Remove removes a lambda instance by ID after stopping it.
func (m *Manager) Remove(id string) error {
	err := m.Stop(id)
	if err != nil && !errors.Is(err, ErrInstanceNotRunning) {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.specs[id]; !ok {
		return ErrInstanceNotFound
	}
	delete(m.specs, id)
	delete(m.baseline, strings.ToLower(strings.TrimSpace(id)))
	delete(m.dynamicInstances, strings.ToLower(strings.TrimSpace(id)))
	m.deleteStrategy(id)
	return nil
}

// Update updates an existing lambda instance with new configuration.
func (m *Manager) Update(ctx context.Context, spec config.LambdaSpec) error {
	spec = sanitizeSpec(spec)
	if spec.ID == "" {
		return ErrInstanceNotFound
	}

	m.mu.RLock()
	current, ok := m.specs[spec.ID]
	_, wasRunning := m.instances[spec.ID]
	m.mu.RUnlock()
	if !ok {
		return ErrInstanceNotFound
	}
	if !equalStringSlices(current.Providers, spec.Providers) {
		return fmt.Errorf("providers are immutable for %s", spec.ID)
	}
	if !equalProviderSymbols(current.ProviderSymbols, spec.ProviderSymbols) {
		return fmt.Errorf("scope assignments are immutable for %s", spec.ID)
	}
	if current.Strategy.Identifier != spec.Strategy.Identifier {
		return fmt.Errorf("strategy is immutable for %s", spec.ID)
	}
	if err := m.ensureSpec(&spec, true); err != nil {
		return err
	}

	if err := m.Stop(spec.ID); err != nil && !errors.Is(err, ErrInstanceNotRunning) {
		return err
	}
	startAfterUpdate := wasRunning
	if startAfterUpdate {
		if _, _, _, err := m.launch(ctx, spec, true); err != nil {
			return err
		}
	}
	return nil
}

// InstanceSummary provides a flattened overview of a lambda instance.
type InstanceSummary struct {
	ID                 string                `json:"id"`
	StrategyIdentifier string                `json:"strategyIdentifier"`
	StrategyTag        string                `json:"strategyTag,omitempty"`
	StrategyHash       string                `json:"strategyHash,omitempty"`
	StrategySelector   string                `json:"strategySelector,omitempty"`
	Providers          []string              `json:"providers"`
	AggregatedSymbols  []string              `json:"aggregatedSymbols"`
	Running            bool                  `json:"running"`
	Usage              *RevisionUsageSummary `json:"usage,omitempty"`
}

// InstanceSnapshot captures the detailed state of a lambda instance.
type InstanceSnapshot struct {
	ID                string                            `json:"id"`
	Strategy          config.LambdaStrategySpec         `json:"strategy"`
	Providers         []string                          `json:"providers"`
	ProviderSymbols   map[string]config.ProviderSymbols `json:"scope"`
	AggregatedSymbols []string                          `json:"aggregatedSymbols"`
	Running           bool                              `json:"running"`
	Usage             *RevisionUsageSummary             `json:"usage,omitempty"`
}

// Instances returns summaries of all lambda instances.
func (m *Manager) Instances() []InstanceSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]InstanceSummary, 0, len(m.specs))
	for id, spec := range m.specs {
		_, running := m.instances[id]
		usage := m.revisionUsageSummaryLocked(spec)
		out = append(out, summaryOf(spec, running, usage))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Instance returns a snapshot of a specific lambda instance by ID.
func (m *Manager) Instance(id string) (InstanceSnapshot, bool) {
	spec, err := m.specForID(id)
	if err != nil {
		return InstanceSnapshot{
			ID: "",
			Strategy: config.LambdaStrategySpec{
				Identifier: "",
				Selector:   "",
				Tag:        "",
				Hash:       "",
				Config:     map[string]any{},
			},
			Providers:         []string{},
			ProviderSymbols:   map[string]config.ProviderSymbols{},
			AggregatedSymbols: []string{},
			Running:           false,
			Usage:             nil,
		}, false
	}
	m.mu.RLock()
	_, running := m.instances[spec.ID]
	usage := m.revisionUsageSummaryLocked(spec)
	m.mu.RUnlock()
	return snapshotOf(spec, running, usage), true
}

// IsBaseline reports whether the instance originated from the baseline manifest.
func (m *Manager) IsBaseline(id string) bool {
	return m.isBaselineInstance(id)
}

// IsDynamic reports whether the instance was created dynamically via control APIs.
func (m *Manager) IsDynamic(id string) bool {
	return m.isDynamicInstance(id)
}

func summaryOf(spec config.LambdaSpec, running bool, usage *RevisionUsageSummary) InstanceSummary {
	providers := append([]string(nil), spec.Providers...)
	aggregated := spec.AllSymbols()
	return InstanceSummary{
		ID:                 spec.ID,
		StrategyIdentifier: spec.Strategy.Identifier,
		StrategyTag:        spec.Strategy.Tag,
		StrategyHash:       spec.Strategy.Hash,
		StrategySelector:   spec.Strategy.Selector,
		Providers:          providers,
		AggregatedSymbols:  aggregated,
		Running:            running,
		Usage:              cloneRevisionUsage(usage),
	}
}

func snapshotOf(spec config.LambdaSpec, running bool, usage *RevisionUsageSummary) InstanceSnapshot {
	strategyConfig := copyMap(spec.Strategy.Config)
	providers := append([]string(nil), spec.Providers...)
	assignments := cloneProviderSymbols(spec.ProviderSymbols)
	aggregated := spec.AllSymbols()
	return InstanceSnapshot{
		ID: spec.ID,
		Strategy: config.LambdaStrategySpec{
			Identifier: spec.Strategy.Identifier,
			Config:     strategyConfig,
			Selector:   spec.Strategy.Selector,
			Tag:        spec.Strategy.Tag,
			Hash:       spec.Strategy.Hash,
		},
		Providers:         providers,
		ProviderSymbols:   assignments,
		AggregatedSymbols: aggregated,
		Running:           running,
		Usage:             cloneRevisionUsage(usage),
	}
}

func (m *Manager) observe(ctx context.Context, id string, errs <-chan error, strat core.TradingStrategy) {
	defer closeStrategy(strat)
	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errs:
			if !ok {
				return
			}
			if err != nil {
				m.logger.Printf("strategy %s: %v", id, err)
			}
		}
	}
}

type providerOrderRouter struct {
	catalog ProviderCatalog
}

func (r *providerOrderRouter) SubmitOrder(ctx context.Context, req schema.OrderRequest) error {
	if r == nil || r.catalog == nil {
		return fmt.Errorf("order router not configured")
	}
	providerName := strings.TrimSpace(req.Provider)
	if providerName == "" {
		return fmt.Errorf("order provider required")
	}
	inst, ok := r.catalog.Provider(providerName)
	if !ok {
		return fmt.Errorf("provider %q unavailable", providerName)
	}
	if err := inst.SubmitOrder(ctx, req); err != nil {
		return fmt.Errorf("submit order to provider %q: %w", providerName, err)
	}
	return nil
}

func closeStrategy(strat core.TradingStrategy) {
	if strat == nil {
		return
	}
	type closer interface {
		Close()
	}
	switch s := strat.(type) {
	case closer:
		s.Close()
	case io.Closer:
		_ = s.Close()
	}
}

func (m *Manager) buildStrategy(spec config.LambdaStrategySpec) (core.TradingStrategy, error) {
	name := strings.ToLower(strings.TrimSpace(spec.Identifier))
	if name == "" {
		return nil, fmt.Errorf("strategy identifier required")
	}
	if spec.Hash != "" && m.jsLoader != nil {
		module, err := m.jsLoader.Get(spec.Hash)
		if err != nil {
			if errors.Is(err, js.ErrModuleNotFound) {
				return nil, fmt.Errorf("strategy %s: revision %s unavailable", name, spec.Hash)
			}
			return nil, fmt.Errorf("strategy %s: %w", name, err)
		}
		if module == nil {
			return nil, fmt.Errorf("strategy %s: revision %s unavailable", name, spec.Hash)
		}
		if !strings.EqualFold(module.Name, name) {
			return nil, fmt.Errorf("strategy %s: revision %s belongs to %s", name, spec.Hash, module.Name)
		}
		strategy, buildErr := js.NewStrategy(module, spec.Config, m.logger)
		if buildErr != nil {
			return nil, fmt.Errorf("strategy %s: %w", name, buildErr)
		}
		return strategy, nil
	}
	def, ok := m.strategies[name]
	if !ok {
		return nil, fmt.Errorf("strategy %q not registered", spec.Identifier)
	}
	return def.factory(copyMap(spec.Config))
}

func sanitizeSpec(spec config.LambdaSpec) config.LambdaSpec {
	spec.ID = strings.TrimSpace(spec.ID)
	spec.Strategy.Normalize()
	spec.RefreshProviders()
	spec.Providers = normalizeProviderList(spec.Providers)
	if spec.ProviderSymbols == nil {
		spec.ProviderSymbols = make(map[string]config.ProviderSymbols)
	}
	return spec
}

func cloneSpec(spec config.LambdaSpec) config.LambdaSpec {
	clone := spec
	clone.Strategy.Config = copyMap(spec.Strategy.Config)
	clone.Strategy.Selector = spec.Strategy.Selector
	clone.Strategy.Tag = spec.Strategy.Tag
	clone.Strategy.Hash = spec.Strategy.Hash
	clone.Providers = append([]string(nil), spec.Providers...)
	clone.ProviderSymbols = cloneProviderSymbols(spec.ProviderSymbols)
	return clone
}

func cloneProviderSymbols(src map[string]config.ProviderSymbols) map[string]config.ProviderSymbols {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]config.ProviderSymbols, len(src))
	for name, assignment := range src {
		cloned := config.ProviderSymbols{
			Symbols: append([]string(nil), assignment.Symbols...),
		}
		dst[name] = cloned
	}
	return dst
}

func copyMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func cloneSymbolMap(src map[string]config.ProviderSymbols) map[string][]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string][]string, len(src))
	for provider, assignment := range src {
		out[provider] = append([]string(nil), assignment.Symbols...)
	}
	return out
}

func buildProviderSymbols(symbols map[string][]string) map[string]config.ProviderSymbols {
	if len(symbols) == 0 {
		return make(map[string]config.ProviderSymbols)
	}
	out := make(map[string]config.ProviderSymbols, len(symbols))
	for provider, vals := range symbols {
		out[provider] = config.ProviderSymbols{Symbols: append([]string(nil), vals...)}
	}
	return out
}

func copyStringSet(src map[string]struct{}) map[string]struct{} {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]struct{}, len(src))
	for key := range src {
		dst[key] = struct{}{}
	}
	return dst
}

func providerInstrumentField(provider string) string {
	trimmed := strings.TrimSpace(provider)
	if trimmed == "" {
		return "instrument"
	}
	return "instrument@" + strings.ToLower(trimmed)
}

func normalizeProviderList(providers []string) []string {
	if len(providers) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(providers))
	out := make([]string, 0, len(providers))
	for _, raw := range providers {
		candidate := strings.TrimSpace(raw)
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalProviderSymbols(a, b map[string]config.ProviderSymbols) bool {
	if len(a) != len(b) {
		return false
	}
	for name, assignmentA := range a {
		assignmentB, ok := b[name]
		if !ok {
			return false
		}
		if len(assignmentA.Symbols) != len(assignmentB.Symbols) {
			return false
		}
		for i := range assignmentA.Symbols {
			if assignmentA.Symbols[i] != assignmentB.Symbols[i] {
				return false
			}
		}
	}
	return true
}

func buildRouteDeclarations(strategy core.TradingStrategy, spec config.LambdaSpec) []dispatcher.RouteDeclaration {
	if strategy == nil {
		return nil
	}
	events := strategy.SubscribedEvents()
	if len(events) == 0 {
		return nil
	}
	routes := make([]dispatcher.RouteDeclaration, 0, len(events))
	providerSymbols := spec.ProviderSymbolMap()
	allSymbols := spec.AllSymbols()
	baseCurrency := ""
	quoteCurrency := ""
	if len(allSymbols) == 1 {
		if base, quote, err := schema.InstrumentCurrencies(allSymbols[0]); err == nil {
			baseCurrency = base
			quoteCurrency = quote
		}
	}
	baseCurrency = strings.ToUpper(strings.TrimSpace(baseCurrency))
	quoteCurrency = strings.ToUpper(strings.TrimSpace(quoteCurrency))

	seenCurrencies := make(map[string]struct{}, 2)
	seenRoutes := make(map[schema.RouteType]struct{})
	for _, evtType := range events {
		routesForEvent := schema.RoutesForEvent(evtType)
		for _, routeName := range routesForEvent {
			routeName = schema.NormalizeRouteType(routeName)
			if err := routeName.Validate(); err != nil {
				continue
			}
			if routeName == schema.RouteTypeAccountBalance {
				candidates := []string{baseCurrency, quoteCurrency}
				for _, currency := range candidates {
					currency = strings.ToUpper(strings.TrimSpace(currency))
					if currency == "" {
						continue
					}
					if _, ok := seenCurrencies[currency]; ok {
						continue
					}
					seenCurrencies[currency] = struct{}{}
					routeFilters := map[string]any{"currency": currency}
					routes = append(routes, dispatcher.RouteDeclaration{
						Type:    routeName,
						Filters: copyMap(routeFilters),
					})
				}
				continue
			}
			if _, ok := seenRoutes[routeName]; ok {
				continue
			}
			seenRoutes[routeName] = struct{}{}
			routeFilters := make(map[string]any)
			if len(allSymbols) > 0 {
				routeFilters["instrument"] = allSymbols
			}
			for provider, symbols := range providerSymbols {
				if len(symbols) == 0 {
					continue
				}
				key := providerInstrumentField(provider)
				routeFilters[key] = symbols
			}
			routes = append(routes, dispatcher.RouteDeclaration{
				Type:    routeName,
				Filters: copyMap(routeFilters),
			})
		}
	}
	return routes
}

func bindStrategy(strategy core.TradingStrategy, base *core.BaseLambda, _ *log.Logger) {
	switch s := strategy.(type) {
	case *js.Strategy:
		s.Attach(base)
	}
}
