package bootstrap

import (
	"math/big"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	coretransport "github.com/coachpo/meltica/core/transport"
)

// Option is a functional option for configuring exchange construction parameters.
type Option func(*ConstructionParams)

// ConstructionParams holds all configuration needed to construct an exchange.
type ConstructionParams struct {
	ConfigOpts []config.Option
	Transports TransportFactories
	Routers    RouterFactories
}

// TransportConfig holds unified transport configuration (REST + WS + credentials).
type TransportConfig struct {
	// REST configuration
	APIKey         string
	Secret         string
	SpotBaseURL    string
	LinearBaseURL  string
	InverseBaseURL string
	HTTPTimeout    time.Duration

	// WebSocket configuration
	PublicURL        string
	PrivateURL       string
	HandshakeTimeout time.Duration

	// Additional metadata
	SymbolRefreshInterval time.Duration
}

// TransportFactories holds factory functions for creating transport clients.
type TransportFactories struct {
	NewRESTClient func(TransportConfig) coretransport.RESTClient
	NewWSClient   func(TransportConfig) coretransport.StreamClient
}

// RouterFactories holds factory functions for creating routers.
type RouterFactories struct {
	NewRESTRouter func(coretransport.RESTClient) interface{}
	NewWSRouter   func(coretransport.StreamClient, interface{}) interface{}
}

// NewConstructionParams creates a new ConstructionParams with zero values.
// Exchange implementations should seed this with their specific defaults.
func NewConstructionParams() *ConstructionParams {
	return &ConstructionParams{
		ConfigOpts: make([]config.Option, 0),
	}
}

// Order captures the canonical state of a client order across venues.
type Order struct {
	ID             string
	ClientOrderID  string
	Symbol         string
	Market         core.Market
	Side           core.OrderSide
	Type           core.OrderType
	TimeInForce    core.TimeInForce
	Status         core.OrderStatus
	Price          *big.Rat
	StopPrice      *big.Rat
	Quantity       *big.Rat
	FilledQuantity *big.Rat
	AveragePrice   *big.Rat
	CreatedAt      time.Time
	UpdatedAt      time.Time
	VenueTimestamp time.Time
}

// Trade represents a canonical execution event emitted by an exchange.
type Trade struct {
	ID         string
	OrderID    string
	Symbol     string
	Market     core.Market
	Side       core.OrderSide
	Price      *big.Rat
	Quantity   *big.Rat
	Fee        Fee
	Liquidity  Liquidity
	ExecutedAt time.Time
}

// Liquidity indicates whether a trade added or removed liquidity.
type Liquidity string

const (
	// LiquidityMaker marks maker executions.
	LiquidityMaker Liquidity = "maker"
	// LiquidityTaker marks taker executions.
	LiquidityTaker Liquidity = "taker"
)

// Fee quantifies charges applied to an execution or balance.
type Fee struct {
	Asset  string
	Amount *big.Rat
}

// Account provides a canonical snapshot of balances and margin state.
type Account struct {
	AccountID       string
	Type            AccountType
	Equity          *big.Rat
	MarginAvailable *big.Rat
	MarginUsed      *big.Rat
	UnrealizedPNL   *big.Rat
	Balances        []Balance
	UpdatedAt       time.Time
}

// AccountType enumerates high-level account categories.
type AccountType string

const (
	// AccountTypeSpot represents cash ledger accounts.
	AccountTypeSpot AccountType = "spot"
	// AccountTypeMargin represents margin trading accounts.
	AccountTypeMargin AccountType = "margin"
	// AccountTypeDerivative represents futures and swap accounts.
	AccountTypeDerivative AccountType = "derivative"
)

// Balance expresses per-asset holdings in canonical form.
type Balance struct {
	Asset     string
	Total     *big.Rat
	Available *big.Rat
	Hold      *big.Rat
}

// Position normalizes derivative position state.
type Position struct {
	Symbol           string
	Market           core.Market
	Side             core.OrderSide
	Quantity         *big.Rat
	EntryPrice       *big.Rat
	MarkPrice        *big.Rat
	LiquidationPrice *big.Rat
	RealizedPNL      *big.Rat
	UnrealizedPNL    *big.Rat
	Leverage         *big.Rat
	Margin           *big.Rat
	UpdatedAt        time.Time
	VenueTimestamp   time.Time
}
