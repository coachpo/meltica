package ws

import (
	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:       "matches",
		corews.TopicTicker:      "ticker",
		corews.TopicDepth:       "level2",
		corews.TopicBook:        "level2",
		corews.TopicUserBalance: "user",
		corews.TopicUserOrder:   "user",
	},
	AdditionalProviderMappings: map[string]string{
		"matches":  corews.TopicTrade,
		"ticker":   corews.TopicTicker,
		"level2":   corews.TopicDepth,
		"user":     corews.TopicUserBalance,
		"match":    corews.TopicTrade,
		"l2update": corews.TopicDepth,
		"snapshot": corews.TopicDepth,
		"received": corews.TopicUserOrder,
		"open":     corews.TopicUserOrder,
		"done":     corews.TopicUserOrder,
		"change":   corews.TopicUserOrder,
		"activate": corews.TopicUserOrder,
		"wallet":   corews.TopicUserBalance,
		"profile":  corews.TopicUserBalance,
	},
})
