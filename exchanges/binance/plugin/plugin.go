package plugin

import (
	"fmt"
	"sync"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
	"github.com/coachpo/meltica/exchanges/binance"
	sharedplugin "github.com/coachpo/meltica/exchanges/shared/plugin"
)

// Name exposes the registry identifier for Binance.
const Name core.ExchangeName = "binance"

var registerOnce sync.Once

// Register binds the Binance exchange factory and symbol translator into the core registries.
// Consumers should invoke Register() during application bootstrap (or import the package for side effects).
func Register() {
	registerOnce.Do(func() {
		sharedplugin.Register(sharedplugin.Registration{
			Name: Name,
			Build: func(cfg coreregistry.Config) (core.Exchange, error) {
				return binance.New(cfg.APIKey, cfg.APISecret)
			},
			SymbolTranslator: func(exchange core.Exchange) (core.SymbolTranslator, error) {
				x, ok := exchange.(*binance.Exchange)
				if !ok {
					return nil, fmt.Errorf("binance: unexpected exchange type %T", exchange)
				}
				return binance.NewSymbolTranslator(x), nil
			},
			TopicTranslator: func(core.Exchange) (core.TopicTranslator, error) {
				return binance.NewTopicTranslator(), nil
			},
			Capabilities:    binance.Capabilities(),
			ProtocolVersion: core.ProtocolVersion,
			Metadata: map[string]string{
				"provider": "Binance",
			},
		})
	})
}

func init() {
	Register()
}
