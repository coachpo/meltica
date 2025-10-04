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

// Parse splits a canonical topic into its channel and symbol components.
// When no separator is present the channel is returned with an empty symbol.
// TODO : When no separator is present the program should panic.
func Parse(topic string) (channel, symbol string) {
	if idx := strings.IndexByte(topic, ':'); idx > 0 {
		return topic[:idx], topic[idx+1:]
	}
	return topic, ""
}
