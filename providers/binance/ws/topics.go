package ws

import (
	corews "github.com/coachpo/meltica/core/ws"
)

// Binance-specific topic constants
const (
	BNXTopicTrade     = "trade"
	BNXTopicTicker    = "bookTicker"
	BNXTopicBookDepth = "depth20@100ms"
	BNXTopicOrder     = "order"
	BNXTopicBalance   = "balance"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       BNXTopicTrade,
		corews.TopicTicker:      BNXTopicTicker,
		corews.TopicBook:        BNXTopicBookDepth,
		corews.TopicUserOrder:   BNXTopicOrder,
		corews.TopicUserBalance: BNXTopicBalance,
	},
})

var providerToProtocol = map[string]string{
	BNXTopicTrade:     corews.TopicTrade,
	BNXTopicTicker:    corews.TopicTicker,
	BNXTopicBookDepth: corews.TopicBook,
	BNXTopicOrder:     corews.TopicUserOrder,
	BNXTopicBalance:   corews.TopicUserBalance,
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
