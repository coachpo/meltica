// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"context"
	"errors"
	"strings"

	"github.com/coachpo/meltica/internal/domain/strategystore"
	"github.com/coachpo/meltica/internal/infra/config"
)

func (m *Manager) strategySnapshot(id string) (strategystore.Snapshot, bool) {
	m.mu.RLock()
	spec, ok := m.specs[id]
	if !ok {
		m.mu.RUnlock()
		var empty strategystore.Snapshot
		return empty, false
	}
	_, running := m.instances[id]
	m.mu.RUnlock()

	snapshot := strategystore.Snapshot{
		ID:              spec.ID,
		Strategy:        strategystore.Strategy{Identifier: spec.Strategy.Identifier, Selector: spec.Strategy.Selector, Tag: spec.Strategy.Tag, Hash: spec.Strategy.Hash, Config: copyMap(spec.Strategy.Config)},
		Providers:       append([]string(nil), spec.Providers...),
		ProviderSymbols: cloneSymbolMap(spec.ProviderSymbols),
		Running:         running,
		Dynamic:         m.isDynamicInstance(spec.ID),
		Baseline:        m.isBaselineInstance(spec.ID),
		Metadata:        map[string]any{},
		UpdatedAt:       m.clock(),
	}
	return snapshot, true
}

func (m *Manager) persistStrategy(id string) {
	if m == nil || m.strategyStore == nil {
		return
	}
	snapshot, ok := m.strategySnapshot(id)
	if !ok {
		return
	}
	ctx := m.parentContext()
	if err := m.strategyStore.Save(ctx, snapshot); err != nil && m.logger != nil {
		m.logger.Printf("strategy/%s: persist failed: %v", id, err)
	}
}

func (m *Manager) deleteStrategy(id string) {
	if m == nil || m.strategyStore == nil {
		return
	}
	ctx := m.parentContext()
	if err := m.strategyStore.Delete(ctx, id); err != nil && m.logger != nil {
		m.logger.Printf("strategy/%s: delete snapshot failed: %v", id, err)
	}
}

func (m *Manager) restoreStrategySnapshot(ctx context.Context, snapshot strategystore.Snapshot) {
	if snapshot.ID == "" {
		return
	}
	spec := specFromSnapshot(snapshot)
	if err := m.ensureSpec(&spec, true); err != nil {
		if m.logger != nil {
			m.logger.Printf("strategy/%s: restore spec failed: %v", snapshot.ID, err)
		}
		return
	}
	m.setBaselineInstance(snapshot.ID, snapshot.Baseline)
	m.setDynamicInstance(snapshot.ID, snapshot.Dynamic)
	if snapshot.Running {
		if err := m.Start(ctx, snapshot.ID); err != nil && m.logger != nil {
			if !errors.Is(err, ErrInstanceAlreadyRunning) {
				m.logger.Printf("strategy/%s: restore start failed: %v", snapshot.ID, err)
			}
		}
	}
}

// RestoreSnapshot rehydrates a strategy instance snapshot without failing the manager on errors.
func (m *Manager) RestoreSnapshot(ctx context.Context, snapshot strategystore.Snapshot) {
	if m == nil {
		return
	}
	m.restoreStrategySnapshot(ctx, snapshot)
}

func specFromSnapshot(snapshot strategystore.Snapshot) config.LambdaSpec {
	spec := config.LambdaSpec{
		ID:              snapshot.ID,
		Strategy:        config.LambdaStrategySpec{Identifier: snapshot.Strategy.Identifier, Config: copyMap(snapshot.Strategy.Config), Selector: snapshot.Strategy.Selector, Tag: snapshot.Strategy.Tag, Hash: snapshot.Strategy.Hash},
		Providers:       append([]string(nil), snapshot.Providers...),
		ProviderSymbols: buildProviderSymbols(snapshot.ProviderSymbols),
	}
	if len(snapshot.Providers) > 0 && len(spec.ProviderSymbols) == 0 {
		spec.Providers = append([]string(nil), snapshot.Providers...)
	} else {
		spec.RefreshProviders()
	}
	return sanitizeSpec(spec)
}

func (m *Manager) setBaselineInstance(id string, baseline bool) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if baseline {
		m.baseline[strings.ToLower(strings.TrimSpace(id))] = struct{}{}
	} else {
		delete(m.baseline, strings.ToLower(strings.TrimSpace(id)))
	}
}

func (m *Manager) isBaselineInstance(id string) bool {
	if m == nil || id == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.baseline[strings.ToLower(strings.TrimSpace(id))]
	return ok
}

func (m *Manager) setDynamicInstance(id string, dynamic bool) {
	if m == nil || id == "" {
		return
	}
	key := strings.ToLower(strings.TrimSpace(id))
	m.mu.Lock()
	defer m.mu.Unlock()
	if dynamic {
		m.dynamicInstances[key] = struct{}{}
	} else {
		delete(m.dynamicInstances, key)
	}
}

func (m *Manager) isDynamicInstance(id string) bool {
	if m == nil || id == "" {
		return false
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.dynamicInstances[strings.ToLower(strings.TrimSpace(id))]
	return ok
}
