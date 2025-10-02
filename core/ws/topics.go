package ws

// Canonical websocket topic helpers (provider-agnostic).
// Topics are expressed as "channel:SYMBOL" where SYMBOL is canonical (e.g., BTC-USDT).
const (
	// Public topics
	TopicTrade     = "trade"
	TopicTicker    = "ticker"
	TopicDepth     = "depth"
	TopicFullBook  = "book"
	TopicSnapshot5 = "book5"
	// Private topics
	TopicOrder   = "order"
	TopicBalance = "balance"
)

func TradeTopic(symbol string) string  { return TopicTrade + ":" + symbol }
func TickerTopic(symbol string) string { return TopicTicker + ":" + symbol }
func DepthTopic(symbol string) string  { return TopicDepth + ":" + symbol }
func OrderTopic(symbol string) string  { return TopicOrder + ":" + symbol }
func BalanceTopic() string             { return TopicBalance }
func BookTopic(symbol string) string   { return TopicFullBook + ":" + symbol }
func Book5Topic(symbol string) string  { return TopicSnapshot5 + ":" + symbol }
