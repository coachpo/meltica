package ws

import (
	"strings"

	"github.com/coachpo/meltica/core"
)

var mapper = core.NewChannelMapper(core.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		core.TopicTrade:     "trade",
		core.TopicTicker:    "bookTicker",
		core.TopicDepth:     "depth5",
		core.TopicFullBook:  "depth",
		core.TopicSnapshot5: "depth5",
	},
	AdditionalProviderMappings: map[string]string{
		"trade":       core.TopicTrade,
		"bookTicker":  core.TopicTicker,
		"depth5":      core.TopicDepth,
		"depth":       core.TopicFullBook,
		"depthUpdate": core.TopicDepth,
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
	case core.TopicTrade:
		return core.TradeTopic(instrument)
	case core.TopicTicker:
		return core.TickerTopic(instrument)
	case core.TopicDepth, core.TopicFullBook:
		return core.DepthTopic(instrument)
	default:
		if protocolTopic == "" {
			return event + ":" + instrument
		}
		return protocolTopic + ":" + instrument
	}
}
