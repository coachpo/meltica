package ws

import (
	"math/big"
	"time"

	corepkg "github.com/coachpo/meltica/core"
)

// Re-export core domain primitives used by websocket events.
type (
	DepthLevel  = corepkg.DepthLevel
	OrderStatus = corepkg.OrderStatus
	Balance     = corepkg.Balance
)

// TradeEvent represents a normalized public trade update.
type TradeEvent struct {
	Symbol   string
	Price    *big.Rat
	Quantity *big.Rat
	Time     time.Time
}

// TickerEvent represents a normalized top of book update.
type TickerEvent struct {
	Symbol string
	Bid    *big.Rat
	Ask    *big.Rat
	Time   time.Time
}

// DepthUpdateType represents the type of depth update.
type DepthUpdateType int

const (
	// DepthUpdateSnapshot represents a full order book snapshot
	DepthUpdateSnapshot DepthUpdateType = iota
	// DepthUpdateDelta represents incremental order book changes
	DepthUpdateDelta
)

// DepthEvent represents an order book delta or snapshot.
type DepthEvent struct {
	Symbol string
	Bids   []DepthLevel
	Asks   []DepthLevel
	Time   time.Time
	// UpdateType indicates whether this is a snapshot or delta update
	UpdateType DepthUpdateType
}

// OrderEvent is a normalized private websocket order update.
type OrderEvent struct {
	Symbol    string
	OrderID   string
	Status    OrderStatus
	FilledQty *big.Rat
	AvgPrice  *big.Rat
	Time      time.Time
}

// BalanceEvent is a normalized private websocket balance snapshot.
type BalanceEvent struct {
	Balances []Balance
}
