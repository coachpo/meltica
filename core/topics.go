package core

import (
	"fmt"
	"strings"
	"sync"
)

// Topic represents a canonical market data or user data channel identifier.
type Topic string

const (
	TopicTrade       Topic = "mkt.trade"
	TopicTicker      Topic = "mkt.ticker"
	TopicBookDelta   Topic = "book.delta"
	TopicUserOrder   Topic = "user.order"
	TopicUserBalance Topic = "user.balance"
)

// TopicTranslator exposes canonical/native conversion helpers for an exchange topic namespace.
type TopicTranslator interface {
	Native(topic Topic) (string, error)
	Canonical(native string) (Topic, error)
}

var (
	topicTranslatorMu sync.RWMutex
	topicTranslators  = make(map[ExchangeName]TopicTranslator)
)

// RegisterTopicTranslator associates the translator with the given exchange.
func RegisterTopicTranslator(name ExchangeName, translator TopicTranslator) {
	if name == "" || translator == nil {
		return
	}
	topicTranslatorMu.Lock()
	defer topicTranslatorMu.Unlock()
	topicTranslators[name] = translator
}

// TopicTranslatorFor returns the registered topic translator for the exchange.
func TopicTranslatorFor(name ExchangeName) (TopicTranslator, error) {
	topicTranslatorMu.RLock()
	translator, ok := topicTranslators[name]
	topicTranslatorMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("core: topic translator for exchange %s not registered", name)
	}
	return translator, nil
}

// NativeTopic resolves a canonical topic identifier to the exchange-native representation.
func NativeTopic(name ExchangeName, topic Topic) (string, error) {
	translator, err := TopicTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Native(topic)
}

// CanonicalTopicFromNative converts an exchange-native topic identifier into canonical form.
func CanonicalTopicFromNative(name ExchangeName, native string) (Topic, error) {
	translator, err := TopicTranslatorFor(name)
	if err != nil {
		return "", err
	}
	return translator.Canonical(native)
}

// TopicRequiresSymbol indicates whether the canonical topic requires a symbol qualifier.
func TopicRequiresSymbol(topic Topic) bool {
	switch topic {
	case TopicUserBalance:
		return false
	default:
		return true
	}
}

// CanonicalTopic builds a canonical topic string using the topic identifier and optional symbol.
func CanonicalTopic(topic Topic, symbol string) (string, error) {
	canonical := normalizeTopic(topic)
	if canonical == "" {
		return "", fmt.Errorf("core: canonical topic: empty topic")
	}
	symbol = strings.TrimSpace(strings.ToUpper(symbol))
	if TopicRequiresSymbol(topic) {
		if symbol == "" {
			return "", fmt.Errorf("core: canonical topic %s requires symbol", canonical)
		}
		return canonical + ":" + symbol, nil
	}
	if symbol == "" {
		return canonical, nil
	}
	return canonical + ":" + symbol, nil
}

// MustCanonicalTopic builds a canonical topic string or panics if validation fails.
func MustCanonicalTopic(topic Topic, symbol string) string {
	canonical, err := CanonicalTopic(topic, symbol)
	if err != nil {
		panic(err)
	}
	return canonical
}

// ParseTopic splits a canonical topic string into its identifier and symbol.
func ParseTopic(value string) (Topic, string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", fmt.Errorf("core: topic: empty")
	}
	channel := trimmed
	symbol := ""
	if idx := strings.IndexByte(trimmed, ':'); idx >= 0 {
		channel = strings.TrimSpace(trimmed[:idx])
		symbol = strings.TrimSpace(trimmed[idx+1:])
	}
	canonical := Topic(strings.ToLower(channel))
	if canonical == "" {
		return "", "", fmt.Errorf("core: topic: empty channel")
	}
	if !isKnownCanonicalTopic(canonical) {
		return "", "", fmt.Errorf("core: topic: unknown channel %q", channel)
	}
	symbol = strings.ToUpper(symbol)
	if TopicRequiresSymbol(canonical) {
		if symbol == "" {
			return "", "", fmt.Errorf("core: topic %s requires symbol", canonical)
		}
		return canonical, symbol, nil
	}
	return canonical, symbol, nil
}

func normalizeTopic(topic Topic) string {
	return strings.TrimSpace(strings.ToLower(string(topic)))
}

func isKnownCanonicalTopic(topic Topic) bool {
	switch topic {
	case TopicTrade, TopicTicker, TopicBookDelta, TopicUserOrder, TopicUserBalance:
		return true
	default:
		return false
	}
}
