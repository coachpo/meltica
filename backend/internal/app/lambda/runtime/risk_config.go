// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"fmt"
	"strings"

	"github.com/coachpo/meltica/internal/app/risk"
	"github.com/coachpo/meltica/internal/infra/config"
)

func parseRiskConfig(cfg config.RiskConfig) (risk.Limits, error) {
	limits, err := risk.ParseLimits(riskLimitsConfigFromConfig(cfg))
	if err != nil {
		return risk.Limits{}, fmt.Errorf("parse risk config: %w", err)
	}
	return limits, nil
}

func riskLimitsConfigFromConfig(cfg config.RiskConfig) risk.LimitsConfig {
	return risk.LimitsConfig{
		MaxPositionSize:     cfg.MaxPositionSize,
		MaxNotionalValue:    cfg.MaxNotionalValue,
		NotionalCurrency:    cfg.NotionalCurrency,
		OrderThrottle:       cfg.OrderThrottle,
		OrderBurst:          cfg.OrderBurst,
		MaxConcurrentOrders: cfg.MaxConcurrentOrders,
		PriceBandPercent:    cfg.PriceBandPercent,
		AllowedOrderTypes:   cfg.AllowedOrderTypes,
		KillSwitchEnabled:   cfg.KillSwitchEnabled,
		MaxRiskBreaches:     cfg.MaxRiskBreaches,
		CircuitBreaker: risk.CircuitBreakerConfig{
			Enabled:   cfg.CircuitBreaker.Enabled,
			Threshold: cfg.CircuitBreaker.Threshold,
			Cooldown:  cfg.CircuitBreaker.Cooldown,
		},
	}
}

// RiskLimits returns the currently applied risk limits.
func (m *Manager) RiskLimits() risk.Limits {
	return m.riskManager.Limits()
}

// UpdateRiskLimits applies new risk limits across strategy instances.
func (m *Manager) UpdateRiskLimits(limits risk.Limits) {
	m.riskManager.UpdateLimits(limits)
	if m.logger != nil {
		allowed := "none"
		if len(limits.AllowedOrderTypes) > 0 {
			names := make([]string, 0, len(limits.AllowedOrderTypes))
			for _, ot := range limits.AllowedOrderTypes {
				names = append(names, string(ot))
			}
			allowed = strings.Join(names, ",")
		}
		m.logger.Printf(
			"risk limits applied: throttle=%.2f burst=%d maxPosition=%s maxNotional=%s concurrent=%d killSwitch=%t priceBand=%.2f allowedTypes=%s circuitBreaker(enabled=%t threshold=%d cooldown=%s)",
			limits.OrderThrottle,
			limits.OrderBurst,
			limits.MaxPositionSize.String(),
			limits.MaxNotionalValue.String(),
			limits.MaxConcurrentOrders,
			limits.KillSwitchEnabled,
			limits.PriceBandPercent,
			allowed,
			limits.CircuitBreaker.Enabled,
			limits.CircuitBreaker.Threshold,
			limits.CircuitBreaker.Cooldown,
		)
	}
}

// ApplyRiskConfig converts the supplied risk configuration into limits and applies them.
func (m *Manager) ApplyRiskConfig(cfg config.RiskConfig) (risk.Limits, error) {
	limits, err := parseRiskConfig(cfg)
	if err != nil {
		return risk.Limits{}, fmt.Errorf("parse risk config: %w", err)
	}
	m.UpdateRiskLimits(limits)
	return limits, nil
}
