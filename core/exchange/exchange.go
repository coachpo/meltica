package exchange

import (
	"context"
	"math/big"
	"time"

	"github.com/coachpo/meltica/core"
)

// ---- Level 1: Infrastructure abstractions ----

// RESTTransport defines the minimal client needed by Level 2 routing to make REST calls.
type RESTTransport interface {
	Do(ctx context.Context, req RESTRequest, out any) error
}

// RESTRequest is a normalized REST invocation payload produced by Level 2.
type RESTRequest struct {
	Method string
	Path   string
	Query  map[string]string
	Body   []byte
	Signed bool
	API    string // exchange-specific surface identifier
}

// WebsocketDialer represents a handle to create websocket subscriptions.
type WebsocketDialer interface {
	SubscribePublic(ctx context.Context, streams []string) (RawSubscription, error)
	SubscribePrivate(ctx context.Context, listenKey string) (RawSubscription, error)
}

// RawSubscription is the Level 1 websocket stream contract.
type RawSubscription interface {
	Raw() <-chan RawMessage
	Err() <-chan error
	Close() error
}

// RawMessage carries unparsed websocket frames.
type RawMessage struct {
	Data []byte
	At   time.Time
}

// ---- Level 2: Routing abstractions ----

// Route identifiers emitted by websocket routing layers.
const (
	RouteUnknown         = "unknown"
	RouteTradeUpdate     = "trade_update"
	RouteTickerUpdate    = "ticker_update"
	RouteBookSnapshot    = "book_snapshot"
	RouteOrderUpdate     = "order_update"
	RouteBalanceSnapshot = "balance_snapshot"
)

// RoutedMessage is the normalized websocket payload delivered to Level 3.
type RoutedMessage struct {
	Topic  string
	Raw    []byte
	At     time.Time
	Route  string
	Parsed any
}

// Router exposes the contract Level 2 must satisfy.
type Router interface {
	SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
	SubscribePrivate(ctx context.Context) (Subscription, error)
	Close() error
}

// Subscription delivers routed websocket messages to Level 3.
type Subscription interface {
	C() <-chan RoutedMessage
	Err() <-chan error
	Close() error
}

// ---- Level 3: Exchange-facing event payloads ----

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

// BookEvent represents an order book snapshot.
type BookEvent struct {
	Symbol string
	Bids   []core.BookDepthLevel
	Asks   []core.BookDepthLevel
	Time   time.Time
}

// OrderEvent is a normalized private websocket order update.
type OrderEvent struct {
	Symbol    string
	OrderID   string
	Status    core.OrderStatus
	FilledQty *big.Rat
	AvgPrice  *big.Rat
	Time      time.Time
}

// BalanceEvent is a normalized private websocket balance snapshot.
type BalanceEvent struct {
	Balances []core.Balance
}
