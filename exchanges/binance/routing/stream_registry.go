package routing

import (
	"encoding/json"
	"strings"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/internal"
)

const accountSnapshotEvent = "outboundAccountPosition"

func topicForSymbol(topic core.Topic, symbol string) string {
	return core.MustCanonicalTopic(topic, symbol)
}

func Trade(symbol string) string     { return topicForSymbol(core.TopicTrade, symbol) }
func Ticker(symbol string) string    { return topicForSymbol(core.TopicTicker, symbol) }
func OrderBook(symbol string) string { return topicForSymbol(core.TopicBookDelta, symbol) }
func UserOrder(symbol string) string { return topicForSymbol(core.TopicUserOrder, symbol) }
func UserBalance() string            { return topicForSymbol(core.TopicUserBalance, "") }

func Parse(topic string) (core.Topic, string, error) {
	canonical, symbol, err := core.ParseTopic(topic)
	if err == nil {
		return canonical, symbol, nil
	}
	trimmed := strings.TrimSpace(strings.ToLower(topic))
	if trimmed == accountSnapshotEvent {
		return core.TopicUserBalance, "", nil
	}
	return "", "", err
}

type StreamHandler func(*RoutedMessage, []byte, string) error

type StreamRegistry struct {
	handlers map[core.Topic]StreamHandler
	deps     WSDependencies
}

func NewStreamRegistry(deps WSDependencies) *StreamRegistry {
	r := &StreamRegistry{
		handlers: make(map[core.Topic]StreamHandler),
		deps:     deps,
	}

	r.handlers[core.TopicTrade] = r.handleTrade
	r.handlers[core.TopicTicker] = r.handleTicker
	r.handlers[core.TopicBookDelta] = r.handleOrderbook
	r.handlers[core.TopicUserOrder] = r.handleOrderUpdate
	r.handlers[core.TopicUserBalance] = r.handleBalanceUpdate

	return r
}

func (r *StreamRegistry) Dispatch(msg *RoutedMessage, payload []byte, stream string, kind core.Topic) error {
	handler, ok := r.handlers[kind]
	if !ok {
		return nil
	}
	return handler(msg, payload, stream)
}

func ParseStreamKind(stream string, eventType string) core.Topic {
	if eventType != "" {
		switch eventType {
		case "24hrTicker":
			return core.TopicTicker
		case "ORDER_TRADE_UPDATE":
			return core.TopicUserOrder
		case "balanceUpdate":
			return core.TopicUserBalance
		case accountSnapshotEvent:
			return core.TopicUserBalance
		}
	}
	switch {
	case strings.Contains(stream, "@trade"):
		return core.TopicTrade
	case strings.Contains(stream, "@ticker"):
		return core.TopicTicker
	case strings.Contains(stream, "@depth"):
		return core.TopicBookDelta
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
