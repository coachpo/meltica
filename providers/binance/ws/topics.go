package ws

import (
	corews "github.com/coachpo/meltica/core/ws"
)

// Binance-specific topic constants
const (
	BNXTradeChannel     = "trade"
	BNXTickerChannel    = "bookTicker"
	BNXBookDepthChannel = "depth@100ms"
	BNXOrderChannel     = "order"
	BNXBalanceChannel   = "balance"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       BNXTradeChannel,
		corews.TopicTicker:      BNXTickerChannel,
		corews.TopicBook:        BNXBookDepthChannel,
		corews.TopicUserOrder:   BNXOrderChannel,
		corews.TopicUserBalance: BNXBalanceChannel,
	},
})

var providerToProtocol = map[string]string{
	BNXTradeChannel:     corews.TopicTrade,
	BNXTickerChannel:    corews.TopicTicker,
	BNXBookDepthChannel: corews.TopicBook,
	BNXOrderChannel:     corews.TopicUserOrder,
	BNXBalanceChannel:   corews.TopicUserBalance,
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
