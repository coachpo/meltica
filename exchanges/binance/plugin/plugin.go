package plugin

import (
	"sync"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
	"github.com/coachpo/meltica/exchanges/binance"
)

// Name exposes the registry identifier for Binance.
const Name core.ExchangeName = "binance"

var registerOnce sync.Once

// Register binds the Binance exchange factory and symbol mapper into the core registries.
// Consumers should invoke Register() during application bootstrap (or import the package for side effects).
func Register() {
	registerOnce.Do(func() {
		coreregistry.Register(Name, func(cfg coreregistry.Config) (core.Exchange, error) {
			x, err := binance.New(cfg.APIKey, cfg.APISecret)
			if err != nil {
				return nil, err
			}
			core.RegisterSymbolMapper(Name, &exchangeMapper{exchange: x})
			return x, nil
		})
	})
}

func init() {
	Register()
}

type exchangeMapper struct {
	exchange *binance.Exchange
}

func (m *exchangeMapper) NativeSymbol(canonical string) (string, error) {
	return m.exchange.NativeSymbol(canonical)
}

func (m *exchangeMapper) CanonicalSymbol(native string) (string, error) {
	return m.exchange.CanonicalSymbol(native)
}
