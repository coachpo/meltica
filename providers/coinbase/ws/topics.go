package ws

import (
	corews "github.com/coachpo/meltica/core/ws"
)

var mapper = corews.NewChannelMapper(corews.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		corews.TopicTrade:  "matches",
		corews.TopicTicker: "ticker",
		// corews.TopicBook:        "level2", need authentication
		corews.TopicBook:        "level2_batch",
		corews.TopicUserBalance: "user",
		corews.TopicUserOrder:   "user",
	},
	AdditionalProviderMappings: map[string]string{
		"matches": corews.TopicTrade,
		"ticker":  corews.TopicTicker,
		// corews.TopicBook:        "level2", need authentication
		"level2_batch": corews.TopicBook,
		"user":         corews.TopicUserBalance,
		"match":        corews.TopicTrade,
		"l2update":     corews.TopicBook,
		"snapshot":     corews.TopicBook,
		"received":     corews.TopicUserOrder,
		"open":         corews.TopicUserOrder,
		"done":         corews.TopicUserOrder,
		"change":       corews.TopicUserOrder,
		"activate":     corews.TopicUserOrder,
		"wallet":       corews.TopicUserBalance,
		"profile":      corews.TopicUserBalance,
	},
})
