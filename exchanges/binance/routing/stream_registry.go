package routing

import (
	"encoding/json"
	"strings"

	"github.com/coachpo/meltica/exchanges/binance/internal"
)

// StreamKind identifies the type of stream based on its suffix.
type StreamKind string

const (
	StreamKindTrade      StreamKind = "trade"
	StreamKindBookTicker StreamKind = "bookTicker"
	StreamKindDepth      StreamKind = "depth"
	StreamKindOrder      StreamKind = "ORDER_TRADE_UPDATE"
	StreamKindBalance    StreamKind = "balanceUpdate"
	StreamKindAccount    StreamKind = "outboundAccountPosition"
)

// StreamDescriptor captures metadata about a subscribed stream upfront.
type StreamDescriptor struct {
	RawName        string
	CanonicalSymbol string
	Kind           StreamKind
}

// StreamHandler processes a raw message and populates a RoutedMessage.
type StreamHandler func(*RoutedMessage, []byte, string) error

// StreamRegistry maps StreamKind to handlers, avoiding repeated parsing.
type StreamRegistry struct {
	handlers map[StreamKind]StreamHandler
	deps     WSDependencies
}

// NewStreamRegistry creates a registry with all known handlers.
func NewStreamRegistry(deps WSDependencies) *StreamRegistry {
	r := &StreamRegistry{
		handlers: make(map[StreamKind]StreamHandler),
		deps:     deps,
	}

	r.handlers[StreamKindTrade] = r.handleTrade
	r.handlers[StreamKindBookTicker] = r.handleBookTicker
	r.handlers[StreamKindDepth] = r.handleDepth
	r.handlers[StreamKindOrder] = r.handleOrderUpdate
	r.handlers[StreamKindBalance] = r.handleBalanceUpdate
	r.handlers[StreamKindAccount] = r.handleAccountPosition

	return r
}

// Dispatch routes a message using the stream kind.
func (r *StreamRegistry) Dispatch(msg *RoutedMessage, payload []byte, stream string, kind StreamKind) error {
	handler, ok := r.handlers[kind]
	if !ok {
		// Unknown stream kind - skip
		return nil
	}
	return handler(msg, payload, stream)
}

// ParseStreamKind determines the kind from stream name or event type.
func ParseStreamKind(stream string, eventType string) StreamKind {
	if eventType != "" {
		switch eventType {
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
	case strings.Contains(stream, "@bookTicker"):
		return StreamKindBookTicker
	case strings.Contains(stream, "@depth"):
		return StreamKindDepth
	default:
		return ""
	}
}

// Handler implementations

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

func (r *StreamRegistry) handleBookTicker(msg *RoutedMessage, payload []byte, stream string) error {
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
		return internal.WrapExchange(err, "decode book ticker")
	}

	symbol, err := r.deps.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return err
	}

	return parseBookTickerEvent(msg, &rec, symbol)
}

func (r *StreamRegistry) handleDepth(msg *RoutedMessage, payload []byte, stream string) error {
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

	return parseDepthEvent(msg, &rec, symbol)
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
	return parseBalanceUpdateEvent(msg, payload, "outboundAccountPosition")
}
