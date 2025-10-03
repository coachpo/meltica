package ws

import (
	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

// OKX-specific topic constants (as plain strings)
const (
	OKXTopicTrade   string = "trades"
	OKXTopicTicker  string = "tickers"
	OKXTopicBook    string = "books" // 400 depth levels, 100ms updates - best balance of depth and performance
	OKXTopicAccount string = "account"
	OKXTopicOrders  string = "orders"
	OKXTopicBalance string = "balance"
)

var mapper = corews.NewChannelMapperFromMap(map[core.Topic]string{
	core.Topic(string(corews.TopicTrade)):       OKXTopicTrade,
	core.Topic(string(corews.TopicTicker)):      OKXTopicTicker,
	core.Topic(string(corews.TopicBook)):        OKXTopicBook,
	core.Topic(string(corews.TopicUserBalance)): OKXTopicAccount,
	core.Topic(string(corews.TopicUserOrder)):   OKXTopicOrders,
})

var providerToProtocol = map[string]core.Topic{
	OKXTopicTrade:   core.Topic(string(corews.TopicTrade)),
	OKXTopicTicker:  core.Topic(string(corews.TopicTicker)),
	OKXTopicBook:    core.Topic(string(corews.TopicBook)),
	OKXTopicAccount: core.Topic(string(corews.TopicUserBalance)),
	OKXTopicOrders:  core.Topic(string(corews.TopicUserOrder)),
}

func protocolTopicFor(channel string) core.Topic {
	if topic, ok := providerToProtocol[channel]; ok {
		return topic
	}
	return core.Topic(channel)
}

func topicFromChannel(channel string, instrument string) core.Topic {
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
		if protocolTopic == core.Topic(channel) || protocolTopic == core.TopicNone {
			return core.Topic(channel + ":" + instrument)
		}
		return core.Topic(string(protocolTopic) + ":" + instrument)
	}
}
