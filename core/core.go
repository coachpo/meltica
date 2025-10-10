package core

import (
	"context"
	"errors"
	"math/big"
	"time"

	exchangecap "github.com/coachpo/meltica/core/exchanges/capabilities"
)

// ProtocolVersion declares the canonical protocol version supported by this repository.
// Version 2.0.0 removes legacy parser compatibility and requires the processor/router architecture.
const ProtocolVersion = "2.0.0"

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
	// IOC is immediate-or-cancel.
	IOC TimeInForce = "ioc"
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
	Total     *big.Rat
	Locked    *big.Rat
	Time      time.Time
}

// BookDepthLevel represents a single price level in an order book.
type BookDepthLevel struct {
	Price *big.Rat
	Qty   *big.Rat
}

// Book captures a depth snapshot in canonical format.
type Book struct {
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
type Capability = exchangecap.Capability

// ExchangeCapabilities is a bitset describing the features available from an exchange implementation.
type ExchangeCapabilities = exchangecap.Set

const (
	// CapabilitySpotPublicREST indicates public spot REST market data.
	CapabilitySpotPublicREST Capability = exchangecap.CapabilitySpotPublicREST
	// CapabilitySpotTradingREST indicates spot trading REST endpoints.
	CapabilitySpotTradingREST Capability = exchangecap.CapabilitySpotTradingREST
	// CapabilityLinearPublicREST indicates public linear futures REST market data.
	CapabilityLinearPublicREST Capability = exchangecap.CapabilityLinearPublicREST
	// CapabilityLinearTradingREST indicates linear futures trading REST endpoints.
	CapabilityLinearTradingREST Capability = exchangecap.CapabilityLinearTradingREST
	// CapabilityInversePublicREST indicates public inverse futures REST market data.
	CapabilityInversePublicREST Capability = exchangecap.CapabilityInversePublicREST
	// CapabilityInverseTradingREST indicates inverse futures trading REST endpoints.
	CapabilityInverseTradingREST Capability = exchangecap.CapabilityInverseTradingREST
	// CapabilityWebsocketPublic indicates support for normalized public websocket topics.
	CapabilityWebsocketPublic Capability = exchangecap.CapabilityWebsocketPublic
	// CapabilityWebsocketPrivate indicates support for normalized private websocket topics.
	CapabilityWebsocketPrivate Capability = exchangecap.CapabilityWebsocketPrivate

	// Extended capability exports.
	CapabilityTradingSpotAmend     Capability = exchangecap.CapabilityTradingSpotAmend
	CapabilityTradingSpotCancel    Capability = exchangecap.CapabilityTradingSpotCancel
	CapabilityTradingLinearAmend   Capability = exchangecap.CapabilityTradingLinearAmend
	CapabilityTradingLinearCancel  Capability = exchangecap.CapabilityTradingLinearCancel
	CapabilityTradingInverseAmend  Capability = exchangecap.CapabilityTradingInverseAmend
	CapabilityTradingInverseCancel Capability = exchangecap.CapabilityTradingInverseCancel
	CapabilityAccountBalances      Capability = exchangecap.CapabilityAccountBalances
	CapabilityAccountPositions     Capability = exchangecap.CapabilityAccountPositions
	CapabilityAccountMargin        Capability = exchangecap.CapabilityAccountMargin
	CapabilityAccountTransfers     Capability = exchangecap.CapabilityAccountTransfers
	CapabilityMarketTrades         Capability = exchangecap.CapabilityMarketTrades
	CapabilityMarketTicker         Capability = exchangecap.CapabilityMarketTicker
	CapabilityMarketOrderBook      Capability = exchangecap.CapabilityMarketOrderBook
	CapabilityMarketCandles        Capability = exchangecap.CapabilityMarketCandles
	CapabilityMarketFundingRates   Capability = exchangecap.CapabilityMarketFundingRates
	CapabilityMarketMarkPrice      Capability = exchangecap.CapabilityMarketMarkPrice
	CapabilityMarketLiquidations   Capability = exchangecap.CapabilityMarketLiquidations
)

// Capabilities builds a ExchangeCapabilities bitset from the provided features.
func Capabilities(caps ...Capability) ExchangeCapabilities {
	return exchangecap.Of(caps...)
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
