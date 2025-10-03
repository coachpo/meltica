package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// Binance-specific topic constants
const (
	TopicTrade     = "trade"
	TopicTicker    = "bookTicker"
	TopicBookDepth = "depth20@100ms"
	TopicOrder     = "order"
	TopicBalance   = "balance"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       TopicTrade,
		corews.TopicTicker:      TopicTicker,
		corews.TopicBook:        TopicBookDepth,
		corews.TopicUserOrder:   TopicOrder,
		corews.TopicUserBalance: TopicBalance,
	},
	AdditionalProviderMappings: map[string]string{
		TopicTrade:     corews.TopicTrade,
		TopicTicker:    corews.TopicTicker,
		TopicBookDepth: corews.TopicBook,
		TopicOrder:     corews.TopicUserOrder,
		TopicBalance:   corews.TopicUserBalance,
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
	default:
		if protocolTopic == "" {
			return channel + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
