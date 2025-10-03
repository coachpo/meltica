package ws

import "strings"

// ChannelMapper provides conversion from protocol topics to provider-specific channels.
// It enforces a consistent mapping pattern across all providers while allowing provider-specific customization.
type ChannelMapper struct {
	protocolToProvider map[string]string
}

// ChannelMappingConfig defines the configuration for creating a channel mapper.
// Providers should define their specific mappings using this structure.
type ChannelMappingConfig struct {
	// ProtocolToProvider defines the primary mappings from protocol topics to provider channels.
	ProtocolToProvider map[string]string
}

// NewChannelMapper creates a new channel mapper with the provided configuration.
func NewChannelMapper(config ChannelMappingConfig) *ChannelMapper {
	if config.ProtocolToProvider == nil {
		config.ProtocolToProvider = make(map[string]string)
	}

	return &ChannelMapper{
		protocolToProvider: config.ProtocolToProvider,
	}
}

// ToProviderChannel converts a protocol topic to a provider-specific channel name.
// Returns the mapped channel name or falls back to lowercase if no mapping exists.
func (m *ChannelMapper) ToProviderChannel(protocolTopic string) string {
	if channel, ok := m.protocolToProvider[protocolTopic]; ok {
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
