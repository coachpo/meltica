package binance

import (
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
)

var linearFuturesEndpoints = futuresEndpoints{
	api:          string(rest.LinearAPI),
	tickerPath:   "/fapi/v1/ticker/bookTicker",
	orderPath:    "/fapi/v1/order",
	positionPath: "/fapi/v2/positionRisk",
}

func newLinearFuturesAPI(x *Exchange) core.FuturesAPI {
	return newFuturesAPI(x, core.MarketLinearFutures, linearFuturesEndpoints)
}
