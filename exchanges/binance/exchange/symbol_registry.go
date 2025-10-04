package exchange

import (
	"strings"
	"sync"

	"github.com/coachpo/meltica/core"
)

type symbolRegistry struct {
	mu                 sync.RWMutex
	nativeToCanonical  map[string]string
	marketCanonicalMap map[core.Market]map[string]string
	loaded             map[core.Market]bool
}

func newSymbolRegistry() *symbolRegistry {
	return &symbolRegistry{
		nativeToCanonical:  make(map[string]string),
		marketCanonicalMap: make(map[core.Market]map[string]string),
		loaded:             make(map[core.Market]bool),
	}
}

func (r *symbolRegistry) register(market core.Market, canonical, native string) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	native = strings.ToUpper(strings.TrimSpace(native))
	if canonical == "" || native == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.marketCanonicalMap[market]; !ok {
		r.marketCanonicalMap[market] = make(map[string]string)
	}
	r.marketCanonicalMap[market][canonical] = native
	r.nativeToCanonical[native] = canonical
}

type symbolEntry struct {
	canonical string
	native    string
}

func (r *symbolRegistry) replace(market core.Market, entries []symbolEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.marketCanonicalMap[market]; ok {
		for _, native := range r.marketCanonicalMap[market] {
			delete(r.nativeToCanonical, native)
		}
	}
	marketMap := make(map[string]string, len(entries))
	for _, entry := range entries {
		canonical := strings.ToUpper(strings.TrimSpace(entry.canonical))
		native := strings.ToUpper(strings.TrimSpace(entry.native))
		if canonical == "" || native == "" {
			continue
		}
		marketMap[canonical] = native
		r.nativeToCanonical[native] = canonical
	}
	r.marketCanonicalMap[market] = marketMap
	r.loaded[market] = true
}

func (r *symbolRegistry) isLoaded(market core.Market) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loaded[market]
}

func (r *symbolRegistry) canonical(native string) (string, bool) {
	native = strings.ToUpper(strings.TrimSpace(native))
	if native == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	canon, ok := r.nativeToCanonical[native]
	return canon, ok
}

func (r *symbolRegistry) native(market core.Market, canonical string) (string, bool) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	if canonical == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if marketMap, ok := r.marketCanonicalMap[market]; ok {
		if native, ok := marketMap[canonical]; ok {
			return native, true
		}
	}
	return "", false
}

func (r *symbolRegistry) nativeAny(canonical string) (string, bool) {
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	if canonical == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, marketMap := range r.marketCanonicalMap {
		if native, ok := marketMap[canonical]; ok {
			return native, true
		}
	}
	return "", false
}
