package capabilities

// Capability represents a discrete feature supported by an exchange adapter.
type Capability uint64

const (
	// Base transport and market scopes (maintained for backward compatibility).
	CapabilitySpotPublicREST Capability = 1 << iota
	CapabilitySpotTradingREST
	CapabilityLinearPublicREST
	CapabilityLinearTradingREST
	CapabilityInversePublicREST
	CapabilityInverseTradingREST
	CapabilityWebsocketPublic
	CapabilityWebsocketPrivate

	// Extended trading operations for fine-grained routing checks.
	CapabilityTradingSpotAmend
	CapabilityTradingSpotCancel
	CapabilityTradingLinearAmend
	CapabilityTradingLinearCancel
	CapabilityTradingInverseAmend
	CapabilityTradingInverseCancel

	// Account surfaces.
	CapabilityAccountBalances
	CapabilityAccountPositions
	CapabilityAccountMargin
	CapabilityAccountTransfers

	// Market data surfaces.
	CapabilityMarketTrades
	CapabilityMarketTicker
	CapabilityMarketOrderBook
	CapabilityMarketCandles
	CapabilityMarketFundingRates
	CapabilityMarketMarkPrice
	CapabilityMarketLiquidations
)

// Set is a bitset describing the capabilities a venue exposes.
type Set uint64

// Of constructs a capability set from the provided capabilities.
func Of(caps ...Capability) Set {
	var bits uint64
	for _, cap := range caps {
		bits |= uint64(cap)
	}
	return Set(bits)
}

// With returns a new set containing the provided capability.
func (s Set) With(cap Capability) Set {
	return Set(uint64(s) | uint64(cap))
}

// Without returns a new set without the provided capability.
func (s Set) Without(cap Capability) Set {
	return Set(uint64(s) &^ uint64(cap))
}

// Has reports whether the capability is present in the set.
func (s Set) Has(cap Capability) bool {
	return uint64(s)&uint64(cap) != 0
}

// All reports whether every capability in caps exists within the set.
func (s Set) All(caps ...Capability) bool {
	for _, cap := range caps {
		if !s.Has(cap) {
			return false
		}
	}
	return true
}

// Any reports whether at least one capability in caps exists within the set.
func (s Set) Any(caps ...Capability) bool {
	for _, cap := range caps {
		if s.Has(cap) {
			return true
		}
	}
	return false
}

// Missing returns the subset of caps that are not included in the set.
func (s Set) Missing(caps ...Capability) []Capability {
	missing := make([]Capability, 0, len(caps))
	for _, cap := range caps {
		if !s.Has(cap) {
			missing = append(missing, cap)
		}
	}
	return missing
}

// Slice expands the set into a slice of capabilities (unordered).
func (s Set) Slice() []Capability {
	out := make([]Capability, 0)
	for bit := Capability(1); bit != 0; bit <<= 1 {
		if s.Has(bit) {
			out = append(out, bit)
		}
		if bit == Capability(1<<63) {
			break
		}
	}
	return out
}

// SpotTrading enumerates capabilities required for spot trading flows.
var SpotTrading = []Capability{
	CapabilitySpotTradingREST,
	CapabilityTradingSpotAmend,
	CapabilityTradingSpotCancel,
}

// LinearTrading enumerates capabilities required for linear futures trading flows.
var LinearTrading = []Capability{
	CapabilityLinearTradingREST,
	CapabilityTradingLinearAmend,
	CapabilityTradingLinearCancel,
}

// InverseTrading enumerates capabilities required for inverse futures trading flows.
var InverseTrading = []Capability{
	CapabilityInverseTradingREST,
	CapabilityTradingInverseAmend,
	CapabilityTradingInverseCancel,
}

// CoreMarketData enumerates the baseline market data capabilities.
var CoreMarketData = []Capability{
	CapabilityMarketTrades,
	CapabilityMarketTicker,
	CapabilityMarketOrderBook,
}
