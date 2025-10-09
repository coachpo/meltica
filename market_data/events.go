package market_data

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
)

type EventType string

const (
	EventTypeTrade         EventType = "trade"
	EventTypeOrderBook     EventType = "order_book"
	EventTypeFunding       EventType = "funding"
	EventTypeAccountUpdate EventType = "account"
)

type Event struct {
	Type         EventType
	Venue        core.ExchangeName
	Symbol       string
	NativeSymbol string
	Timestamp    time.Time
	Sequence     uint64
	Metadata     map[string]string
	Payload      EventPayload
}

type EventPayload interface {
	eventPayload()
}

type TradePayload struct {
	Price        *big.Rat
	Quantity     *big.Rat
	Side         core.OrderSide
	IsTaker      bool
	VenueTradeID string
}

func (*TradePayload) eventPayload() {}

type OrderBookPayload struct {
	Snapshot bool
	Bids     []core.BookDepthLevel
	Asks     []core.BookDepthLevel
}

func (*OrderBookPayload) eventPayload() {}

type FundingPayload struct {
	Rate        *big.Rat
	Interval    time.Duration
	EffectiveAt time.Time
}

func (*FundingPayload) eventPayload() {}

type AccountEventKind string

const (
	AccountUnknown      AccountEventKind = "unknown"
	AccountBalanceDelta AccountEventKind = "balance_delta"
)

type AccountBalance struct {
	Asset     string
	Total     *big.Rat
	Available *big.Rat
}

type AccountPayload struct {
	Kind     AccountEventKind
	Balances []AccountBalance
}

func (*AccountPayload) eventPayload() {}

func ParseEvent(exchange core.ExchangeName, raw []byte) (*Event, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("market_data: decode envelope: %w", err)
	}

	rawType, err := readString(envelope, "type")
	if err != nil {
		return nil, err
	}

	rawSymbol, err := readString(envelope, "symbol")
	if err != nil {
		return nil, err
	}
	nativeSymbol, _ := readString(envelope, "native_symbol")

	canonical, native, err := resolveSymbols(rawSymbol, nativeSymbol)
	if err != nil {
		return nil, err
	}

	timestamp, err := readTimestamp(envelope, "timestamp")
	if err != nil {
		return nil, err
	}

	seq, err := readUint64(envelope, "sequence")
	if err != nil {
		return nil, err
	}

	evt := &Event{
		Type:         canonicalEventType(rawType),
		Venue:        exchange,
		Symbol:       canonical,
		NativeSymbol: native,
		Timestamp:    timestamp,
		Sequence:     seq,
		Metadata:     map[string]string{"raw_type": strings.ToLower(rawType)},
	}

	payload, err := buildPayload(evt.Type, raw)
	if err != nil {
		return nil, err
	}
	evt.Payload = payload
	return evt, nil
}

func buildPayload(kind EventType, raw []byte) (EventPayload, error) {
	switch kind {
	case EventTypeTrade:
		return buildTradePayload(raw)
	case EventTypeOrderBook:
		return buildOrderBookPayload(raw)
	case EventTypeFunding:
		return buildFundingPayload(raw)
	case EventTypeAccountUpdate:
		return buildAccountPayload(raw)
	default:
		return nil, fmt.Errorf("market_data: unsupported event type %q", kind)
	}
}

func buildTradePayload(raw []byte) (*TradePayload, error) {
	var t struct {
		Price    string `json:"price"`
		Quantity string `json:"quantity"`
		Side     string `json:"side"`
		Taker    bool   `json:"taker"`
		TradeID  string `json:"trade_id"`
	}
	if err := json.Unmarshal(raw, &t); err != nil {
		return nil, fmt.Errorf("market_data: trade payload decode: %w", err)
	}

	price, err := parsePrecise("price", t.Price)
	if err != nil {
		return nil, err
	}
	qty, err := parsePrecise("quantity", t.Quantity)
	if err != nil {
		return nil, err
	}
	side, err := parseSide(t.Side)
	if err != nil {
		return nil, err
	}

	return &TradePayload{
		Price:        price,
		Quantity:     qty,
		Side:         side,
		IsTaker:      t.Taker,
		VenueTradeID: t.TradeID,
	}, nil
}

