package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// OKX-specific topic constants
const (
	TopicTrade   = "trades"
	TopicTicker  = "tickers"
	TopicBook    = "books" // 400 depth levels, 100ms updates - best balance of depth and performance
	TopicAccount = "account"
	TopicOrders  = "orders"
	TopicBalance = "balance"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       TopicTrade,
		corews.TopicTicker:      TopicTicker,
		corews.TopicBook:        TopicBook,
		corews.TopicUserBalance: TopicAccount,
		corews.TopicUserOrder:   TopicOrders,
	},
	AdditionalProviderMappings: map[string]string{
		TopicTrade:   corews.TopicTrade,
		TopicTicker:  corews.TopicTicker,
		TopicBook:    corews.TopicBook,
		TopicAccount: corews.TopicUserBalance,
		TopicOrders:  corews.TopicUserOrder,
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
