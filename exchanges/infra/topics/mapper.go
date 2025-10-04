package topics

import "strings"

// ChannelMapper provides conversion from canonical topics to exchange-specific channel names.
type ChannelMapper struct {
	protocolToExchange map[string]string
}

// MappingConfig defines configuration for creating a ChannelMapper.
type MappingConfig struct {
	ProtocolToExchange map[string]string
}

// NewMapper constructs a ChannelMapper using the supplied configuration.
func NewMapper(config MappingConfig) *ChannelMapper {
	if config.ProtocolToExchange == nil {
		config.ProtocolToExchange = make(map[string]string)
	}
	return &ChannelMapper{protocolToExchange: config.ProtocolToExchange}
}

// ToExchange returns the exchange-specific channel for a canonical topic.
// If no explicit mapping exists the lower-cased topic is returned.
func (m *ChannelMapper) ToExchange(protocolTopic string) string {
	if channel, ok := m.protocolToExchange[protocolTopic]; ok {
		return channel
	}
	return strings.ToLower(protocolTopic)
}
