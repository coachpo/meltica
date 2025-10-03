package ws

import (
	corepkg "github.com/coachpo/meltica/core"
)

// Canonical websocket topic helpers (provider-agnostic).
// Topics are expressed as "channel:SYMBOL" where SYMBOL is canonical (e.g., BTC-USDT).
const (
	TopicTrade       corepkg.TopicTemplate = "trade"
	TopicTicker      corepkg.TopicTemplate = "ticker"
	TopicBook        corepkg.TopicTemplate = "book"
	TopicUserOrder   corepkg.TopicTemplate = "order"
	TopicUserBalance corepkg.TopicTemplate = "balance"
)

func TradeTopic(symbol string) corepkg.Topic { return corepkg.Topic(string(TopicTrade) + ":" + symbol) }
func TickerTopic(symbol string) corepkg.Topic {
	return corepkg.Topic(string(TopicTicker) + ":" + symbol)
}
func BookTopic(symbol string) corepkg.Topic { return corepkg.Topic(string(TopicBook) + ":" + symbol) }
func UserOrderTopic(symbol string) corepkg.Topic {
	return corepkg.Topic(string(TopicUserOrder) + ":" + symbol)
}
func UserBalanceTopic() corepkg.Topic { return corepkg.Topic(TopicUserBalance) }
