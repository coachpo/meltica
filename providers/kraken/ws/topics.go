package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// Kraken-specific topic constants
const (
	KRKTopicTrade      = "trade"
	KRKTopicTicker     = "ticker"
	KRKTopicBook       = "book"
	KRKTopicBalance    = "balance"
	KRKTopicSpread     = "spread"
	KRKTopicOwnTrades  = "ownTrades"
	KRKTopicOpenOrders = "openOrders"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       KRKTopicTrade,
		corews.TopicTicker:      KRKTopicTicker,
		corews.TopicBook:        KRKTopicBook,
		corews.TopicUserBalance: KRKTopicBalance, // for private streams
	},
	AdditionalProviderMappings: map[string]string{
		KRKTopicTrade:      corews.TopicTrade,
		KRKTopicTicker:     corews.TopicTicker,
		KRKTopicBook:       corews.TopicBook,
		KRKTopicBalance:    corews.TopicUserBalance,
		KRKTopicSpread:     corews.TopicBook,
		KRKTopicOwnTrades:  corews.TopicUserOrder,
		KRKTopicOpenOrders: corews.TopicUserOrder,
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
