package core

import (
	"math/big"
	"time"
)

type TradeEvent struct {
	Symbol   string
	Price    *big.Rat
	Quantity *big.Rat
	Time     time.Time
}

type TickerEvent struct {
	Symbol string
	Bid    *big.Rat
	Ask    *big.Rat
	Time   time.Time
}

type DepthEvent struct {
	Symbol string
	Bids   []DepthLevel
	Asks   []DepthLevel
	Time   time.Time
}

// OrderEvent is a normalized private WS order update
type OrderEvent struct {
	Symbol    string
	OrderID   string
	Status    OrderStatus
	FilledQty *big.Rat
	AvgPrice  *big.Rat
	Time      time.Time
}

type BalanceEvent struct {
	Balances []Balance
}
