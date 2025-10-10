package streams

import (
	"math/big"
	"time"

	"github.com/coachpo/meltica/core"
)

// TradeEvent represents a normalized public trade update.
type TradeEvent struct {
	Symbol      string
	VenueSymbol string
	Price       *big.Rat
	Quantity    *big.Rat
	Time        time.Time
}

// TickerEvent represents a normalized top of book update.
type TickerEvent struct {
	Symbol      string
	VenueSymbol string
	Bid         *big.Rat
	Ask         *big.Rat
	Time        time.Time
}

// BookEvent represents an order book snapshot.
type BookEvent struct {
	Symbol      string
	VenueSymbol string
	Bids        []core.BookDepthLevel
	Asks        []core.BookDepthLevel
	Time        time.Time
}

// OrderEvent is a normalized private websocket order update.
type OrderEvent struct {
	Symbol      string
	OrderID     string
	Status      core.OrderStatus
	FilledQty   *big.Rat
	AvgPrice    *big.Rat
	Quantity    *big.Rat
	Price       *big.Rat
	Side        core.OrderSide
	Type        core.OrderType
	TimeInForce core.TimeInForce
	Time        time.Time
}

// BalanceEvent is a normalized private websocket balance snapshot.
type BalanceEvent struct {
	Balances []core.Balance
}
