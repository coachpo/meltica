package plugin

import (
	"sort"
	"sync"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
)

// Registration captures the components required to register an exchange adapter.
type Registration struct {
	Name             core.ExchangeName
	Build            coreregistry.ExchangeFactory
	SymbolTranslator func(core.Exchange) (core.SymbolTranslator, error)
	TopicTranslator  func(core.Exchange) (core.TopicTranslator, error)
	Capabilities     core.ExchangeCapabilities
	ProtocolVersion  string
	Metadata         map[string]string
}

// Summary exposes lightweight registration details for discovery tooling.
type Summary struct {
	Name            core.ExchangeName
	Capabilities    core.ExchangeCapabilities
	ProtocolVersion string
	Metadata        map[string]string
}

var (
	summaryMu sync.RWMutex
	summaries = make(map[core.ExchangeName]Summary)
)

// Register wires the exchange factory and supporting translators into shared registries.
func Register(reg Registration) {
	if reg.Name == "" {
		panic("plugin: registration requires name")
	}
	if reg.Build == nil {
		panic("plugin: registration requires build function")
	}
	coreregistry.Register(reg.Name, func(cfg coreregistry.Config) (core.Exchange, error) {
		exchange, err := reg.Build(cfg)
		if err != nil {
			return nil, err
		}
		if reg.SymbolTranslator != nil {
			translator, err := reg.SymbolTranslator(exchange)
			if err != nil {
				return nil, err
			}
			if translator != nil {
				core.RegisterSymbolTranslator(reg.Name, translator)
			}
		}
		if reg.TopicTranslator != nil {
			translator, err := reg.TopicTranslator(exchange)
			if err != nil {
				return nil, err
			}
			if translator != nil {
				core.RegisterTopicTranslator(reg.Name, translator)
			}
		}
		return exchange, nil
	})
	summaryMu.Lock()
	summaries[reg.Name] = Summary{
		Name:            reg.Name,
		Capabilities:    reg.Capabilities,
		ProtocolVersion: reg.ProtocolVersion,
		Metadata:        cloneStringMap(reg.Metadata),
	}
	summaryMu.Unlock()
}

// Summaries reports registered exchanges for discovery tools.
func Summaries() []Summary {
	summaryMu.RLock()
	defer summaryMu.RUnlock()
	items := make([]Summary, 0, len(summaries))
	for _, summary := range summaries {
		items = append(items, Summary{
			Name:            summary.Name,
			Capabilities:    summary.Capabilities,
			ProtocolVersion: summary.ProtocolVersion,
			Metadata:        cloneStringMap(summary.Metadata),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// Reset clears discovery metadata. Intended for tests.
func Reset() {
	summaryMu.Lock()
	defer summaryMu.Unlock()
	summaries = make(map[core.ExchangeName]Summary)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
