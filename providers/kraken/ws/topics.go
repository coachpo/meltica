package ws

import (
	"strings"

	corews "github.com/coachpo/meltica/core/ws"
)

// channelMapper provides bidirectional conversion between protocol topics and Kraken channels.
type channelMapper struct {
	protocolToKraken map[string]string
	krakenToProtocol map[string]string
}

// newChannelMapper creates a mapper with bidirectional conversion tables.
func newChannelMapper() *channelMapper {
	protocolToKraken := map[string]string{
		corews.TopicTrade:       "trade",
		corews.TopicTicker:      "ticker",
		corews.TopicBook:        "book",
		corews.TopicUserBalance: "balance", // for private streams
	}

	krakenToProtocol := make(map[string]string, len(protocolToKraken))
	for protocol, kraken := range protocolToKraken {
		krakenToProtocol[kraken] = protocol
	}

	krakenToProtocol["spread"] = corews.TopicBook
	krakenToProtocol["ownTrades"] = corews.TopicUserOrder
	krakenToProtocol["openOrders"] = corews.TopicUserOrder

	return &channelMapper{
		protocolToKraken: protocolToKraken,
		krakenToProtocol: krakenToProtocol,
	}
}

// toKrakenChannel converts a protocol topic to Kraken channel name.
func (m *channelMapper) toKrakenChannel(protocolTopic string) string {
	if channel, ok := m.protocolToKraken[protocolTopic]; ok {
		return channel
	}
	return strings.ToLower(protocolTopic)
}

// toProtocolTopic converts a Kraken channel name to protocol topic.
func (m *channelMapper) toProtocolTopic(krakenChannel string) string {
	if topic, ok := m.krakenToProtocol[krakenChannel]; ok {
		return topic
	}
	return krakenChannel
}

// mapper is the shared channel mapper instance for the websocket client.
var mapper = newChannelMapper()

// parseTopic splits a topic "channel:symbol" into channel and symbol parts.
func parseTopic(topic string) (channel, symbol string) {
	parts := strings.Split(topic, ":")
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// normalizePublicChannel maps a protocol topic to Kraken's channel naming.
func normalizePublicChannel(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}
	return mapper.toKrakenChannel(trimmed)
}

// topicFromChannelName builds the protocol topic for a channel and symbol.
func topicFromChannelName(name, symbol string) string {
	protocolTopic := mapper.toProtocolTopic(name)
	if symbol == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case corews.TopicTrade:
		return corews.TradeTopic(symbol)
	case corews.TopicTicker:
		return corews.TickerTopic(symbol)
	case corews.TopicUserOrder:
		return corews.UserOrderTopic(symbol)
	case corews.TopicBook:
		return corews.BookTopic(symbol)
	case corews.TopicUserBalance:
		return corews.UserBalanceTopic()
	default:
		return protocolTopic + ":" + symbol
	}
}
