package binance

import (
	"github.com/coachpo/meltica/providers/binance/provider"
)

type Provider = provider.Provider

func New(apiKey, secret string) (*Provider, error) {
	return provider.New(apiKey, secret)
}
