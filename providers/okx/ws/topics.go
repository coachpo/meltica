package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:     "trades",
		corews.TopicTicker:    "tickers",
		corews.TopicDepth:     "books5",
		corews.TopicFullBook:  "books",
		corews.TopicSnapshot5: "books5",
		corews.TopicBalance:   "account",
		corews.TopicOrder:     "orders",
	},
	AdditionalProviderMappings: map[string]string{
		"trades":         corews.TopicTrade,
		"tickers":        corews.TopicTicker,
		"books5":         corews.TopicDepth,
		"books":          corews.TopicFullBook,
		"books1":         corews.TopicDepth,
		"books-l2-tbt":   corews.TopicDepth,
		"books50-l2-tbt": corews.TopicDepth,
		"account":        corews.TopicBalance,
		"orders":         corews.TopicOrder,
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
	case corews.TopicDepth:
		return corews.DepthTopic(instrument)
	case corews.TopicFullBook:
		return corews.BookTopic(instrument)
	case corews.TopicOrder:
		return corews.OrderTopic(instrument)
	case corews.TopicBalance:
		return corews.BalanceTopic()
	default:
		if protocolTopic == "" {
			return instrument
		}
		return strings.Join([]string{protocolTopic, instrument}, ":")
	}
}
