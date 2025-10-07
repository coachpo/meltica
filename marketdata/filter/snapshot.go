package filter

import (
	"strings"
	"sync"
)

type snapshotCache struct {
	mu      sync.RWMutex
	latest  map[EventKind]map[string]EventEnvelope
	enabled bool
}

func newSnapshotCache(enabled bool) *snapshotCache {
	if !enabled {
		return nil
	}
	return &snapshotCache{
		latest:  make(map[EventKind]map[string]EventEnvelope),
		enabled: true,
	}
}

func (c *snapshotCache) Update(evt EventEnvelope) {
	if c == nil || !c.enabled {
		return
	}
	symbol := normalizeSymbol(evt.Symbol)
	if symbol == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.latest[evt.Kind]; !ok {
		c.latest[evt.Kind] = make(map[string]EventEnvelope)
	}
	c.latest[evt.Kind][symbol] = evt
}

func (c *snapshotCache) Get(kind EventKind, symbol string) (EventEnvelope, bool) {
	if c == nil || !c.enabled {
		return EventEnvelope{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	symbol = normalizeSymbol(symbol)
	if symbol == "" {
		return EventEnvelope{}, false
	}
	evtMap, ok := c.latest[kind]
	if !ok {
		return EventEnvelope{}, false
	}
	evt, ok := evtMap[symbol]
	return evt, ok
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
