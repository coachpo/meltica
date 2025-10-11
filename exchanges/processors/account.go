package processors

import (
	"context"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/market_data"
)

type AccountProcessor struct{}

func NewAccountProcessor() *AccountProcessor {
	return &AccountProcessor{}
}

func (p *AccountProcessor) Initialize(ctx context.Context) error {
	return contextError(ctx)
}

func (p *AccountProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var payload struct {
		Reason   string `json:"reason"`
		Balances []struct {
			Asset     string `json:"asset"`
			Total     string `json:"total"`
			Available string `json:"available"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, decodeError("account payload decode failed", err)
	}
	if len(payload.Balances) == 0 {
		return nil, invalidFieldError("balances", "must contain at least one entry", nil)
	}
	balances := make([]market_data.AccountBalance, len(payload.Balances))
	for i, entry := range payload.Balances {
		asset := strings.ToUpper(strings.TrimSpace(entry.Asset))
		if asset == "" {
			return nil, invalidFieldError("balances.asset", "value required", nil)
		}
		total, err := parseDecimal("balances.total", entry.Total, false)
		if err != nil {
			return nil, err
		}
		if total.Sign() < 0 {
			return nil, invalidFieldError("balances.total", "must be non-negative", nil)
		}
		available, err := parseDecimal("balances.available", entry.Available, false)
		if err != nil {
			return nil, err
		}
		if available.Sign() < 0 {
			return nil, invalidFieldError("balances.available", "must be non-negative", nil)
		}
		if available.Cmp(total) > 0 {
			return nil, invalidFieldError("balances.available", "cannot exceed total", nil)
		}
		balances[i] = market_data.AccountBalance{Asset: asset, Total: total, Available: available}
	}
	return &market_data.AccountPayload{Kind: resolveAccountKind(payload.Reason), Balances: balances}, nil
}

func (p *AccountProcessor) MessageTypeID() string { return "account" }

func resolveAccountKind(reason string) market_data.AccountEventKind {
	switch strings.ToLower(strings.TrimSpace(reason)) {
	case "", "balance_update", "balance_delta":
		return market_data.AccountBalanceDelta
	default:
		return market_data.AccountUnknown
	}
}

var _ Processor = (*AccountProcessor)(nil)
