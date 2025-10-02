package okx

import (
	"strings"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/protocol"
)

var mapper = protocol.NewChannelMapper(protocol.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		core.TopicTrade:     "trades",
		core.TopicTicker:    "tickers",
		core.TopicDepth:     "books5",
		core.TopicFullBook:  "books",
		core.TopicSnapshot5: "books5",
		core.TopicBalance:   "account",
		core.TopicOrder:     "orders",
	},
	AdditionalProviderMappings: map[string]string{
		"trades":         core.TopicTrade,
		"tickers":        core.TopicTicker,
		"books5":         core.TopicDepth,
		"books":          core.TopicFullBook,
		"books1":         core.TopicDepth,
		"books-l2-tbt":   core.TopicDepth,
		"books50-l2-tbt": core.TopicDepth,
		"account":        core.TopicBalance,
		"orders":         core.TopicOrder,
	},
})

func topicFromChannel(channel, instrument string) string {
	protocolTopic := mapper.ToProtocolTopic(channel)
	if instrument == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case core.TopicTrade:
		return core.TradeTopic(instrument)
	case core.TopicTicker:
		return core.TickerTopic(instrument)
	case core.TopicDepth:
		return core.DepthTopic(instrument)
	case core.TopicFullBook:
		return core.BookTopic(instrument)
	case core.TopicOrder:
		return core.OrderTopic(instrument)
	case core.TopicBalance:
		return core.BalanceTopic()
	default:
		if protocolTopic == "" {
			return instrument
		}
		return strings.Join([]string{protocolTopic, instrument}, ":")
	}
}
