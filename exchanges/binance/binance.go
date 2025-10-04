package binance

import (
	"github.com/coachpo/meltica/exchanges/binance/exchange"
)

type Exchange = exchange.Exchange

func New(apiKey, secret string) (*Exchange, error) {
	return exchange.New(apiKey, secret)
}
