package ws

import (
	"strings"

	"github.com/coachpo/meltica/core"
)

// channelMapper provides bidirectional conversion between protocol topics and Kraken channels.
type channelMapper struct {
	protocolToKraken map[string]string
	krakenToProtocol map[string]string
}

// newChannelMapper creates a mapper with bidirectional conversion tables.
func newChannelMapper() *channelMapper {
	protocolToKraken := map[string]string{
		core.TopicTrade:    "trade",
		core.TopicTicker:   "ticker",
		core.TopicDepth:    "book",
		core.TopicFullBook: "level3",  // Level 3 order book (individual orders)
		core.TopicBalance:  "balance", // for private streams
	}

	krakenToProtocol := make(map[string]string, len(protocolToKraken))
	for protocol, kraken := range protocolToKraken {
		krakenToProtocol[kraken] = protocol
	}

	krakenToProtocol["spread"] = core.TopicDepth
	krakenToProtocol["ownTrades"] = core.TopicOrder
	krakenToProtocol["openOrders"] = core.TopicOrder
	krakenToProtocol["level3"] = core.TopicFullBook

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
	case core.TopicTrade:
		return core.TradeTopic(symbol)
	case core.TopicTicker:
		return core.TickerTopic(symbol)
	case core.TopicDepth:
		return core.DepthTopic(symbol)
	case core.TopicOrder:
		return core.OrderTopic(symbol)
	case core.TopicFullBook:
		return core.BookTopic(symbol)
	case core.TopicBalance:
		return core.BalanceTopic()
	default:
		return protocolTopic + ":" + symbol
	}
}
