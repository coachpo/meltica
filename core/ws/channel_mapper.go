package ws

import "strings"

// ChannelMapper provides conversion from protocol topics to exchange-specific channels.
// It enforces a consistent mapping pattern across all exchanges while allowing exchange-specific customization.
type ChannelMapper struct {
	protocolToExchange map[string]string
}

// ChannelMappingConfig defines the configuration for creating a channel mapper.
// Exchanges should define their specific mappings using this structure.
type ChannelMappingConfig struct {
	// ProtocolToExchange defines the primary mappings from protocol topics to exchange channels.
	ProtocolToExchange map[string]string
}

// NewChannelMapper creates a new channel mapper with the provided configuration.
func NewChannelMapper(config ChannelMappingConfig) *ChannelMapper {
	if config.ProtocolToExchange == nil {
		config.ProtocolToExchange = make(map[string]string)
	}

	return &ChannelMapper{
		protocolToExchange: config.ProtocolToExchange,
	}
}

// ToExchangeChannel converts a protocol topic to an exchange-specific channel name.
// Returns the mapped channel name or falls back to lowercase if no mapping exists.
func (m *ChannelMapper) ToExchangeChannel(protocolTopic string) string {
	if channel, ok := m.protocolToExchange[protocolTopic]; ok {
		return channel
	}
	return strings.ToLower(protocolTopic)
}

// ParseTopic splits a topic string in the form "channel:symbol" into its components.
// If no separator is present the entire topic is returned as the channel with an empty symbol.
func ParseTopic(topic string) (channel, symbol string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}
