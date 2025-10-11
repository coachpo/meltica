package routing

import (
	"context"
	"strings"

	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/processors"
	gojson "github.com/goccy/go-json"
)

type symbolResolver interface {
	CanonicalSymbol(binanceSymbol string) (string, error)
}

type tradeProcessorAdapter struct {
	resolver symbolResolver
}

func NewTradeProcessorAdapter(resolver symbolResolver) processors.Processor {
	return &tradeProcessorAdapter{resolver: resolver}
}

func (p *tradeProcessorAdapter) Initialize(ctx context.Context) error {
	return contextCancelled(ctx)
}

func (p *tradeProcessorAdapter) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextCancelled(ctx); err != nil {
		return nil, err
	}
	payload, _ := unwrapCombinedPayload(raw)
	var rec tradeRecord
	if err := gojson.Unmarshal(payload, &rec); err != nil {
		return nil, decodeFailure("binance trade decode failed", err)
	}
	if strings.TrimSpace(rec.Symbol) == "" {
		return nil, invalidField("s", "symbol required", nil)
	}
	symbol, err := p.resolver.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return nil, err
	}
	msg := &corestreams.RoutedMessage{Raw: payload}
	if err := parseTradeEvent(msg, &rec, symbol); err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *tradeProcessorAdapter) MessageTypeID() string { return "binance.trade" }

type orderBookProcessorAdapter struct {
	resolver symbolResolver
}

func NewOrderBookProcessorAdapter(resolver symbolResolver) processors.Processor {
	return &orderBookProcessorAdapter{resolver: resolver}
}

func (p *orderBookProcessorAdapter) Initialize(ctx context.Context) error {
	return contextCancelled(ctx)
}

func (p *orderBookProcessorAdapter) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextCancelled(ctx); err != nil {
		return nil, err
	}
	payload, _ := unwrapCombinedPayload(raw)
	event, err := eventTypeFromPayload(payload)
	if err != nil {
		return nil, decodeFailure("binance orderbook decode failed", err)
	}
	var rec orderbookRecord
	if err := gojson.Unmarshal(payload, &rec); err != nil {
		return nil, decodeFailure("binance orderbook decode failed", err)
	}
	if strings.TrimSpace(rec.Symbol) == "" {
		return nil, invalidField("s", "symbol required", nil)
	}
	symbol, err := p.resolver.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return nil, err
	}
	msg := &corestreams.RoutedMessage{Raw: payload}
	if err := parseOrderbookEvent(msg, &rec, symbol, event); err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *orderBookProcessorAdapter) MessageTypeID() string { return "binance.orderbook" }

type tickerProcessorAdapter struct {
	resolver symbolResolver
}

func NewTickerProcessorAdapter(resolver symbolResolver) processors.Processor {
	return &tickerProcessorAdapter{resolver: resolver}
}

func (p *tickerProcessorAdapter) Initialize(ctx context.Context) error {
	return contextCancelled(ctx)
}

func (p *tickerProcessorAdapter) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextCancelled(ctx); err != nil {
		return nil, err
	}
	payload, _ := unwrapCombinedPayload(raw)
	var rec tickerRecord
	if err := gojson.Unmarshal(payload, &rec); err != nil {
		return nil, decodeFailure("binance ticker decode failed", err)
	}
	if strings.TrimSpace(rec.Symbol) == "" {
		return nil, invalidField("s", "symbol required", nil)
	}
	symbol, err := p.resolver.CanonicalSymbol(rec.Symbol)
	if err != nil {
		return nil, err
	}
	msg := &corestreams.RoutedMessage{Raw: payload}
	if err := parseTickerEvent(msg, &rec, symbol); err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *tickerProcessorAdapter) MessageTypeID() string { return "binance.ticker" }

type orderUpdateProcessorAdapter struct{}

func NewOrderUpdateProcessorAdapter() processors.Processor {
	return &orderUpdateProcessorAdapter{}
}

func (p *orderUpdateProcessorAdapter) Initialize(ctx context.Context) error {
	return contextCancelled(ctx)
}

func (p *orderUpdateProcessorAdapter) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextCancelled(ctx); err != nil {
		return nil, err
	}
	payload, _ := unwrapCombinedPayload(raw)
	msg := &corestreams.RoutedMessage{Raw: payload}
	if err := parseOrderUpdateEvent(msg, payload); err != nil {
		return nil, decodeFailure("binance order update decode failed", err)
	}
	return msg, nil
}

func (p *orderUpdateProcessorAdapter) MessageTypeID() string { return "binance.user.order" }

type balanceProcessorAdapter struct{}

func NewBalanceProcessorAdapter() processors.Processor {
	return &balanceProcessorAdapter{}
}

func (p *balanceProcessorAdapter) Initialize(ctx context.Context) error {
	return contextCancelled(ctx)
}

func (p *balanceProcessorAdapter) Process(ctx context.Context, raw []byte) (interface{}, error) {
	if err := contextCancelled(ctx); err != nil {
		return nil, err
	}
	payload, _ := unwrapCombinedPayload(raw)
	event, err := eventTypeFromPayload(payload)
	if err != nil {
		return nil, decodeFailure("binance balance decode failed", err)
	}
	msg := &corestreams.RoutedMessage{Raw: payload}
	if err := parseBalanceUpdateEvent(msg, payload, event); err != nil {
		return nil, err
	}
	return msg, nil
}

func (p *balanceProcessorAdapter) MessageTypeID() string { return "binance.user.balance" }

func contextCancelled(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errs.New("", errs.CodeInvalid, errs.WithMessage("processor context cancelled"), errs.WithCause(err))
	}
	return nil
}

func decodeFailure(message string, cause error) error {
	return errs.New("", errs.CodeInvalid, errs.WithMessage(message), errs.WithCause(cause))
}

func invalidField(field, message string, cause error) error {
	msg := field + " " + message
	opts := []errs.Option{errs.WithMessage(msg), errs.WithVenueField("field", field)}
	if cause != nil {
		opts = append(opts, errs.WithCause(cause))
	}
	return errs.New("", errs.CodeInvalid, opts...)
}
