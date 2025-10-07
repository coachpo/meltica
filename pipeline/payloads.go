package pipeline

import (
	"math/big"

	corestreams "github.com/coachpo/meltica/core/streams"
)

type BookPayload struct {
	Book *corestreams.BookEvent
}

func (BookPayload) isPayload() {}

type TradePayload struct {
	Trade *corestreams.TradeEvent
}

func (TradePayload) isPayload() {}

type TickerPayload struct {
	Ticker *corestreams.TickerEvent
}

func (TickerPayload) isPayload() {}

type AnalyticsEvent struct {
	Symbol     string
	VWAP       *big.Rat
	TradeCount int64
}

type AnalyticsPayload struct {
	Analytics *AnalyticsEvent
}

func (AnalyticsPayload) isPayload() {}

type AccountEvent struct {
	Symbol    string
	Balance   *big.Rat
	Available *big.Rat
	Locked    *big.Rat
}

type AccountPayload struct {
	Account *AccountEvent
}

func (AccountPayload) isPayload() {}

type OrderEvent struct {
	Symbol      string
	OrderID     string
	Side        string
	Price       *big.Rat
	Quantity    *big.Rat
	Status      string
	Type        string
	TimeInForce string
}

type OrderPayload struct {
	Order *OrderEvent
}

func (OrderPayload) isPayload() {}

type RestResponse struct {
	RequestID  string
	Method     string
	Path       string
	StatusCode int
	Body       any
	Error      error
}

type RestResponsePayload struct {
	Response *RestResponse
}

func (RestResponsePayload) isPayload() {}

type UnknownPayload struct {
	Detail any
}

func (UnknownPayload) isPayload() {}
