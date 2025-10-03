package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// OKX-specific topic constants
const (
	OKXTopicTrade   = "trades"
	OKXTopicTicker  = "tickers"
	OKXTopicBook    = "books" // 400 depth levels, 100ms updates - best balance of depth and performance
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
	AdditionalProviderMappings: map[string]string{
		OKXTopicTrade:   corews.TopicTrade,
		OKXTopicTicker:  corews.TopicTicker,
		OKXTopicBook:    corews.TopicBook,
		OKXTopicAccount: corews.TopicUserBalance,
		OKXTopicOrders:  corews.TopicUserOrder,
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
			return instrument
		}
		return strings.Join([]string{protocolTopic, instrument}, ":")
	}
}
