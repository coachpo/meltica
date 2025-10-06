package core

import (
	"context"
	"errors"
	"math/big"
	"time"
)

// ProtocolVersion declares the canonical protocol version supported by this repository.
// Version 1.2.0 introduces WebSocket code organization refactoring and Channel Mapper architecture.
const ProtocolVersion = "1.2.0"

// Market enumerates the product families supported by the protocol.
type Market string

const (
	// MarketSpot reports spot instruments traded on a cash ledger.
	MarketSpot Market = "spot"
	// MarketLinearFutures reports linear (USDT/USDC margined) perpetuals or futures.
	MarketLinearFutures Market = "linear_futures"
	// MarketInverseFutures reports inverse (coin margined) perpetuals or futures.
	MarketInverseFutures Market = "inverse_futures"
)

// OrderSide expresses trade direction.
type OrderSide string

// OrderType enumerates canonical order entry types.
type OrderType string

// TimeInForce enumerates supported execution policies.
type TimeInForce string

const (
	// SideBuy submits a bid order.
	SideBuy OrderSide = "buy"
	// SideSell submits an ask order.
	SideSell OrderSide = "sell"

	// TypeMarket executes immediately against the book.
	TypeMarket OrderType = "market"
	// TypeLimit rests at a limit price.
	TypeLimit OrderType = "limit"

	// GTC is good-till-cancelled.
	GTC TimeInForce = "gtc"
	// FOK is fill-or-kill.
	FOK TimeInForce = "fok"
	// ICO is immediate-or-cancel.
	ICO TimeInForce = "ioc"
)

// Instrument is the canonical contract definition returned by every exchange.
type Instrument struct {
	Symbol     string
	Base       string
	Quote      string
	Market     Market
	PriceScale int
	QtyScale   int
}

// OrderRequest is a normalized order placement payload.
type OrderRequest struct {
	Symbol      string
	Side        OrderSide
	Type        OrderType
	Quantity    *big.Rat
	Price       *big.Rat
	TimeInForce TimeInForce
	ClientID    string
	ReduceOnly  bool
}

