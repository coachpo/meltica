package topics

import (
	"fmt"
	"strings"
)

// Canonical websocket topic identifiers shared across the platform.
// Topics are expressed as "channel:SYMBOL" where SYMBOL is canonical (e.g., BTC-USDT).
const (
	TopicTrade       = "trade"
	TopicTicker      = "ticker"
	TopicBook        = "book"
	TopicUserOrder   = "order"
	TopicUserBalance = "balance"
)

func Trade(symbol string) string     { return buildTopic(TopicTrade, symbol) }
func Ticker(symbol string) string    { return buildTopic(TopicTicker, symbol) }
func Book(symbol string) string      { return buildTopic(TopicBook, symbol) }
func UserOrder(symbol string) string { return buildTopic(TopicUserOrder, symbol) }
func UserBalance() string            { return TopicUserBalance }

func buildTopic(channel, symbol string) string {
	channel = strings.TrimSpace(channel)
	symbol = strings.TrimSpace(symbol)
	if channel == "" {
		panic("topics: empty channel")
	}
	if symbol == "" {
		panic(fmt.Sprintf("topics: empty symbol for channel %s", channel))
	}
	return channel + ":" + symbol
}

// Parse splits a canonical topic into its channel and symbol components, validating format.
func Parse(topic string) (channel, symbol string, err error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return "", "", fmt.Errorf("topics: empty topic")
	}
	idx := strings.IndexByte(trimmed, ':')
	if idx < 0 {
		return "", "", fmt.Errorf("topics: missing separator")
	}
	channel = strings.TrimSpace(trimmed[:idx])
	symbol = strings.TrimSpace(trimmed[idx+1:])
	if channel == "" {
		return "", "", fmt.Errorf("topics: empty channel")
	}
	if symbol == "" {
		return "", "", fmt.Errorf("topics: empty symbol")
	}
	return channel, symbol, nil
}
