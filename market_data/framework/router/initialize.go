package router

import (
	"time"

	"github.com/coachpo/meltica/market_data/processors"
)

// InitializeRouter constructs a routing table with the built-in processors registered.
func InitializeRouter() (*RoutingTable, error) {
	rt := NewRoutingTable()

	if err := rt.SetDefault(processors.NewDefaultProcessor()); err != nil {
		return nil, err
	}

	mappings := []struct {
		descriptor *MessageTypeDescriptor
		proc       processors.Processor
	}{
		{
			descriptor: &MessageTypeDescriptor{
				ID:            "trade",
				DisplayName:   "Trade Event",
				ProcessorRef:  "trade",
				SchemaVersion: "v1",
				CreatedAt:     time.Now().UTC(),
				DetectionRules: []DetectionRule{{
					Strategy:      DetectionStrategyFieldBased,
					FieldPath:     "type",
					ExpectedValue: "trade",
				}},
			},
			proc: processors.NewTradeProcessor(),
		},
		{
			descriptor: &MessageTypeDescriptor{
				ID:            "orderbook",
				DisplayName:   "Order Book",
				ProcessorRef:  "orderbook",
				SchemaVersion: "v1",
				CreatedAt:     time.Now().UTC(),
				DetectionRules: []DetectionRule{{
					Strategy:      DetectionStrategyFieldBased,
					FieldPath:     "type",
					ExpectedValue: "book",
				}},
			},
			proc: processors.NewOrderBookProcessor(),
		},
		{
			descriptor: &MessageTypeDescriptor{
				ID:            "account",
				DisplayName:   "Account Update",
				ProcessorRef:  "account",
				SchemaVersion: "v1",
				CreatedAt:     time.Now().UTC(),
				DetectionRules: []DetectionRule{{
					Strategy:      DetectionStrategyFieldBased,
					FieldPath:     "type",
					ExpectedValue: "account",
				}},
			},
			proc: processors.NewAccountProcessor(),
		},
		{
			descriptor: &MessageTypeDescriptor{
				ID:            "funding",
				DisplayName:   "Funding Rate",
				ProcessorRef:  "funding",
				SchemaVersion: "v1",
				CreatedAt:     time.Now().UTC(),
				DetectionRules: []DetectionRule{{
					Strategy:      DetectionStrategyFieldBased,
					FieldPath:     "type",
					ExpectedValue: "funding",
				}},
			},
			proc: processors.NewFundingProcessor(),
		},
	}

	for _, mapping := range mappings {
		if err := rt.Register(mapping.descriptor, mapping.proc); err != nil {
			return nil, err
		}
	}

	return rt, nil
}
