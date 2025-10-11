package wsrouting

import (
	"bytes"
	"context"
	"strings"
	"time"

	gojson "github.com/goccy/go-json"

	"github.com/coachpo/meltica/errs"
	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

const (
	tradeType         = "binance.trade"
	orderBookType     = "binance.orderbook"
	tickerType        = "binance.ticker"
	orderUpdateType   = "binance.user.order"
	balanceUpdateType = "binance.user.balance"
)

// RawMessagePublisher delivers raw payloads downstream for domain-specific handling.
type RawMessagePublisher func(context.Context, string, []byte) error

// Pipeline bundles parser and publisher implementations for Binance streams.
type Pipeline struct {
	parser    wsrouting.Parser
	publisher wsrouting.PublishFunc
}

// Parser exposes the parser implementation.
func (p *Pipeline) Parser() wsrouting.Parser {
	if p == nil {
		return nil
	}
	return p.parser
}

// Publisher exposes the publish function.
func (p *Pipeline) Publisher() wsrouting.PublishFunc {
	if p == nil {
		return nil
	}
	return p.publisher
}

// NewPipeline constructs parser and publisher wiring for Binance websocket events.
func NewPipeline(sink RawMessagePublisher) (*Pipeline, error) {
	if sink == nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("binance pipeline requires raw message publisher"))
	}
	parser := &binanceParser{}
	return &Pipeline{
		parser:    parser,
		publisher: newPublisher(sink),
	}, nil
}

type binanceParser struct{}

func (p *binanceParser) Parse(ctx context.Context, raw []byte) (*wsrouting.Message, error) {
	if len(raw) == 0 {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("payload required"))
	}
	payload, stream := unwrapCombinedPayload(raw)
	if len(payload) == 0 {
		payload = raw
	}
	event, err := eventType(payload)
	if err != nil {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("failed to decode event"), errs.WithCause(err))
	}
	if event == "" {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("event type missing"))
	}
	typeID, ok := mapEventToType(event)
	if !ok {
		return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("unsupported event"), errs.WithVenueField("event", event))
	}
	metadata := map[string]string{"event": event}
	if stream != "" {
		metadata["stream"] = stream
	}
	clone := append([]byte(nil), payload...)
	return &wsrouting.Message{
		Type:       typeID,
		Payload:    map[string]any{"raw": clone},
		Metadata:   metadata,
		ReceivedAt: time.Now().UTC(),
	}, nil
}

func mapEventToType(event string) (string, bool) {
	switch strings.ToLower(event) {
	case "trade":
		return tradeType, true
	case "depthupdate":
		return orderBookType, true
	case "24hrticker":
		return tickerType, true
	case "order_trade_update":
		return orderUpdateType, true
	case "balanceupdate", "outboundaccountposition":
		return balanceUpdateType, true
	default:
		return "", false
	}
}

// eventType extracts the Binance event identifier from the payload.
func eventType(payload []byte) (string, error) {
	fields, err := decodePayloadFields(payload)
	if err != nil {
		return "", err
	}
	if rawEvent, ok := fields["e"]; ok {
		var value string
		if err := gojson.Unmarshal(rawEvent, &value); err != nil {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}
	return "", nil
}

func newPublisher(sink RawMessagePublisher) wsrouting.PublishFunc {
	return func(ctx context.Context, msg *wsrouting.Message) error {
		if msg == nil {
			return errs.New("", errs.CodeInvalid, errs.WithMessage("message required"))
		}
		payload, ok := msg.Payload["raw"]
		if !ok {
			return errs.New("", errs.CodeExchange, errs.WithMessage("missing raw payload"))
		}
		bytesPayload, ok := payload.([]byte)
		if !ok {
			return errs.New("", errs.CodeExchange, errs.WithMessage("invalid raw payload type"))
		}
		return sink(ctx, msg.Type, bytesPayload)
	}
}

func decodePayloadFields(raw []byte) (map[string]gojson.RawMessage, error) {
	dec := gojson.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var fields map[string]gojson.RawMessage
	if err := dec.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func unwrapCombinedPayload(raw []byte) ([]byte, string) {
	var envelope struct {
		Stream string            `json:"stream"`
		Data   gojson.RawMessage `json:"data"`
	}
	if err := gojson.Unmarshal(raw, &envelope); err == nil && envelope.Stream != "" && len(envelope.Data) > 0 {
		return envelope.Data, envelope.Stream
	}
	return raw, ""
}
