package topics

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

// ExchangeChannelID returns the exchange-specific channel for a canonical topic and panics if no mapping exists.
func (m *ChannelMapper) ExchangeChannelID(protocolTopic string) string {
	if channel, ok := m.protocolToExchange[protocolTopic]; ok {
		return channel
	}
	panic("missing exchange channel mapping for protocol topic")
}
