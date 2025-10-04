package routing

import (
	coretopics "github.com/coachpo/meltica/core/topics"
	infratopics "github.com/coachpo/meltica/exchanges/shared/infra/topics"
)

const (
	BNXTradeChannel     = "trade"
	BNXTickerChannel    = "ticker"
	BNXBookDepthChannel = "depth@100ms"
	BNXOrderChannel     = "order"
	BNXBalanceChannel   = "balance"
)

var mapper = infratopics.NewMapper(infratopics.MappingConfig{
	ProtocolToExchange: map[string]string{
		coretopics.TopicTrade:       BNXTradeChannel,
		coretopics.TopicTicker:      BNXTickerChannel,
		coretopics.TopicBook:        BNXBookDepthChannel,
		coretopics.TopicUserOrder:   BNXOrderChannel,
		coretopics.TopicUserBalance: BNXBalanceChannel,
	},
})

var exchangeToProtocol = map[string]string{
	BNXTradeChannel:     coretopics.TopicTrade,
	BNXTickerChannel:    coretopics.TopicTicker,
	BNXBookDepthChannel: coretopics.TopicBook,
	BNXOrderChannel:     coretopics.TopicUserOrder,
	BNXBalanceChannel:   coretopics.TopicUserBalance,
}

func protocolTopicFor(channel string) string {
	if topic, ok := exchangeToProtocol[channel]; ok {
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
	case coretopics.TopicTrade:
		return coretopics.Trade(instrument)
	case coretopics.TopicTicker:
		return coretopics.Ticker(instrument)
	case coretopics.TopicBook:
		return coretopics.Book(instrument)
	case coretopics.TopicUserOrder:
		return coretopics.UserOrder(instrument)
	case coretopics.TopicUserBalance:
		return coretopics.UserBalance()
	default:
		if protocolTopic == channel || protocolTopic == "" {
			return channel + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
