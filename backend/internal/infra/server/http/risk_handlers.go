package httpserver

import (
	"fmt"
	"net/http"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/app/risk"
	"github.com/coachpo/meltica/internal/infra/config"
)

func (s *httpServer) getRiskLimits(w http.ResponseWriter, _ *http.Request) {
	limits := s.manager.RiskLimits()
	writeJSON(w, http.StatusOK, map[string]any{"limits": riskConfigFromLimits(limits)})
}

func (s *httpServer) updateRiskLimits(w http.ResponseWriter, r *http.Request) {
	limitRequestBody(w, r)
	cfg, err := decodeRiskConfig(r)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	limits, err := s.manager.ApplyRiskConfig(cfg)
	if err != nil {
		writeDecodeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "updated", "limits": riskConfigFromLimits(limits)})
}

func decodeRiskConfig(r *http.Request) (config.RiskConfig, error) {
	defer func() {
		_ = r.Body.Close()
	}()
	var cfg config.RiskConfig
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode payload: %w", err)
	}
	cfg.MaxPositionSize = strings.TrimSpace(cfg.MaxPositionSize)
	cfg.MaxNotionalValue = strings.TrimSpace(cfg.MaxNotionalValue)
	cfg.NotionalCurrency = strings.TrimSpace(cfg.NotionalCurrency)
	if len(cfg.AllowedOrderTypes) > 0 {
		normalized := make([]string, 0, len(cfg.AllowedOrderTypes))
		seen := make(map[string]struct{}, len(cfg.AllowedOrderTypes))
		for _, ot := range cfg.AllowedOrderTypes {
			trimmed := strings.TrimSpace(ot)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			normalized = append(normalized, trimmed)
		}
		cfg.AllowedOrderTypes = normalized
	}
	if err := validateRiskConfig(cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func riskConfigFromLimits(limits risk.Limits) config.RiskConfig {
	allowed := make([]string, 0, len(limits.AllowedOrderTypes))
	for _, ot := range limits.AllowedOrderTypes {
		allowed = append(allowed, string(ot))
	}
	cooldown := ""
	if limits.CircuitBreaker.Cooldown > 0 {
		cooldown = limits.CircuitBreaker.Cooldown.String()
	}
	return config.RiskConfig{
		MaxPositionSize:     limits.MaxPositionSize.String(),
		MaxNotionalValue:    limits.MaxNotionalValue.String(),
		NotionalCurrency:    limits.NotionalCurrency,
		OrderThrottle:       limits.OrderThrottle,
		OrderBurst:          limits.OrderBurst,
		MaxConcurrentOrders: limits.MaxConcurrentOrders,
		PriceBandPercent:    limits.PriceBandPercent,
		AllowedOrderTypes:   allowed,
		KillSwitchEnabled:   limits.KillSwitchEnabled,
		MaxRiskBreaches:     limits.MaxRiskBreaches,
		CircuitBreaker: config.CircuitBreakerConfig{
			Enabled:   limits.CircuitBreaker.Enabled,
			Threshold: limits.CircuitBreaker.Threshold,
			Cooldown:  cooldown,
		},
	}
}

func parseRiskConfigLimits(cfg config.RiskConfig) (risk.Limits, error) {
	limits, err := risk.ParseLimits(riskLimitsConfigFromConfig(cfg))
	if err != nil {
		return risk.Limits{}, fmt.Errorf("parse risk config: %w", err)
	}
	return limits, nil
}

func validateRiskConfig(cfg config.RiskConfig) error {
	_, err := parseRiskConfigLimits(cfg)
	return err
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
