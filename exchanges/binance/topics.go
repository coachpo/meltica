package binance

import (
	"strings"

	"github.com/coachpo/meltica/core"
)

// NewTopicTranslator returns a TopicTranslator for Binance canonical/native channel mappings.
func NewTopicTranslator() core.TopicTranslator {
	return topicTranslator{}
}

type topicTranslator struct{}

var binanceTopicToNative = map[core.Topic]string{
	core.TopicTrade:       "trade",
	core.TopicTicker:      "ticker",
	core.TopicBookDelta:   "depth@100ms",
	core.TopicUserOrder:   "order",
	core.TopicUserBalance: "balance",
}

func (topicTranslator) Native(topic core.Topic) (string, error) {
	if val, ok := binanceTopicToNative[topic]; ok {
		return val, nil
	}
	return "", core.ErrNotSupported
}

func (topicTranslator) Canonical(native string) (core.Topic, error) {
	normalized := strings.ToLower(strings.TrimSpace(native))
	for topic, id := range binanceTopicToNative {
		if normalized == strings.ToLower(id) {
			return topic, nil
		}
	}
	return "", core.ErrNotSupported
}
