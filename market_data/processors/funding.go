package processors

import (
	"context"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/market_data"
)

type FundingProcessor struct{}

func NewFundingProcessor() *FundingProcessor {
	return &FundingProcessor{}
}

func (p *FundingProcessor) Initialize(ctx context.Context) error {
	return contextError(ctx)
}

func (p *FundingProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var payload struct {
		Rate         string `json:"rate"`
		IntervalHour int    `json:"interval_hours"`
		NextFunding  string `json:"next_funding"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, decodeError("funding payload decode failed", err)
	}
	rate, err := parseDecimal("rate", payload.Rate, false)
	if err != nil {
		return nil, err
	}
	if payload.IntervalHour < 0 {
		return nil, invalidFieldError("interval_hours", "must be non-negative", nil)
	}
	interval := time.Duration(payload.IntervalHour) * time.Hour
	effectiveAt, err := parseTimestamp("next_funding", payload.NextFunding)
	if err != nil {
		return nil, err
	}
	return &market_data.FundingPayload{Rate: rate, Interval: interval, EffectiveAt: effectiveAt}, nil
}

func (p *FundingProcessor) MessageTypeID() string { return "funding" }

var _ Processor = (*FundingProcessor)(nil)
