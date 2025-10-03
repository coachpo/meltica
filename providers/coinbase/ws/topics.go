package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// Coinbase-specific topic constants
const (
	CNBTopicTrade     = "matches"
	CNBTopicTicker    = "ticker"
	CNBTopicBookDepth = "level2_batch"
	CNBTopicUser      = "user"
	CNBTopicLevel2    = "level2"
	CNBTopicMatches   = "matches"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:  CNBTopicTrade,
		corews.TopicTicker: CNBTopicTicker,
		// corews.TopicBook:        "level2", need authentication
		corews.TopicBook:        CNBTopicBookDepth,
		corews.TopicUserBalance: CNBTopicUser,
		corews.TopicUserOrder:   CNBTopicUser,
	},
	AdditionalProviderMappings: map[string]string{
		CNBTopicTrade:  corews.TopicTrade,
		CNBTopicTicker: corews.TopicTicker,
		// corews.TopicBook:        "level2", need authentication
		CNBTopicBookDepth: corews.TopicBook,
		CNBTopicUser:      corews.TopicUserBalance,
		"match":           corews.TopicTrade,
		"l2update":        corews.TopicBook,
		"snapshot":        corews.TopicBook,
		"received":        corews.TopicUserOrder,
		"open":            corews.TopicUserOrder,
		"done":            corews.TopicUserOrder,
		"change":          corews.TopicUserOrder,
		"activate":        corews.TopicUserOrder,
		"wallet":          corews.TopicUserBalance,
		"profile":         corews.TopicUserBalance,
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
