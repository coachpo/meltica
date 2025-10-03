package ws

import (
	"strings"

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
	AdditionalProviderMappings: map[string]string{
		BNXTopicTrade:     corews.TopicTrade,
		BNXTopicTicker:    corews.TopicTicker,
		BNXTopicBookDepth: corews.TopicBook,
		BNXTopicOrder:     corews.TopicUserOrder,
		BNXTopicBalance:   corews.TopicUserBalance,
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
