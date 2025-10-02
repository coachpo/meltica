package ws

import (
	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:     "matches",
		corews.TopicTicker:    "ticker",
		corews.TopicDepth:     "level2",
		corews.TopicFullBook:  "level2",
		corews.TopicSnapshot5: "level2",
		corews.TopicBalance:   "user",
		corews.TopicOrder:     "user",
	},
	AdditionalProviderMappings: map[string]string{
		"matches":  corews.TopicTrade,
		"ticker":   corews.TopicTicker,
		"level2":   corews.TopicDepth,
		"user":     corews.TopicBalance,
		"match":    corews.TopicTrade,
		"l2update": corews.TopicDepth,
		"snapshot": corews.TopicDepth,
		"received": corews.TopicOrder,
		"open":     corews.TopicOrder,
		"done":     corews.TopicOrder,
		"change":   corews.TopicOrder,
		"activate": corews.TopicOrder,
		"wallet":   corews.TopicBalance,
		"profile":  corews.TopicBalance,
	},
})
