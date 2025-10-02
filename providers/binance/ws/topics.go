package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:  "trade",
		corews.TopicTicker: "bookTicker",
		corews.TopicBook:   "depth20@100ms",
	},
	AdditionalProviderMappings: map[string]string{
		"trade":         corews.TopicTrade,
		"bookTicker":    corews.TopicTicker,
		"depth20@100ms": corews.TopicBook,
	},
})

func splitTopic(topic string) (channel, instrument string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}

func topicFromEvent(event, instrument string) string {
	protocolTopic := mapper.ToProtocolTopic(event)
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
			return event + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
