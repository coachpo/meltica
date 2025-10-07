package routing

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coachpo/meltica/exchanges/binance/internal"
)

type StreamKind string

const (
	StreamKindTrade     StreamKind = "trade"
	StreamKindTicker    StreamKind = "ticker"
	StreamKindBookDelta StreamKind = "depth@100ms"
	StreamKindOrder     StreamKind = "order"
	StreamKindBalance   StreamKind = "balance"
	StreamKindAccount   StreamKind = "outboundAccountPosition"
)

func (k StreamKind) String() string { return string(k) }

var streamKindIndex = func() map[string]StreamKind {
	kinds := []StreamKind{
		StreamKindTrade,
		StreamKindTicker,
		StreamKindBookDelta,
		StreamKindOrder,
		StreamKindBalance,
		StreamKindAccount,
	}
	index := make(map[string]StreamKind, len(kinds))
	for _, kind := range kinds {
		index[strings.ToLower(kind.String())] = kind
	}
	return index
}()

func streamKindRequiresSymbol(kind StreamKind) bool {
	switch kind {
	case StreamKindBalance, StreamKindAccount:
		return false
	default:
		return true
	}
}

func topicForSymbol(kind StreamKind, symbol string) string {
	trimmed := strings.TrimSpace(symbol)
	if streamKindRequiresSymbol(kind) {
		if trimmed == "" {
			panic(fmt.Sprintf("topics: empty symbol for stream kind %s", kind))
		}
		return buildTopic(kind.String(), trimmed)
	}
	if trimmed == "" {
		return kind.String()
	}
	return buildTopic(kind.String(), trimmed)
}

func Trade(symbol string) string     { return topicForSymbol(StreamKindTrade, symbol) }
func Ticker(symbol string) string    { return topicForSymbol(StreamKindTicker, symbol) }
func OrderBook(symbol string) string { return topicForSymbol(StreamKindBookDelta, symbol) }
func UserOrder(symbol string) string { return topicForSymbol(StreamKindOrder, symbol) }
func UserBalance() string            { return topicForSymbol(StreamKindBalance, "") }
func AccountSnapshot() string        { return topicForSymbol(StreamKindAccount, "") }

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

func Parse(topic string) (StreamKind, string, error) {
	trimmed := strings.TrimSpace(topic)
	if trimmed == "" {
		return "", "", fmt.Errorf("topics: empty topic")
	}
	channel := trimmed
	symbol := ""
	if idx := strings.IndexByte(trimmed, ':'); idx >= 0 {
		channel = strings.TrimSpace(trimmed[:idx])
		symbol = strings.TrimSpace(trimmed[idx+1:])
	}
	kind, ok := streamKindIndex[strings.ToLower(channel)]
	if !ok {
		return "", "", fmt.Errorf("topics: unknown channel %q", channel)
	}
	if streamKindRequiresSymbol(kind) && symbol == "" {
		return "", "", fmt.Errorf("topics: missing symbol for %s", kind)
	}
	return kind, symbol, nil
}

type StreamHandler func(*RoutedMessage, []byte, string) error

type StreamRegistry struct {
	handlers map[StreamKind]StreamHandler
	deps     WSDependencies
}

func NewStreamRegistry(deps WSDependencies) *StreamRegistry {
	r := &StreamRegistry{
		handlers: make(map[StreamKind]StreamHandler),
		deps:     deps,
	}

	r.handlers[StreamKindTrade] = r.handleTrade
	r.handlers[StreamKindTicker] = r.handleTicker
	r.handlers[StreamKindBookDelta] = r.handleOrderbook
	r.handlers[StreamKindOrder] = r.handleOrderUpdate
	r.handlers[StreamKindBalance] = r.handleBalanceUpdate
	r.handlers[StreamKindAccount] = r.handleAccountPosition

	return r
}

func (r *StreamRegistry) Dispatch(msg *RoutedMessage, payload []byte, stream string, kind StreamKind) error {
	handler, ok := r.handlers[kind]
	if !ok {
		return nil
	}
	return handler(msg, payload, stream)
}

func ParseStreamKind(stream string, eventType string) StreamKind {
	if eventType != "" {
		switch eventType {
		case "24hrTicker":
			return StreamKindTicker
		case "ORDER_TRADE_UPDATE":
			return StreamKindOrder
		case "balanceUpdate":
			return StreamKindBalance
		case "outboundAccountPosition":
			return StreamKindAccount
		}
	}
	switch {
	case strings.Contains(stream, "@trade"):
		return StreamKindTrade
	case strings.Contains(stream, "@ticker"):
		return StreamKindTicker
	case strings.Contains(stream, "@depth"):
		return StreamKindBookDelta
	default:
		return ""
	}
}

func (r *StreamRegistry) handleTrade(msg *RoutedMessage, payload []byte, stream string) error {
	var rec struct {
		Symbol string      `json:"s"`
		Price  json.Number `json:"p"`
		Qty    json.Number `json:"q"`
		Time   int64       `json:"T"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode trade event")
	}
	symbol, err := r.deps.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return err
	}
	return parseTradeEvent(msg, &rec, symbol)
}

func (r *StreamRegistry) handleTicker(msg *RoutedMessage, payload []byte, stream string) error {
	var rec struct {
		EventType string      `json:"e"`
		EventTime int64       `json:"E"`
		Symbol    string      `json:"s"`
		BidPrice  json.Number `json:"b"`
		BidQty    json.Number `json:"B"`
		AskPrice  json.Number `json:"a"`
		AskQty    json.Number `json:"A"`
		LastPrice json.Number `json:"c"`
		LastQty   json.Number `json:"Q"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode ticker event")
	}
	symbol, err := r.deps.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return err
	}
	return parseTickerEvent(msg, &rec, symbol)
}

func (r *StreamRegistry) handleOrderbook(msg *RoutedMessage, payload []byte, stream string) error {
	var rec struct {
		Event         string          `json:"e"`
		Symbol        string          `json:"s"`
		FirstUpdateID int64           `json:"U"`
		LastUpdateID  int64           `json:"u"`
		Bids          [][]interface{} `json:"b"`
		Asks          [][]interface{} `json:"a"`
		EventTime     int64           `json:"E"`
	}
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode depth")
	}
	symbol, err := r.deps.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return err
	}
	return parseOrderbookEvent(msg, &rec, symbol)
}

func (r *StreamRegistry) handleOrderUpdate(msg *RoutedMessage, payload []byte, stream string) error {
	return parseOrderUpdateEvent(msg, payload)
}

func (r *StreamRegistry) handleBalanceUpdate(msg *RoutedMessage, payload []byte, stream string) error {
	var env struct {
		Event string `json:"e"`
	}
	if err := json.Unmarshal(payload, &env); err != nil {
		return err
	}
	return parseBalanceUpdateEvent(msg, payload, env.Event)
}

func (r *StreamRegistry) handleAccountPosition(msg *RoutedMessage, payload []byte, stream string) error {
	return parseBalanceUpdateEvent(msg, payload, StreamKindAccount.String())
}
