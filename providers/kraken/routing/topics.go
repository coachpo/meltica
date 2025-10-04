package routing

const (
	KrkTopicTrade      = "trade"
	KrkTopicTicker     = "ticker"
	KrkTopicBook       = "book"
	KrkTopicBalance    = "balance"
	KrkTopicOwnTrades  = "ownTrades"
	KrkTopicOpenOrders = "openOrders"
)

func topicFromChannel(channel, instrument string) string {
	if instrument == "" {
		return channel
	}
	if channel == KrkTopicBalance {
		return channel
	}
	return channel + ":" + instrument
}
