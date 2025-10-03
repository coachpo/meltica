package ws

import (
	corepkg "github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

// Binance-specific provider channel type and constants
type BinanceChannel string

// Binance-specific topic constants
const (
	BNXTradeChannel     BinanceChannel = "trade"
	BNXTickerChannel    BinanceChannel = "bookTicker"
	BNXBookDepthChannel BinanceChannel = "depth20@100ms"
	BNXOrderChannel     BinanceChannel = "order"
	BNXBalanceChannel   BinanceChannel = "balance"
)

var mapper = corews.NewChannelMapperFromAnyMap(map[corepkg.Topic]BinanceChannel{
	corews.TopicTrade:       BNXTradeChannel,
	corews.TopicTicker:      BNXTickerChannel,
	corews.TopicBook:        BNXBookDepthChannel,
	corews.TopicUserOrder:   BNXOrderChannel,
	corews.TopicUserBalance: BNXBalanceChannel,
})

var providerToProtocol = map[BinanceChannel]corepkg.Topic{
	BNXTradeChannel:     corews.TopicTrade,
	BNXTickerChannel:    corews.TopicTicker,
	BNXBookDepthChannel: corews.TopicBook,
	BNXOrderChannel:     corews.TopicUserOrder,
	BNXBalanceChannel:   corews.TopicUserBalance,
}

func protocolTopicFor(channel BinanceChannel) corepkg.Topic {
	if topic, ok := providerToProtocol[channel]; ok {
		return topic
	}
	return corepkg.Topic(string(channel))
}

func topicFromChannel(channel BinanceChannel, instrument string) corepkg.Topic {
	protocolTopic := protocolTopicFor(channel)
	if instrument == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case corepkg.Topic(string(corews.TopicTrade)):
		return corews.TradeTopic(instrument)
	case corepkg.Topic(string(corews.TopicTicker)):
		return corews.TickerTopic(instrument)
	case corepkg.Topic(string(corews.TopicBook)):
		return corews.BookTopic(instrument)
	case corepkg.Topic(string(corews.TopicUserOrder)):
		return corews.UserOrderTopic(instrument)
	case corepkg.Topic(string(corews.TopicUserBalance)):
		return corews.UserBalanceTopic()
	default:
		if protocolTopic == corepkg.Topic(string(channel)) || protocolTopic == corepkg.TopicNone {
			return corepkg.Topic(string(channel) + ":" + instrument)
		}
		return corepkg.Topic(string(protocolTopic) + ":" + instrument)
	}
}
