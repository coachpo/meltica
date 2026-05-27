// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"context"
	"sort"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/coachpo/meltica/internal/app/lambda/js"
	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/coachpo/meltica/internal/infra/telemetry"
)

const revisionKeySeparator = "\x1f"

type revisionUsage struct {
	strategy  string
	hash      string
	instances map[string]struct{}
	firstSeen time.Time
	lastSeen  time.Time
}

// RevisionUsageSummary captures runtime usage information for a strategy revision.
type RevisionUsageSummary struct {
	Strategy  string    `json:"strategy"`
	Hash      string    `json:"hash"`
	Instances []string  `json:"instances"`
	Count     int       `json:"count"`
	FirstSeen time.Time `json:"firstSeen"`
	LastSeen  time.Time `json:"lastSeen"`
	IsRunning bool      `json:"running"`
}

func newRevisionUsage(strategy, hash string) *revisionUsage {
	return &revisionUsage{
		strategy:  strategy,
		hash:      hash,
		instances: make(map[string]struct{}, 4),
		firstSeen: time.Time{},
		lastSeen:  time.Time{},
	}
}

func (u *revisionUsage) addInstance(id string, now time.Time) bool {
	if u == nil || id == "" {
		return false
	}
	if u.instances == nil {
		u.instances = make(map[string]struct{}, 4)
	}
	if _, exists := u.instances[id]; exists {
		u.lastSeen = now
		return false
	}
	u.instances[id] = struct{}{}
	if u.firstSeen.IsZero() {
		u.firstSeen = now
	}
	u.lastSeen = now
	return true
}

func (u *revisionUsage) removeInstance(id string, now time.Time) bool {
	if u == nil || id == "" {
		return false
	}
	if _, exists := u.instances[id]; !exists {
		return false
	}
	delete(u.instances, id)
	u.lastSeen = now
	return true
}

func (u *revisionUsage) snapshot() RevisionUsageSummary {
	if u == nil {
		var empty RevisionUsageSummary
		return empty
	}
	out := RevisionUsageSummary{
		Strategy:  u.strategy,
		Hash:      u.hash,
		Instances: nil,
		Count:     len(u.instances),
		FirstSeen: u.firstSeen,
		LastSeen:  u.lastSeen,
		IsRunning: len(u.instances) > 0,
	}
	if len(u.instances) > 0 {
		names := make([]string, 0, len(u.instances))
		for id := range u.instances {
			names = append(names, id)
		}
		sort.Strings(names)
		out.Instances = names
	}
	return out
}

func (m *Manager) observeRevisionUsage(_ context.Context, observer metric.Int64Observer) error {
	if m == nil || observer == nil {
		return nil
	}
	m.mu.RLock()
	snapshot := m.revisionUsageSnapshotLocked()
	m.mu.RUnlock()
	env := telemetry.Environment()
	for _, usage := range snapshot {
		observer.Observe(int64(usage.Count), metric.WithAttributes(
			attribute.String("environment", env),
			attribute.String("strategy", usage.Strategy),
			attribute.String("hash", usage.Hash),
		))
	}
	return nil
}

func (m *Manager) now() time.Time {
	if m == nil || m.clock == nil {
		return time.Now()
	}
	return m.clock()
}

func (m *Manager) ensureRevisionUsageLocked(strategy, hash string) *revisionUsage {
	key := buildRevisionKey(strategy, hash)
	usage, ok := m.revisionUsage[key]
	if !ok {
		usage = newRevisionUsage(strategy, hash)
		m.revisionUsage[key] = usage
	}
	return usage
}

func (m *Manager) markInstanceRunningLocked(spec config.LambdaSpec, instanceID string) string {
	strategy, hash, key := revisionSignatureForSpec(spec)
	usage := m.ensureRevisionUsageLocked(strategy, hash)
	if usage.addInstance(instanceID, m.now()) {
		m.recordRevisionLifecycle(usage, "start")
	}
	return key
}

func (m *Manager) markInstanceStoppedLocked(revisionKey, instanceID string) {
	if revisionKey == "" {
		return
	}
	usage, ok := m.revisionUsage[revisionKey]
	if !ok {
		return
	}
	if usage.removeInstance(instanceID, m.now()) {
		m.recordRevisionLifecycle(usage, "stop")
	}
}

func (m *Manager) revisionUsageSummary(spec config.LambdaSpec) *RevisionUsageSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.revisionUsageSummaryLocked(spec)
}

func (m *Manager) revisionUsageSummaryLocked(spec config.LambdaSpec) *RevisionUsageSummary {
	strategy, hash, key := revisionSignatureForSpec(spec)
	if usage, ok := m.revisionUsage[key]; ok {
		snapshot := usage.snapshot()
		return &snapshot
	}
	summary := RevisionUsageSummary{
		Strategy:  strategy,
		Hash:      hash,
		Instances: nil,
		Count:     0,
		FirstSeen: time.Time{},
		LastSeen:  time.Time{},
		IsRunning: false,
	}
	return &summary
}

