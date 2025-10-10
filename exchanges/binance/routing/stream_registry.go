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
	var rec tradeRecord
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
	var rec tickerRecord
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
	event, err := eventTypeFromPayload(payload)
	if err != nil {
		return internal.WrapExchange(err, "decode depth")
	}
	var rec orderbookRecord
	if err := json.Unmarshal(payload, &rec); err != nil {
		return internal.WrapExchange(err, "decode depth")
	}
	symbol, err := r.deps.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return err
	}
	return parseOrderbookEvent(msg, &rec, symbol, event)
}

func (r *StreamRegistry) handleOrderUpdate(msg *RoutedMessage, payload []byte, stream string) error {
	return parseOrderUpdateEvent(msg, payload)
}

func (r *StreamRegistry) handleBalanceUpdate(msg *RoutedMessage, payload []byte, stream string) error {
	event, err := eventTypeFromPayload(payload)
	if err != nil {
		return err
	}
	return parseBalanceUpdateEvent(msg, payload, event)
}
