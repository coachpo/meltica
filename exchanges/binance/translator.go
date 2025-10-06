package binance

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/coachpo/meltica/core"
)

type lazySymbolTranslator struct {
	once              sync.Once
	loader            func() (registrySnapshot, error)
	canonicalToNative map[string]string
	nativeToCanonical map[string]string
	loadErr           error
}

func (t *lazySymbolTranslator) ensureLoaded() error {
	t.once.Do(func() {
		snapshot, err := t.loader()
		if err != nil {
			t.loadErr = err
			return
		}
		canonicalToNative := make(map[string]string, len(snapshot.nativeToCanonical))
		nativeToCanonical := make(map[string]string, len(snapshot.nativeToCanonical))

		for _, market := range marketsOrAll() {
			entries, ok := snapshot.marketCanonicalMap[market]
			if !ok {
				continue
			}
			for canonical, native := range entries {
				canonical = strings.ToUpper(strings.TrimSpace(canonical))
				native = strings.ToUpper(strings.TrimSpace(native))
				if canonical == "" || native == "" {
					continue
				}
				if _, exists := canonicalToNative[canonical]; !exists {
					canonicalToNative[canonical] = native
				}
				nativeToCanonical[native] = canonical
			}
		}
		for native, canonical := range snapshot.nativeToCanonical {
			native = strings.ToUpper(strings.TrimSpace(native))
			canonical = strings.ToUpper(strings.TrimSpace(canonical))
			if native == "" || canonical == "" {
				continue
			}
			nativeToCanonical[native] = canonical
			if _, exists := canonicalToNative[canonical]; !exists {
				canonicalToNative[canonical] = native
			}
		}
		t.canonicalToNative = canonicalToNative
		t.nativeToCanonical = nativeToCanonical
		t.loader = nil
	})
	return t.loadErr
}

func (t *lazySymbolTranslator) Native(canonical string) (string, error) {
	if err := t.ensureLoaded(); err != nil {
		return "", err
	}
	canonical = strings.ToUpper(strings.TrimSpace(canonical))
	if canonical == "" {
		return "", fmt.Errorf("translator: empty canonical symbol")
	}
	if native, ok := t.canonicalToNative[canonical]; ok {
		return native, nil
	}
	return "", fmt.Errorf("translator: unsupported canonical symbol %q", canonical)
}

func (t *lazySymbolTranslator) Canonical(native string) (string, error) {
	if err := t.ensureLoaded(); err != nil {
		return "", err
	}
	native = strings.ToUpper(strings.TrimSpace(native))
	if native == "" {
		return "", fmt.Errorf("translator: empty native symbol")
	}
	if canonical, ok := t.nativeToCanonical[native]; ok {
		return canonical, nil
	}
	return "", fmt.Errorf("translator: unsupported native symbol %q", native)
}

// NewSymbolTranslator returns a lazily-initialised translator backed by the exchange symbol registry.
func NewSymbolTranslator(x *Exchange) core.SymbolTranslator {
	translator := &lazySymbolTranslator{}
	exchangeRef := x
	translator.loader = func() (registrySnapshot, error) {
		ex := exchangeRef
		if ex == nil {
			return registrySnapshot{}, fmt.Errorf("translator: exchange unavailable")
		}
		for _, market := range marketsOrAll() {
			if err := ex.ensureMarketSymbols(context.Background(), market); err != nil {
				translator.loadErr = err
				return registrySnapshot{}, err
			}
		}
		snapshot, err := ex.symbols.snapshot(context.Background())
		if err != nil {
			translator.loadErr = err
			return registrySnapshot{}, err
		}
		exchangeRef = nil
		return snapshot, nil
	}
	return translator
}
