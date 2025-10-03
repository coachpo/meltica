package ws

// Canonical websocket topic helpers (provider-agnostic).
// Topics are expressed as "channel:SYMBOL" where SYMBOL is canonical (e.g., BTC-USDT).
const (
	TopicTrade       = "trade"
	TopicTicker      = "ticker"
	TopicBook        = "book"
	TopicUserOrder   = "order"
	TopicUserBalance = "balance"
)

func TradeTopic(symbol string) string     { return TopicTrade + ":" + symbol }
func TickerTopic(symbol string) string    { return TopicTicker + ":" + symbol }
func UserOrderTopic(symbol string) string { return TopicUserOrder + ":" + symbol }
func UserBalanceTopic() string            { return TopicUserBalance }
func BookTopic(symbol string) string      { return TopicBook + ":" + symbol }
