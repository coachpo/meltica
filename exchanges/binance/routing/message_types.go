package routing

import (
	"time"

	frameworkrouter "github.com/coachpo/meltica/market_data/framework/router"
)

// BinanceMessageTypeDescriptors returns framework descriptors for Binance websocket messages.
func BinanceMessageTypeDescriptors() []*frameworkrouter.MessageTypeDescriptor {
	now := time.Now().UTC()
	return []*frameworkrouter.MessageTypeDescriptor{
		{
			ID:            "binance.trade",
			DisplayName:   "Binance Trade",
			ProcessorRef:  "binance.trade",
			SchemaVersion: "v1",
			CreatedAt:     now,
			DetectionRules: []frameworkrouter.DetectionRule{
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "trade"},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "trade", Priority: 1},
			},
		},
		{
			ID:            "binance.orderbook",
			DisplayName:   "Binance Order Book",
			ProcessorRef:  "binance.orderbook",
			SchemaVersion: "v1",
			CreatedAt:     now,
			DetectionRules: []frameworkrouter.DetectionRule{
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "depthUpdate"},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "depthUpdate", Priority: 1},
			},
		},
		{
			ID:            "binance.ticker",
			DisplayName:   "Binance 24h Ticker",
			ProcessorRef:  "binance.ticker",
			SchemaVersion: "v1",
			CreatedAt:     now,
			DetectionRules: []frameworkrouter.DetectionRule{
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "24hrTicker"},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "24hrTicker", Priority: 1},
			},
		},
		{
			ID:            "binance.user.order",
			DisplayName:   "Binance User Order",
			ProcessorRef:  "binance.user.order",
			SchemaVersion: "v1",
			CreatedAt:     now,
			DetectionRules: []frameworkrouter.DetectionRule{
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "ORDER_TRADE_UPDATE"},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "ORDER_TRADE_UPDATE", Priority: 1},
			},
		},
		{
			ID:            "binance.user.balance",
			DisplayName:   "Binance User Balance",
			ProcessorRef:  "binance.user.balance",
			SchemaVersion: "v1",
			CreatedAt:     now,
			DetectionRules: []frameworkrouter.DetectionRule{
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "balanceUpdate"},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "data.e", ExpectedValue: "outboundAccountPosition", Priority: 1},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "balanceUpdate", Priority: 2},
				{Strategy: frameworkrouter.DetectionStrategyFieldBased, FieldPath: "e", ExpectedValue: "outboundAccountPosition", Priority: 3},
			},
		},
	}
}
