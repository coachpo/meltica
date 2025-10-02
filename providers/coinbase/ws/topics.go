package ws

import (
	"github.com/coachpo/meltica/core"
)

var mapper = core.NewChannelMapper(core.ChannelMappingConfig{
	ProtocolToProvider: map[string]string{
		core.TopicTrade:     "matches",
		core.TopicTicker:    "ticker",
		core.TopicDepth:     "level2",
		core.TopicFullBook:  "level2",
		core.TopicSnapshot5: "level2",
		core.TopicBalance:   "user",
		core.TopicOrder:     "user",
	},
	AdditionalProviderMappings: map[string]string{
		"matches":  core.TopicTrade,
		"ticker":   core.TopicTicker,
		"level2":   core.TopicDepth,
		"user":     core.TopicBalance,
		"match":    core.TopicTrade,
		"l2update": core.TopicDepth,
		"snapshot": core.TopicDepth,
		"received": core.TopicOrder,
		"open":     core.TopicOrder,
		"done":     core.TopicOrder,
		"change":   core.TopicOrder,
		"activate": core.TopicOrder,
		"wallet":   core.TopicBalance,
		"profile":  core.TopicBalance,
	},
})
