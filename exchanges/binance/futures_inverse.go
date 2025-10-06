package binance

import (
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
)

var inverseFuturesEndpoints = futuresEndpoints{
	api:          string(rest.InverseAPI),
	tickerPath:   "/dapi/v1/ticker/bookTicker",
	orderPath:    "/dapi/v1/order",
	positionPath: "/dapi/v1/positionRisk",
}

func newInverseFuturesAPI(x *Exchange) core.FuturesAPI {
	return newFuturesAPI(x, core.MarketInverseFutures, inverseFuturesEndpoints)
}
