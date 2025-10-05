package binance

import (
	"github.com/coachpo/meltica/core"
	coreexchanges "github.com/coachpo/meltica/core/exchanges"
	"github.com/coachpo/meltica/exchanges/binance"
)

// Name exposes the registry identifier for Binance.
const Name core.ExchangeName = "binance"

func init() {
	coreexchanges.Register(Name, newBinance)
}

func newBinance(cfg coreexchanges.Config) (core.Exchange, error) {
	x, err := binance.New(cfg.APIKey, cfg.APISecret)
	if err != nil {
		return nil, err
	}
	core.RegisterSymbolMapper(Name, &mapper{exchange: x})
	return x, nil
}

// mapper adapts the Binance symbol registry to the core.SymbolMapper interface.
type mapper struct {
	exchange *binance.Exchange
}

func (m *mapper) NativeSymbol(canonical string) (string, error) {
	return m.exchange.NativeSymbol(canonical)
}

func (m *mapper) CanonicalSymbol(native string) (string, error) {
	return m.exchange.CanonicalSymbol(native)
}
