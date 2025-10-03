package ws

import (
	"github.com/coachpo/meltica/core"

	corews "github.com/coachpo/meltica/core/ws"
)

// Kraken-specific provider channel type and constants
type KrakenChannel string

// Kraken-specific topic constants
const (
	KRKTopicTrade      KrakenChannel = "trade"
	KRKTopicTicker     KrakenChannel = "ticker"
	KRKTopicBook       KrakenChannel = "book"
	KRKTopicBalance    KrakenChannel = "balance"
	KRKTopicOwnTrades  KrakenChannel = "ownTrades"
	KRKTopicOpenOrders KrakenChannel = "openOrders"
)

var mapper = corews.NewChannelMapperFromMap(map[core.Topic]string{
	core.Topic(string(corews.TopicTrade)):       string(KRKTopicTrade),
	core.Topic(string(corews.TopicTicker)):      string(KRKTopicTicker),
	core.Topic(string(corews.TopicBook)):        string(KRKTopicBook),
	core.Topic(string(corews.TopicUserBalance)): string(KRKTopicBalance), // for private streams
})

var providerToProtocol = map[KrakenChannel]core.Topic{
	KRKTopicTrade:      core.Topic(string(corews.TopicTrade)),
	KRKTopicTicker:     core.Topic(string(corews.TopicTicker)),
	KRKTopicBook:       core.Topic(string(corews.TopicBook)),
	KRKTopicBalance:    core.Topic(string(corews.TopicUserBalance)),
	KRKTopicOwnTrades:  core.Topic(string(corews.TopicUserOrder)),
	KRKTopicOpenOrders: core.Topic(string(corews.TopicUserOrder)),
}

func protocolTopicFor(channel KrakenChannel) core.Topic {
	if topic, ok := providerToProtocol[channel]; ok {
		return topic
	}
	return core.Topic(string(channel))
}

func topicFromChannel(channel KrakenChannel, instrument string) core.Topic {
	protocolTopic := protocolTopicFor(channel)
	if instrument == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case core.Topic(string(corews.TopicTrade)):
		return corews.TradeTopic(instrument)
	case core.Topic(string(corews.TopicTicker)):
		return corews.TickerTopic(instrument)
	case core.Topic(string(corews.TopicBook)):
		return corews.BookTopic(instrument)
	case core.Topic(string(corews.TopicUserOrder)):
		return corews.UserOrderTopic(instrument)
	case core.Topic(string(corews.TopicUserBalance)):
		return corews.UserBalanceTopic()
	default:
		if protocolTopic == core.Topic(string(channel)) || protocolTopic == core.TopicNone {
			return core.Topic(string(channel) + ":" + instrument)
		}
		return core.Topic(string(protocolTopic) + ":" + instrument)
	}
}
