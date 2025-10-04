package routing

import (
	corews "github.com/coachpo/meltica/core/ws"
)

const (
	OKXTopicTrade   = "trades"
	OKXTopicTicker  = "tickers"
	OKXTopicBook    = "books"
	OKXTopicAccount = "account"
	OKXTopicOrders  = "orders"
	OKXTopicBalance = "balance"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       OKXTopicTrade,
		corews.TopicTicker:      OKXTopicTicker,
		corews.TopicBook:        OKXTopicBook,
		corews.TopicUserBalance: OKXTopicAccount,
		corews.TopicUserOrder:   OKXTopicOrders,
	},
})

var providerToProtocol = map[string]string{
	OKXTopicTrade:   corews.TopicTrade,
	OKXTopicTicker:  corews.TopicTicker,
	OKXTopicBook:    corews.TopicBook,
	OKXTopicAccount: corews.TopicUserBalance,
	OKXTopicOrders:  corews.TopicUserOrder,
}

func protocolTopicFor(channel string) string {
	if topic, ok := providerToProtocol[channel]; ok {
		return topic
	}
	return channel
}

func topicFromChannel(channel, instrument string) string {
	protocolTopic := protocolTopicFor(channel)
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
		if protocolTopic == channel || protocolTopic == "" {
			return channel + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
