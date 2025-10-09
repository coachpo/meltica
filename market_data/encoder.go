package market_data

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/coachpo/meltica/core"
)

// EncodeEvent marshals the canonical event into a JSON envelope using string-based rationals.
func EncodeEvent(evt *Event) ([]byte, error) {
	if evt == nil {
		return nil, fmt.Errorf("market_data: event is nil")
	}
	if evt.Type == "" {
		return nil, fmt.Errorf("market_data: event type is required")
	}
	if evt.Symbol == "" {
		return nil, fmt.Errorf("market_data: canonical symbol is required")
	}
	if evt.Timestamp.IsZero() {
		return nil, fmt.Errorf("market_data: timestamp is required")
	}

	envelope := map[string]any{
		"type":          string(evt.Type),
		"symbol":        evt.Symbol,
		"native_symbol": evt.NativeSymbol,
		"timestamp":     evt.Timestamp.UTC().Format(time.RFC3339Nano),
	}
	if evt.Sequence != 0 {
		envelope["sequence"] = evt.Sequence
	}
	if evt.Metadata != nil && len(evt.Metadata) > 0 {
		envelope["metadata"] = evt.Metadata
	}
	if evt.Venue != "" {
		envelope["venue"] = string(evt.Venue)
	}

	switch payload := evt.Payload.(type) {
	case *TradePayload:
		if payload == nil {
			return nil, fmt.Errorf("market_data: trade payload is nil")
		}
		price, err := ratString("trade.price", payload.Price)
		if err != nil {
			return nil, err
		}
		qty, err := ratString("trade.quantity", payload.Quantity)
		if err != nil {
			return nil, err
		}
		envelope["price"] = price
		envelope["quantity"] = qty
		envelope["side"] = string(payload.Side)
		envelope["taker"] = payload.IsTaker
		if payload.VenueTradeID != "" {
			envelope["trade_id"] = payload.VenueTradeID
		}
	case *OrderBookPayload:
		if payload == nil {
			return nil, fmt.Errorf("market_data: order book payload is nil")
		}
		envelope["snapshot"] = payload.Snapshot
		bids, err := encodeDepthLevels("bids", payload.Bids)
		if err != nil {
			return nil, err
		}
		asks, err := encodeDepthLevels("asks", payload.Asks)
		if err != nil {
			return nil, err
		}
		envelope["bids"] = bids
		envelope["asks"] = asks
	case *FundingPayload:
		if payload == nil {
			return nil, fmt.Errorf("market_data: funding payload is nil")
		}
		rate, err := ratString("funding.rate", payload.Rate)
		if err != nil {
			return nil, err
		}
		envelope["rate"] = rate
		if payload.Interval > 0 {
			envelope["interval_hours"] = int(payload.Interval / time.Hour)
		}
		if !payload.EffectiveAt.IsZero() {
			envelope["next_funding"] = payload.EffectiveAt.UTC().Format(time.RFC3339Nano)
		}
	case *AccountPayload:
		if payload == nil {
			return nil, fmt.Errorf("market_data: account payload is nil")
		}
		if payload.Kind != "" {
			envelope["reason"] = string(payload.Kind)
		}
		balances := make([]map[string]string, 0, len(payload.Balances))
		for idx, bal := range payload.Balances {
			total, err := ratString(fmt.Sprintf("account.balances[%d].total", idx), bal.Total)
			if err != nil {
				return nil, err
			}
			available, err := ratString(fmt.Sprintf("account.balances[%d].available", idx), bal.Available)
			if err != nil {
				return nil, err
			}
			balances = append(balances, map[string]string{
				"asset":     bal.Asset,
				"total":     total,
				"available": available,
			})
		}
		envelope["balances"] = balances
	default:
		return nil, fmt.Errorf("market_data: unsupported payload type %T", evt.Payload)
	}

	return json.Marshal(envelope)
}

// DecodeEvent decodes the JSON envelope back into a canonical event using ParseEvent.
func DecodeEvent(exchange core.ExchangeName, raw []byte) (*Event, error) {
	return ParseEvent(exchange, raw)
}

func encodeDepthLevels(field string, levels []core.BookDepthLevel) ([]map[string]string, error) {
	encoded := make([]map[string]string, 0, len(levels))
	for idx, level := range levels {
		price, err := ratString(fmt.Sprintf("%s[%d].price", field, idx), level.Price)
		if err != nil {
			return nil, err
		}
		qty, err := ratString(fmt.Sprintf("%s[%d].quantity", field, idx), level.Qty)
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, map[string]string{
			"price":    price,
			"quantity": qty,
		})
	}
	return encoded, nil
}

func ratString(field string, value *big.Rat) (string, error) {
	if value == nil {
		return "", fmt.Errorf("market_data: %s is nil", field)
	}
	return value.RatString(), nil
}
