package pipeline

import (
	"strings"
	"sync"
)

type snapshotCache struct {
	mu      sync.RWMutex
	latest  map[string]map[string]ClientEvent
	enabled bool
}

func newSnapshotCache(enabled bool) *snapshotCache {
	if !enabled {
		return nil
	}
	return &snapshotCache{
		latest:  make(map[string]map[string]ClientEvent),
		enabled: true,
	}
}

func (c *snapshotCache) Update(evt ClientEvent) {
	if c == nil || !c.enabled {
		return
	}
	symbol := normalizeSymbol(evt.Symbol)
	if symbol == "" {
		return
	}
	eventType := payloadIdentity(evt.Payload)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.latest[eventType]; !ok {
		c.latest[eventType] = make(map[string]ClientEvent)
	}
	c.latest[eventType][symbol] = evt
}

func (c *snapshotCache) Get(eventType string, symbol string) (ClientEvent, bool) {
	if c == nil || !c.enabled {
		return ClientEvent{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	symbol = normalizeSymbol(symbol)
	if symbol == "" {
		return ClientEvent{}, false
	}
	evtMap, ok := c.latest[eventType]
	if !ok {
		return ClientEvent{}, false
	}
	evt, ok := evtMap[symbol]
	return evt, ok
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
