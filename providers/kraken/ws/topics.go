package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// Kraken-specific topic constants
const (
	TopicTrade      = "trade"
	TopicTicker     = "ticker"
	TopicBook       = "book"
	TopicBalance    = "balance"
	TopicSpread     = "spread"
	TopicOwnTrades  = "ownTrades"
	TopicOpenOrders = "openOrders"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       TopicTrade,
		corews.TopicTicker:      TopicTicker,
		corews.TopicBook:        TopicBook,
		corews.TopicUserBalance: TopicBalance, // for private streams
	},
	AdditionalProviderMappings: map[string]string{
		TopicTrade:      corews.TopicTrade,
		TopicTicker:     corews.TopicTicker,
		TopicBook:       corews.TopicBook,
		TopicBalance:    corews.TopicUserBalance,
		TopicSpread:     corews.TopicBook,
		TopicOwnTrades:  corews.TopicUserOrder,
		TopicOpenOrders: corews.TopicUserOrder,
	},
})

// parseTopic splits a topic "channel:instrument" into channel and instrument parts.
func parseTopic(topic string) (channel, instrument string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}

// normalizePublicChannel maps a protocol topic to Kraken's channel naming.
func normalizePublicChannel(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return mapper.ToProviderChannel(trimmed)
}

// topicFromChannel builds the protocol topic for a channel and instrument.
func topicFromChannel(channel, instrument string) string {
	protocolTopic := mapper.ToProtocolTopic(channel)
	if instrument == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case corews.TopicTrade:
		return corews.TradeTopic(instrument)
	case corews.TopicTicker:
		return corews.TickerTopic(instrument)
	case corews.TopicUserOrder:
		return corews.UserOrderTopic(instrument)
	case corews.TopicBook:
		return corews.BookTopic(instrument)
	case corews.TopicUserBalance:
		return corews.UserBalanceTopic()
	default:
		return protocolTopic + ":" + instrument
	}
}