// Order represents a normalized order status snapshot.
type Order struct {
	ID        string
	Symbol    string
	Status    OrderStatus
	FilledQty *big.Rat
	AvgPrice  *big.Rat
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Ticker conveys the top-of-book quote for a symbol.
type Ticker struct {
	Symbol string
	Bid    *big.Rat
	Ask    *big.Rat
	Time   time.Time
}

// Position is a normalized futures position snapshot.
type Position struct {
	Symbol     string
	Side       OrderSide
	Quantity   *big.Rat
	EntryPrice *big.Rat
	Leverage   *big.Rat
	Unrealized *big.Rat
	UpdatedAt  time.Time
}

// Trade is a normalized historical trade record.
type Trade struct {
	Symbol   string
	ID       string
	Price    *big.Rat
	Quantity *big.Rat
	Side     OrderSide
	Time     time.Time
}

// Balance represents a normalized account balance.
type Balance struct {
	Asset     string
	Available *big.Rat
	Time      time.Time
}

// BookDepthLevel represents a single price level in an order book.
type BookDepthLevel struct {
	Price *big.Rat
	Qty   *big.Rat
}

// OrderBook captures a depth snapshot in canonical format.
type OrderBook struct {
	Symbol string
	Bids   []BookDepthLevel
	Asks   []BookDepthLevel
	Time   time.Time
}

// Candlestick is a normalized candlestick record.
type Candlestick struct {
	Symbol   string
	Interval time.Duration
	Open     *big.Rat
	High     *big.Rat
	Low      *big.Rat
	Close    *big.Rat
	Volume   *big.Rat
	Start    time.Time
	End      time.Time
}

// Capability describes a discrete feature of an exchange.
type Capability uint64

const (
	// CapabilitySpotPublicREST indicates public spot REST market data.
	CapabilitySpotPublicREST Capability = 1 << iota
	// CapabilitySpotTradingREST indicates spot trading REST endpoints.
	CapabilitySpotTradingREST
	// CapabilityLinearPublicREST indicates public linear futures REST market data.
	CapabilityLinearPublicREST
	// CapabilityLinearTradingREST indicates linear futures trading REST endpoints.
	CapabilityLinearTradingREST
	// CapabilityInversePublicREST indicates public inverse futures REST market data.
	CapabilityInversePublicREST
	// CapabilityInverseTradingREST indicates inverse futures trading REST endpoints.
	CapabilityInverseTradingREST
	// CapabilityWebsocketPublic indicates support for normalized public websocket topics.
	CapabilityWebsocketPublic
	// CapabilityWebsocketPrivate indicates support for normalized private websocket topics.
	CapabilityWebsocketPrivate
)

// ExchangeCapabilities is a bitset describing the features available from an exchange implementation.
type ExchangeCapabilities uint64

// Capabilities builds a ExchangeCapabilities bitset from the provided features.
func Capabilities(caps ...Capability) ExchangeCapabilities {
	var bits uint64
	for _, c := range caps {
		bits |= uint64(c)
	}
	return ExchangeCapabilities(bits)
}

// Has reports whether the capability bit is present.
func (pc ExchangeCapabilities) Has(cap Capability) bool {
	return uint64(pc)&uint64(cap) != 0
}

// Exchange is the stable abstraction implemented by every concrete exchange adapter.
// The interface is intentionally business-agnostic; market-specific behaviour is surfaced through
// optional participant interfaces such as SpotParticipant or WebsocketParticipant.
type Exchange interface {
	Name() string
	Capabilities() ExchangeCapabilities
	SupportedProtocolVersion() string
	Close() error
}

// SpotParticipant is implemented by exchanges that expose spot market APIs.
type SpotParticipant interface {
	Spot(ctx context.Context) SpotAPI
}

// LinearFuturesParticipant is implemented by exchanges that expose linear futures APIs.
type LinearFuturesParticipant interface {
	LinearFutures(ctx context.Context) FuturesAPI
}

// InverseFuturesParticipant is implemented by exchanges that expose inverse futures APIs.
type InverseFuturesParticipant interface {
	InverseFutures(ctx context.Context) FuturesAPI
}

// WebsocketParticipant is implemented by exchanges that provide websocket access.
type WebsocketParticipant interface {
	WS() WS
}

// SpotAPI exposes canonicalized spot REST endpoints.
type SpotAPI interface {
	ServerTime(ctx context.Context) (time.Time, error)
	Instruments(ctx context.Context) ([]Instrument, error)
	Ticker(ctx context.Context, symbol string) (Ticker, error)
	Balances(ctx context.Context) ([]Balance, error)
	Trades(ctx context.Context, symbol string, since int64) ([]Trade, error)
	PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
	GetOrder(ctx context.Context, symbol, id, clientID string) (Order, error)
	CancelOrder(ctx context.Context, symbol, id, clientID string) error
}

// FuturesAPI exposes canonicalized linear or inverse futures REST endpoints.
type FuturesAPI interface {
	Instruments(ctx context.Context) ([]Instrument, error)
	Ticker(ctx context.Context, symbol string) (Ticker, error)
	PlaceOrder(ctx context.Context, req OrderRequest) (Order, error)
	Positions(ctx context.Context, symbols ...string) ([]Position, error)
}

// WS exposes public and private websocket subscriptions.
type WS interface {
	SubscribePublic(ctx context.Context, topics ...string) (Subscription, error)
	SubscribePrivate(ctx context.Context, topics ...string) (Subscription, error)
}

// Subscription delivers normalized websocket messages for a topic set.
type Subscription interface {
	C() <-chan Message
	Err() <-chan error
	Close() error
}

// Message is the canonical websocket envelope emitted by subscriptions.
type Message struct {
	Topic  string
	Raw    []byte
	At     time.Time
	Event  string
	Parsed any
}

// ErrNotSupported indicates that a capability is not available for an exchange.
var ErrNotSupported = errors.New("not supported")