func buildOrderBookPayload(raw []byte) (*OrderBookPayload, error) {
	var b struct {
		Snapshot bool `json:"snapshot"`
		Bids     []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"bids"`
		Asks []struct {
			Price    string `json:"price"`
			Quantity string `json:"quantity"`
		} `json:"asks"`
	}
	if err := json.Unmarshal(raw, &b); err != nil {
		return nil, fmt.Errorf("market_data: order book payload decode: %w", err)
	}

	bids := make([]core.BookDepthLevel, len(b.Bids))
	for i, level := range b.Bids {
		price, err := parsePrecise("bid.price", level.Price)
		if err != nil {
			return nil, err
		}
		qty, err := parsePrecise("bid.quantity", level.Quantity)
		if err != nil {
			return nil, err
		}
		bids[i] = core.BookDepthLevel{Price: price, Qty: qty}
	}

	asks := make([]core.BookDepthLevel, len(b.Asks))
	for i, level := range b.Asks {
		price, err := parsePrecise("ask.price", level.Price)
		if err != nil {
			return nil, err
		}
		qty, err := parsePrecise("ask.quantity", level.Quantity)
		if err != nil {
			return nil, err
		}
		asks[i] = core.BookDepthLevel{Price: price, Qty: qty}
	}

	return &OrderBookPayload{Snapshot: b.Snapshot, Bids: bids, Asks: asks}, nil
}

func buildFundingPayload(raw []byte) (*FundingPayload, error) {
	var f struct {
		Rate         string `json:"rate"`
		IntervalHour int    `json:"interval_hours"`
		NextFunding  string `json:"next_funding"`
	}
	if err := json.Unmarshal(raw, &f); err != nil {
		return nil, fmt.Errorf("market_data: funding payload decode: %w", err)
	}

	rate, err := parsePrecise("rate", f.Rate)
	if err != nil {
		return nil, err
	}

	effectiveAt, err := time.Parse(time.RFC3339, f.NextFunding)
	if err != nil {
		return nil, fmt.Errorf("market_data: funding next_funding parse: %w", err)
	}

	interval := time.Duration(f.IntervalHour) * time.Hour
	if f.IntervalHour <= 0 {
		interval = 0
	}

	return &FundingPayload{Rate: rate, Interval: interval, EffectiveAt: effectiveAt}, nil
}

func buildAccountPayload(raw []byte) (*AccountPayload, error) {
	var a struct {
		Reason   string `json:"reason"`
		Balances []struct {
			Asset     string `json:"asset"`
			Total     string `json:"total"`
			Available string `json:"available"`
		} `json:"balances"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, fmt.Errorf("market_data: account payload decode: %w", err)
	}

	balances := make([]AccountBalance, len(a.Balances))
	for i, b := range a.Balances {
		total, err := parsePrecise("balance.total", b.Total)
		if err != nil {
			return nil, err
		}
		available, err := parsePrecise("balance.available", b.Available)
		if err != nil {
			return nil, err
		}
		balances[i] = AccountBalance{Asset: strings.ToUpper(b.Asset), Total: total, Available: available}
	}

	kind := AccountUnknown
	switch strings.ToLower(a.Reason) {
	case "", "balance_update", "balance_delta":
		kind = AccountBalanceDelta
	}

	return &AccountPayload{Kind: kind, Balances: balances}, nil
}

func resolveSymbols(symbol, native string) (string, string, error) {
	candidate := symbol
	if candidate == "" {
		candidate = native
	}
	canonical, err := core.CanonicalSymbolFor(candidate)
	if err != nil {
		return "", "", fmt.Errorf("market_data: canonical symbol: %w", err)
	}
	nativeSymbol := native
	if nativeSymbol == "" {
		nativeSymbol = candidate
	}
	return canonical, strings.ToUpper(nativeSymbol), nil
}

func canonicalEventType(raw string) EventType {
	switch strings.ToLower(raw) {
	case "trade":
		return EventTypeTrade
	case "book", "order_book":
		return EventTypeOrderBook
	case "funding":
		return EventTypeFunding
	case "account", "account_update":
		return EventTypeAccountUpdate
	default:
		return EventType(strings.ToLower(raw))
	}
}

func readString(raw map[string]json.RawMessage, key string) (string, error) {
	data, ok := raw[key]
	if !ok {
		return "", fmt.Errorf("market_data: missing field %s", key)
	}
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return "", fmt.Errorf("market_data: field %s decode: %w", key, err)
	}
	return value, nil
}

func readTimestamp(raw map[string]json.RawMessage, key string) (time.Time, error) {
	str, err := readString(raw, key)
	if err != nil {
		return time.Time{}, err
	}
	ts, err := time.Parse(time.RFC3339Nano, str)
	if err != nil {
		return time.Time{}, fmt.Errorf("market_data: timestamp %s parse: %w", key, err)
	}
	return ts, nil
}

func readUint64(raw map[string]json.RawMessage, key string) (uint64, error) {
	data, ok := raw[key]
	if !ok {
		return 0, nil
	}
	var number json.Number
	if err := json.Unmarshal(data, &number); err != nil {
		return 0, fmt.Errorf("market_data: field %s decode: %w", key, err)
	}
	if number == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(number.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("market_data: field %s parse: %w", key, err)
	}
	return value, nil
}

func parsePrecise(field, value string) (*big.Rat, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, fmt.Errorf("market_data: field %s cannot be empty", field)
	}
	rat, ok := new(big.Rat).SetString(trimmed)
	if !ok {
		return nil, fmt.Errorf("market_data: field %s invalid rational %q", field, value)
	}
	return rat, nil
}

func parseSide(raw string) (core.OrderSide, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "buy":
		return core.SideBuy, nil
	case "sell":
		return core.SideSell, nil
	default:
		return "", fmt.Errorf("market_data: unsupported side %q", raw)
	}
}

func (e *Event) Canonical() string {
	if e == nil {
		return ""
	}
	return e.Symbol
}

func (e *Event) Native() string {
	if e == nil {
		return ""
	}
	return e.NativeSymbol
}
