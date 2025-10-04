package topics

import "strings"

// Canonical websocket topic identifiers shared across the platform.
// Topics are expressed as "channel:SYMBOL" where SYMBOL is canonical (e.g., BTC-USDT).
const (
	TopicTrade       = "trade"
	TopicTicker      = "ticker"
	TopicBook        = "book"
	TopicUserOrder   = "order"
	TopicUserBalance = "balance"
)

func Trade(symbol string) string     { return TopicTrade + ":" + symbol }
func Ticker(symbol string) string    { return TopicTicker + ":" + symbol }
func Book(symbol string) string      { return TopicBook + ":" + symbol }
func UserOrder(symbol string) string { return TopicUserOrder + ":" + symbol }
func UserBalance() string            { return TopicUserBalance }

// ChannelMapper provides conversion from canonical topics to exchange-specific channel names.
type ChannelMapper struct {
	protocolToExchange map[string]string
}

// MappingConfig defines the configuration for constructing a ChannelMapper.
type MappingConfig struct {
	ProtocolToExchange map[string]string
}

// NewMapper creates a channel mapper with the provided configuration.
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

// Parse splits a canonical topic into its channel and symbol components.
// When no separator is present the channel is returned with an empty symbol.
func Parse(topic string) (channel, symbol string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}
