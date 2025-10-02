package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// Coinbase-specific topic constants
const (
	TopicTrade     = "matches"
	TopicTicker    = "ticker"
	TopicBookDepth = "level2_batch"
	TopicUser      = "user"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:  TopicTrade,
		corews.TopicTicker: TopicTicker,
		// corews.TopicBook:        "level2", need authentication
		corews.TopicBook:        TopicBookDepth,
		corews.TopicUserBalance: TopicUser,
		corews.TopicUserOrder:   TopicUser,
	},
	AdditionalProviderMappings: map[string]string{
		TopicTrade:  corews.TopicTrade,
		TopicTicker: corews.TopicTicker,
		// corews.TopicBook:        "level2", need authentication
		TopicBookDepth: corews.TopicBook,
		TopicUser:      corews.TopicUserBalance,
		"match":        corews.TopicTrade,
		"l2update":     corews.TopicBook,
		"snapshot":     corews.TopicBook,
		"received":     corews.TopicUserOrder,
		"open":         corews.TopicUserOrder,
		"done":         corews.TopicUserOrder,
		"change":       corews.TopicUserOrder,
		"activate":     corews.TopicUserOrder,
		"wallet":       corews.TopicUserBalance,
		"profile":      corews.TopicUserBalance,
	},
})

func parseTopic(topic string) (channel, instrument string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}

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
	case corews.TopicBook:
		return corews.BookTopic(instrument)
	case corews.TopicUserOrder:
		return corews.UserOrderTopic(instrument)
	case corews.TopicUserBalance:
		return corews.UserBalanceTopic()
	default:
		if protocolTopic == "" {
			return channel + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