func (m *Manager) recordRevisionLifecycle(usage *revisionUsage, action string) {
	if m == nil || usage == nil || m.revisionLifecycleMetric == nil {
		return
	}
	ctx := context.Background()
	m.revisionLifecycleMetric.Add(ctx, 1, metric.WithAttributes(
		attribute.String("environment", telemetry.Environment()),
		attribute.String("strategy", usage.strategy),
		attribute.String("hash", usage.hash),
		attribute.String("action", strings.ToLower(strings.TrimSpace(action))),
	))
}

func (m *Manager) revisionUsageSnapshotLocked() []RevisionUsageSummary {
	if m == nil {
		return nil
	}
	out := make([]RevisionUsageSummary, 0, len(m.revisionUsage))
	for _, usage := range m.revisionUsage {
		out = append(out, usage.snapshot())
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Strategy != out[j].Strategy {
			return out[i].Strategy < out[j].Strategy
		}
		return out[i].Hash < out[j].Hash
	})
	return out
}

// RevisionUsageSnapshot returns a stable view of revision usage for external consumers.
func (m *Manager) RevisionUsageSnapshot() []RevisionUsageSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snapshot := m.revisionUsageSnapshotLocked()
	if len(snapshot) == 0 {
		return nil
	}
	out := make([]RevisionUsageSummary, len(snapshot))
	copy(out, snapshot)
	return out
}

func revisionSignatureForSpec(spec config.LambdaSpec) (strategy string, hash string, key string) {
	strategy = normalizeStrategyName(spec.Strategy.Identifier)
	hash = normalizeRevisionHash(spec.Strategy.Hash)
	key = buildRevisionKey(strategy, hash)
	return
}

func normalizeStrategyName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeRevisionHash(hash string) string {
	return strings.TrimSpace(hash)
}

func buildRevisionKey(strategy, hash string) string {
	return strategy + revisionKeySeparator + hash
}

func normalizeSelector(selector string) string {
	return strings.ToLower(strings.TrimSpace(selector))
}

func cloneRevisionUsage(src *RevisionUsageSummary) *RevisionUsageSummary {
	if src == nil {
		return nil
	}
	cloned := *src
	if len(src.Instances) > 0 {
		cloned.Instances = append([]string(nil), src.Instances...)
	}
	return &cloned
}

func convertModuleUsageSnapshots(usages []RevisionUsageSummary) []js.ModuleUsageSnapshot {
	if len(usages) == 0 {
		return nil
	}
	out := make([]js.ModuleUsageSnapshot, 0, len(usages))
	for _, usage := range usages {
		snapshot := js.ModuleUsageSnapshot{
			Name:      usage.Strategy,
			Hash:      usage.Hash,
			Instances: append([]string(nil), usage.Instances...),
			Count:     usage.Count,
			FirstSeen: usage.FirstSeen,
			LastSeen:  usage.LastSeen,
		}
		out = append(out, snapshot)
	}
	return out
}

// RevisionUsageFor returns usage metadata for the specified strategy revision.
func (m *Manager) RevisionUsageFor(strategy, hash string) RevisionUsageSummary {
	spec := config.LambdaSpec{
		ID:              "",
		Strategy:        config.LambdaStrategySpec{Identifier: strategy, Config: nil, Selector: "", Tag: "", Hash: hash},
		ProviderSymbols: nil,
		Providers:       nil,
	}
	summary := m.revisionUsageSummary(spec)
	if summary == nil {
		return RevisionUsageSummary{
			Strategy:  normalizeStrategyName(strategy),
			Hash:      normalizeRevisionHash(hash),
			Instances: nil,
			Count:     0,
			FirstSeen: time.Time{},
			LastSeen:  time.Time{},
			IsRunning: false,
		}
	}
	cloned := cloneRevisionUsage(summary)
	if cloned == nil {
		var empty RevisionUsageSummary
		return empty
	}
	return *cloned
}

// RevisionInstances returns instance summaries pinned to the specified revision.
func (m *Manager) RevisionInstances(strategy, hash string, includeStopped bool) []InstanceSummary {
	normalizedStrategy := normalizeStrategyName(strategy)
	normalizedHash := normalizeRevisionHash(hash)

	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]InstanceSummary, 0)
	for id, spec := range m.specs {
		specStrategy := normalizeStrategyName(spec.Strategy.Identifier)
		specHash := normalizeRevisionHash(spec.Strategy.Hash)
		if specStrategy != normalizedStrategy || specHash != normalizedHash {
			continue
		}
		_, running := m.instances[id]
		if !running && !includeStopped {
			continue
		}
		usage := m.revisionUsageSummaryLocked(spec)
		out = append(out, summaryOf(spec, running, usage))
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out
}

// RevisionUsageDetail resolves a selector and returns usage metadata with matching instances.
func (m *Manager) RevisionUsageDetail(selector string, includeStopped bool) (RevisionUsageSummary, string, []InstanceSummary, error) {
	resolution, err := m.ResolveStrategySelector(selector)
	if err != nil {
		var empty RevisionUsageSummary
		return empty, "", nil, err
	}
	summary := m.RevisionUsageFor(resolution.Name, resolution.Hash)
	canonical := canonicalSelector(selector, resolution)
	if canonical == "" {
		canonical = selector
	}
	instances := m.RevisionInstances(resolution.Name, resolution.Hash, includeStopped)
	return summary, canonical, instances, nil
}
