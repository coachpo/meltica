package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       "trades",
		corews.TopicTicker:      "tickers",
		corews.TopicBook:        "books", // 400 depth levels, 100ms updates - best balance of depth and performance
		corews.TopicUserBalance: "account",
		corews.TopicUserOrder:   "orders",
	},
	AdditionalProviderMappings: map[string]string{
		"trades":  corews.TopicTrade,
		"tickers": corews.TopicTicker,
		"books":   corews.TopicBook,
		"account": corews.TopicUserBalance,
		"orders":  corews.TopicUserOrder,
	},
})

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
