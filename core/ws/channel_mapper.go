package ws

import "strings"

// ChannelMapper provides bidirectional conversion between protocol topics and provider-specific channels.
// It enforces a consistent mapping pattern across all providers while allowing provider-specific customization.
type ChannelMapper struct {
	protocolToProvider map[string]string
	providerToProtocol map[string]string
}

// ChannelMappingConfig defines the configuration for creating a channel mapper.
// Providers should define their specific mappings using this structure.
type ChannelMappingConfig struct {
	// ProtocolToProvider defines the primary mappings from protocol topics to provider channels.
	ProtocolToProvider map[string]string

	// AdditionalProviderMappings defines provider-specific aliases that map back to protocol topics.
	AdditionalProviderMappings map[string]string
}

// NewChannelMapper creates a new channel mapper with the provided configuration.
// It automatically generates the reverse mapping and combines it with additional mappings.
func NewChannelMapper(config ChannelMappingConfig) *ChannelMapper {
	if config.ProtocolToProvider == nil {
		config.ProtocolToProvider = make(map[string]string)
	}
	if config.AdditionalProviderMappings == nil {
		config.AdditionalProviderMappings = make(map[string]string)
	}

	// Start with additional mappings (these take precedence for reverse mapping).
	providerToProtocol := make(map[string]string)
	for provider, protocol := range config.AdditionalProviderMappings {
		providerToProtocol[provider] = protocol
	}

	// Add reverse mappings from ProtocolToProvider (only if not already set).
	for protocol, provider := range config.ProtocolToProvider {
		if _, exists := providerToProtocol[provider]; !exists {
			providerToProtocol[provider] = protocol
		}
	}

	return &ChannelMapper{
		protocolToProvider: config.ProtocolToProvider,
		providerToProtocol: providerToProtocol,
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

// ToProtocolTopic converts a provider-specific channel name to a protocol topic.
// Returns the mapped protocol topic or falls back to the original channel name.
func (m *ChannelMapper) ToProtocolTopic(providerChannel string) string {
	if topic, ok := m.providerToProtocol[providerChannel]; ok {
		return topic
	}
	return providerChannel
}

// TopicFromChannelName constructs a complete topic string from a provider channel and symbol.
// This is a helper function that handles the common pattern of building topic:symbol strings.
func (m *ChannelMapper) TopicFromChannelName(providerChannel, symbol string) string {
	protocolTopic := m.ToProtocolTopic(providerChannel)

	if symbol == "" {
		return protocolTopic
	}

	switch protocolTopic {
	case TopicTrade:
		return TradeTopic(symbol)
	case TopicTicker:
		return TickerTopic(symbol)
	case TopicDepth:
		return DepthTopic(symbol)
	case TopicOrder:
		return OrderTopic(symbol)
	case TopicFullBook:
		return BookTopic(symbol)
	case TopicSnapshot5:
		return Book5Topic(symbol)
	case TopicBalance:
		return BalanceTopic()
	default:
		return protocolTopic + ":" + symbol
	}
}

// GetProtocolToProviderMappings returns a copy of the protocol-to-provider mappings.
func (m *ChannelMapper) GetProtocolToProviderMappings() map[string]string {
	result := make(map[string]string, len(m.protocolToProvider))
	for k, v := range m.protocolToProvider {
		result[k] = v
	}
	return result
}

// GetProviderToProtocolMappings returns a copy of the provider-to-protocol mappings.
func (m *ChannelMapper) GetProviderToProtocolMappings() map[string]string {
	result := make(map[string]string, len(m.providerToProtocol))
	for k, v := range m.providerToProtocol {
		result[k] = v
	}
	return result
}
