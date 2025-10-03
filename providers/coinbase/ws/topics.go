package ws

import (
	"github.com/coachpo/meltica/core"

	corews "github.com/coachpo/meltica/core/ws"
)

// Coinbase-specific provider channel type and constants
type CoinbaseChannel string

// Coinbase-specific topic constants
const (
	CNBTopicTrade     CoinbaseChannel = "matches"
	CNBTopicTicker    CoinbaseChannel = "ticker"
	CNBTopicBookDepth CoinbaseChannel = "level2_batch"
	CNBTopicUser      CoinbaseChannel = "user"
	CNBTopicLevel2    CoinbaseChannel = "level2"
	CNBTopicMatches   CoinbaseChannel = "matches"
)

var mapper = corews.NewChannelMapperFromMap(map[core.Topic]string{
	core.Topic(string(corews.TopicTrade)):  string(CNBTopicTrade),
	core.Topic(string(corews.TopicTicker)): string(CNBTopicTicker),
	// corews.TopicBook:        "level2", need authentication
	core.Topic(string(corews.TopicBook)):        string(CNBTopicBookDepth),
	core.Topic(string(corews.TopicUserBalance)): string(CNBTopicUser),
	core.Topic(string(corews.TopicUserOrder)):   string(CNBTopicUser),
})

var providerToProtocol = map[CoinbaseChannel]core.Topic{
	CNBTopicTrade:               core.Topic(string(corews.TopicTrade)),
	CNBTopicTicker:              core.Topic(string(corews.TopicTicker)),
	CNBTopicBookDepth:           core.Topic(string(corews.TopicBook)),
	CNBTopicUser:                core.Topic(string(corews.TopicUserBalance)),
	CoinbaseChannel("match"):    core.Topic(string(corews.TopicTrade)),
	CoinbaseChannel("l2update"): core.Topic(string(corews.TopicBook)),
	CoinbaseChannel("snapshot"): core.Topic(string(corews.TopicBook)),
	CoinbaseChannel("received"): core.Topic(string(corews.TopicUserOrder)),
	CoinbaseChannel("open"):     core.Topic(string(corews.TopicUserOrder)),
	CoinbaseChannel("done"):     core.Topic(string(corews.TopicUserOrder)),
	CoinbaseChannel("change"):   core.Topic(string(corews.TopicUserOrder)),
	CoinbaseChannel("activate"): core.Topic(string(corews.TopicUserOrder)),
	CoinbaseChannel("wallet"):   core.Topic(string(corews.TopicUserBalance)),
	CoinbaseChannel("profile"):  core.Topic(string(corews.TopicUserBalance)),
}

func protocolTopicFor(name string) core.Topic {
	if topic, ok := providerToProtocol[CoinbaseChannel(name)]; ok {
		return topic
	}
	return core.Topic(name)
}

func topicFromProviderName(name, instrument string) core.Topic {
	protocolTopic := protocolTopicFor(name)
	if instrument == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case core.Topic(string(corews.TopicTrade)):
		return corews.TradeTopic(instrument)
	case core.Topic(string(corews.TopicTicker)):
		return corews.TickerTopic(instrument)
	case core.Topic(string(corews.TopicBook)):
		return corews.BookTopic(instrument)
	case core.Topic(string(corews.TopicUserOrder)):
		return corews.UserOrderTopic(instrument)
	case core.Topic(string(corews.TopicUserBalance)):
		return corews.UserBalanceTopic()
	default:
		if protocolTopic == core.Topic(name) || protocolTopic == core.TopicNone {
			return core.Topic(name + ":" + instrument)
		}
		return core.Topic(string(protocolTopic) + ":" + instrument)
	}
}
