package processors

import (
	"context"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/market_data"
)

type OrderBookProcessor struct{}

func NewOrderBookProcessor() *OrderBookProcessor {
	return &OrderBookProcessor{}
}

func (p *OrderBookProcessor) Initialize(ctx context.Context) error {
	return contextError(ctx)
}

func (p *OrderBookProcessor) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	var payload struct {
		Snapshot bool `json:"snapshot"`
		Bids     []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"bids"`
		Asks []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"asks"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, decodeError("order book payload decode failed", err)
	}
	if len(payload.Bids) == 0 {
		return nil, invalidFieldError("bids", "must contain at least one level", nil)
	}
	if len(payload.Asks) == 0 {
		return nil, invalidFieldError("asks", "must contain at least one level", nil)
	}
	bids := make([]core.BookDepthLevel, len(payload.Bids))
	for i, level := range payload.Bids {
		price, err := parseDecimal("bids.price", level.Price, true)
		if err != nil {
			return nil, err
		}
		qty, err := parseDecimal("bids.quantity", level.Quantity, true)
		if err != nil {
			return nil, err
		}
		bids[i] = core.BookDepthLevel{Price: price, Qty: qty}
	}
	asks := make([]core.BookDepthLevel, len(payload.Asks))
	for i, level := range payload.Asks {
		price, err := parseDecimal("asks.price", level.Price, true)
		if err != nil {
			return nil, err
		}
		qty, err := parseDecimal("asks.quantity", level.Quantity, true)
		if err != nil {
			return nil, err
		}
		asks[i] = core.BookDepthLevel{Price: price, Qty: qty}
	}
	return &market_data.OrderBookPayload{Snapshot: payload.Snapshot, Bids: bids, Asks: asks}, nil
}

func (p *OrderBookProcessor) MessageTypeID() string { return "orderbook" }

var _ Processor = (*OrderBookProcessor)(nil)
