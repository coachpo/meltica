package core

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var canonicalSymbolPattern = regexp.MustCompile(`^[A-Z0-9]+-[A-Z0-9]+$`)

type symbolEntry struct {
	canonical string
	aliases   []string
}

type symbolRegistry struct {
	mu        sync.RWMutex
	canonical map[string]symbolEntry
	alias     map[string]string
}

func newSymbolRegistry() *symbolRegistry {
	return &symbolRegistry{
		canonical: make(map[string]symbolEntry),
		alias:     make(map[string]string),
	}
}

var registry = newSymbolRegistry()

// SymbolDefinition declares a canonical symbol and its known aliases.
type SymbolDefinition struct {
	Canonical string
	Aliases   []string
}

// RegisterSymbol adds the canonical symbol and aliases to the shared registry.
func RegisterSymbol(def SymbolDefinition) error {
	return registry.register(def)
}

// RegisterSymbols bulk registers multiple symbol definitions.
func RegisterSymbols(defs ...SymbolDefinition) error {
	for _, def := range defs {
		if err := RegisterSymbol(def); err != nil {
			return err
		}
	}
	return nil
}

// CanonicalSymbolFor resolves the provided symbol or alias to its canonical representation.
func CanonicalSymbolFor(symbol string) (string, error) {
	canonical, ok := registry.resolve(symbol)
	if !ok {
		return "", fmt.Errorf("core: symbol %s not registered", strings.TrimSpace(symbol))
	}
	return canonical, nil
}

// SymbolAliases returns the registered aliases for the canonical symbol.
func SymbolAliases(symbol string) []string {
	return registry.aliases(symbol)
}

// IsCanonicalSymbol reports whether the string matches the canonical symbol pattern.
func IsCanonicalSymbol(symbol string) bool {
	trimmed := strings.ToUpper(strings.TrimSpace(symbol))
	return canonicalSymbolPattern.MatchString(trimmed)
}

// ParseCanonicalSymbol splits a canonical symbol into its base and quote components.
func ParseCanonicalSymbol(symbol string) (string, string, error) {
	canonical := strings.ToUpper(strings.TrimSpace(symbol))
	if !canonicalSymbolPattern.MatchString(canonical) {
		return "", "", fmt.Errorf("core: invalid canonical symbol %q", symbol)
	}
	parts := strings.SplitN(canonical, "-", 2)
	return parts[0], parts[1], nil
}

func (r *symbolRegistry) register(def SymbolDefinition) error {
	canonical := strings.ToUpper(strings.TrimSpace(def.Canonical))
	if canonical == "" {
		return fmt.Errorf("core: canonical symbol cannot be empty")
	}
	if !canonicalSymbolPattern.MatchString(canonical) {
		return fmt.Errorf("core: canonical symbol %q is invalid", def.Canonical)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.canonical[canonical]; exists {
		return fmt.Errorf("core: canonical symbol %s already registered", canonical)
	}

	seen := make(map[string]struct{}, len(def.Aliases))
	aliases := make([]string, 0, len(def.Aliases))
	for _, alias := range def.Aliases {
		trimmed := strings.TrimSpace(alias)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToUpper(trimmed)
		if normalized == canonical {
			continue
		}
		if _, dup := seen[normalized]; dup {
			continue
		}
		if owner, exists := r.alias[normalized]; exists {
			return fmt.Errorf("core: alias %s already registered for %s", normalized, owner)
		}
		seen[normalized] = struct{}{}
		aliases = append(aliases, normalized)
	}

	r.canonical[canonical] = symbolEntry{canonical: canonical, aliases: aliases}
	for _, alias := range aliases {
		r.alias[alias] = canonical
	}
	return nil
}

func (r *symbolRegistry) resolve(symbol string) (string, bool) {
	key := strings.ToUpper(strings.TrimSpace(symbol))
	if key == "" {
		return "", false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.canonical[key]; ok {
		return key, true
	}
	if canonical, ok := r.alias[key]; ok {
		return canonical, true
	}
	return "", false
}

func (r *symbolRegistry) aliases(symbol string) []string {
	canonical := strings.ToUpper(strings.TrimSpace(symbol))
	r.mu.RLock()
	entry, ok := r.canonical[canonical]
	r.mu.RUnlock()
	if !ok {
		return nil
	}
	out := make([]string, len(entry.aliases))
	copy(out, entry.aliases)
	return out
}

// ExchangeName identifies a registered exchange implementation.
type ExchangeName string

// SymbolTranslator exposes canonical/native conversion helpers for an exchange.
type SymbolTranslator interface {
	Native(canonical string) (string, error)
	Canonical(native string) (string, error)
}

var (
	translatorMu sync.RWMutex
	translators  = make(map[ExchangeName]SymbolTranslator)
)

// RegisterSymbolTranslator associates the translator with the given exchange name.
func RegisterSymbolTranslator(name ExchangeName, translator SymbolTranslator) {
	if name == "" || translator == nil {
		return
	}
	translatorMu.Lock()
	defer translatorMu.Unlock()
	translators[name] = translator
}

// SymbolTranslatorFor returns the registered translator for the exchange.
func SymbolTranslatorFor(name ExchangeName) (SymbolTranslator, error) {
	translatorMu.RLock()
	translator, ok := translators[name]
	translatorMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("core: symbol translator for exchange %s not registered", name)
	}
	return translator, nil
}

// NativeSymbol resolves a canonical symbol to the exchange native representation.
func NativeSymbol(name ExchangeName, canonical string) (string, error) {
	translator, err := SymbolTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Native(canonical)
}

// CanonicalFromNative converts an exchange native symbol into canonical form.
func CanonicalFromNative(name ExchangeName, native string) (string, error) {
	translator, err := SymbolTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Canonical(native)
}

// CanonicalSymbol returns a canonical symbol form Base-Quote (e.g., BTC-USDT)
func CanonicalSymbol(base, quote string) string {
	return strings.ToUpper(base) + "-" + strings.ToUpper(quote)
}
