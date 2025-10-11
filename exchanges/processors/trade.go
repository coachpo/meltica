package processors

import (
	"context"
	"strings"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/market_data"
)

// TradeProcessor converts raw trade payloads into typed TradePayload models.
type TradeProcessor struct{}

// NewTradeProcessor constructs a TradeProcessor instance.
func NewTradeProcessor() *TradeProcessor {
	return &TradeProcessor{}
}

// Initialize prepares the processor for use, respecting context cancellation.
func (p *TradeProcessor) Initialize(ctx context.Context) error {
	return contextError(ctx)
}

// Process decodes a trade message into a *market_data.TradePayload.
func (p *TradeProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var payload struct {
		Price    string `json:"price"`
		Quantity string `json:"quantity"`
		Side     string `json:"side"`
		Taker    bool   `json:"taker"`
		TradeID  string `json:"trade_id"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, decodeError("trade payload decode failed", err)
	}
	price, err := parseDecimal("price", payload.Price, true)
	if err != nil {
		return nil, err
	}
	quantity, err := parseDecimal("quantity", payload.Quantity, true)
	if err != nil {
		return nil, err
	}
	side, err := parseSide(payload.Side)
	if err != nil {
		return nil, err
	}
	tradeID := strings.TrimSpace(payload.TradeID)
	if tradeID == "" {
		return nil, invalidFieldError("trade_id", "value required", nil)
	}

	return &market_data.TradePayload{
		Price:        price,
		Quantity:     quantity,
		Side:         side,
		IsTaker:      payload.Taker,
		VenueTradeID: tradeID,
	}, nil
}

// MessageTypeID reports the message type handled by the processor.
func (p *TradeProcessor) MessageTypeID() string { return "trade" }

var _ Processor = (*TradeProcessor)(nil)
