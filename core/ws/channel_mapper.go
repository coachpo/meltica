package ws

import (
	"fmt"
	"strings"

	corepkg "github.com/coachpo/meltica/core"
)

// ChannelMapper provides conversion from protocol topics to provider-specific channels.
// It enforces a consistent mapping pattern across all providers while allowing provider-specific customization.
type ChannelMapper struct {
	protocolToProvider map[corepkg.Topic]string
}

// NewChannelMapperFromMap creates a new channel mapper from a provided mapping.
// The input map is defensively copied so callers can reuse their map safely.
func NewChannelMapperFromMap(m map[corepkg.Topic]string) *ChannelMapper {
	cp := make(map[corepkg.Topic]string, len(m))
	for k, v := range m {
		cp[k] = v
	}
	return &ChannelMapper{protocolToProvider: cp}
}

// NewChannelMapperFromAnyMap creates a mapper from a map of protocol topics to any string-like type.
// It accepts values whose underlying type is string and converts them to string internally.
func NewChannelMapperFromAnyMap[T ~string](m map[corepkg.Topic]T) *ChannelMapper {
	cp := make(map[corepkg.Topic]string, len(m))
	for k, v := range m {
		cp[k] = string(v)
	}
	return &ChannelMapper{protocolToProvider: cp}
}

// ToProviderChannel converts a protocol topic to a provider-specific channel name.
// Returns the mapped channel name or falls back to lowercase if no mapping exists.
func (m *ChannelMapper) ToProviderChannel(protocolTopic string) string {
	if channel, ok := m.protocolToProvider[corepkg.Topic(protocolTopic)]; ok {
		return channel
	}
	return strings.ToLower(protocolTopic)
}

// ParseTopic splits a topic string in the form "channel:symbol" into its components.
// If no separator is present the entire topic is returned as the channel with an empty symbol.
func ParseTopic(topic string) (channel, symbol string) {
	idx := strings.IndexByte(topic, ':')
	if idx <= 0 || idx+1 >= len(topic) {
		panic(fmt.Errorf("invalid topic: expected 'channel:SYMBOL', got %q", topic))
	}
	return topic[:idx], topic[idx+1:]
}
