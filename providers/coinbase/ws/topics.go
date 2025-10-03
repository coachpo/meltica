package ws

import (
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
})

var providerToProtocol = map[string]string{
	CNBTopicTrade:     corews.TopicTrade,
	CNBTopicTicker:    corews.TopicTicker,
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
}

func protocolTopicFor(name string) string {
	if topic, ok := providerToProtocol[name]; ok {
		return topic
	}
	return name
}

func topicFromProviderName(name, instrument string) string {
	protocolTopic := protocolTopicFor(name)
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
		if protocolTopic == name || protocolTopic == "" {
			return name + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
