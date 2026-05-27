package risk

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/coachpo/meltica/internal/domain/schema"
)

// CircuitBreakerConfig contains raw circuit-breaker fields for semantic parsing.
type CircuitBreakerConfig struct {
	Enabled   bool
	Threshold int
	Cooldown  string
}

// LimitsConfig contains raw risk-limit fields for semantic parsing.
type LimitsConfig struct {
	MaxPositionSize     string
	MaxNotionalValue    string
	NotionalCurrency    string
	OrderThrottle       float64
	OrderBurst          int
	MaxConcurrentOrders int
	PriceBandPercent    float64
	AllowedOrderTypes   []string
	KillSwitchEnabled   bool
	MaxRiskBreaches     int
	CircuitBreaker      CircuitBreakerConfig
}

// ParseLimits validates raw risk-limit fields and returns executable limits.
func ParseLimits(input LimitsConfig) (Limits, error) {
	maxPositionSize, err := parsePositiveDecimal("maxPositionSize", input.MaxPositionSize)
	if err != nil {
		return Limits{}, err
	}
	maxNotionalValue, err := parsePositiveDecimal("maxNotionalValue", input.MaxNotionalValue)
	if err != nil {
		return Limits{}, err
	}
	notionalCurrency := strings.TrimSpace(input.NotionalCurrency)
	if notionalCurrency == "" {
		return Limits{}, fmt.Errorf("notionalCurrency required")
	}
	if input.OrderThrottle <= 0 {
		return Limits{}, fmt.Errorf("orderThrottle must be > 0")
	}
	if input.OrderBurst <= 0 {
		return Limits{}, fmt.Errorf("orderBurst must be > 0")
	}
	if input.MaxConcurrentOrders < 0 {
		return Limits{}, fmt.Errorf("maxConcurrentOrders must be >= 0")
	}
	if input.PriceBandPercent < 0 {
		return Limits{}, fmt.Errorf("priceBandPercent must be >= 0")
	}
	if input.MaxRiskBreaches < 0 {
		return Limits{}, fmt.Errorf("maxRiskBreaches must be >= 0")
	}
	if input.CircuitBreaker.Threshold < 0 {
		return Limits{}, fmt.Errorf("circuitBreaker.threshold must be >= 0")
	}

	cooldown, err := parseCircuitBreakerCooldown(input.CircuitBreaker)
	if err != nil {
		return Limits{}, err
	}

	return Limits{
		MaxPositionSize:     maxPositionSize,
		MaxNotionalValue:    maxNotionalValue,
		NotionalCurrency:    notionalCurrency,
		OrderThrottle:       input.OrderThrottle,
		OrderBurst:          input.OrderBurst,
		MaxConcurrentOrders: input.MaxConcurrentOrders,
		PriceBandPercent:    input.PriceBandPercent,
		AllowedOrderTypes:   parseAllowedOrderTypes(input.AllowedOrderTypes),
		KillSwitchEnabled:   input.KillSwitchEnabled,
		MaxRiskBreaches:     input.MaxRiskBreaches,
		CircuitBreaker: CircuitBreaker{
			Enabled:   input.CircuitBreaker.Enabled,
			Threshold: input.CircuitBreaker.Threshold,
			Cooldown:  cooldown,
		},
	}, nil
}

func parsePositiveDecimal(field, value string) (decimal.Decimal, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return decimal.Zero, fmt.Errorf("%s required", field)
	}
	parsed, err := decimal.NewFromString(trimmed)
	if err != nil {
		return decimal.Zero, fmt.Errorf("%s must be a valid decimal number: %w", field, err)
	}
	if parsed.Cmp(decimal.Zero) <= 0 {
		return decimal.Zero, fmt.Errorf("%s must be greater than 0", field)
	}
	return parsed, nil
}

func parseCircuitBreakerCooldown(input CircuitBreakerConfig) (time.Duration, error) {
	trimmed := strings.TrimSpace(input.Cooldown)
	if input.Enabled && trimmed == "" {
		return 0, fmt.Errorf("circuitBreaker.cooldown required when enabled")
	}
	if trimmed == "" {
		return 0, nil
	}
	parsed, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("circuitBreaker.cooldown must be a valid duration: %w", err)
	}
	return parsed, nil
}

func parseAllowedOrderTypes(types []string) []schema.OrderType {
	if len(types) == 0 {
		return nil
	}
	allowed := make([]schema.OrderType, 0, len(types))
	seen := make(map[string]struct{}, len(types))
	for _, raw := range types {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		allowed = append(allowed, schema.OrderType(trimmed))
	}
	if len(allowed) == 0 {
		return nil
	}
	return allowed
}
