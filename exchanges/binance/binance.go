package binance

import (
	"github.com/coachpo/meltica/exchanges/binance/provider"
)

type Exchange = provider.Exchange

func New(apiKey, secret string) (*Exchange, error) {
	return provider.New(apiKey, secret)
}
