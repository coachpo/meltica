// Package binance provides adapters for Binance market data integration.
package binance

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

// Parser normalises Binance payloads into canonical events.
type Parser struct {
	pools        *pool.PoolManager
	providerName string
}

const parserAcquireWarnDelay = 250 * time.Millisecond

// NewParser creates a Binance payload parser.
func NewParser() *Parser {
	return NewParserWithPool("binance", nil)
}

// NewParserWithPool creates a parser configured with pooling support.
func NewParserWithPool(providerName string, pools *pool.PoolManager) *Parser {
	if providerName == "" {
		providerName = "binance"
	}
	return &Parser{pools: pools, providerName: providerName}
}

// Parse converts a websocket frame into canonical events.
func (p *Parser) Parse(ctx context.Context, frame []byte, ingestTS time.Time) ([]*schema.Event, error) {
	var envelope wsEnvelope
	if err := json.Unmarshal(frame, &envelope); err != nil {
		return nil, fmt.Errorf("parse binance ws frame: %w", err)
	}

	meta := make(map[string]json.RawMessage)
	if err := json.Unmarshal(envelope.Data, &meta); err != nil {
		// Binance sends non-data messages like subscription responses: {"result":null,"id":1}
		// These aren't errors, just skip them
		//nolint:nilerr // Non-data messages are expected and should be skipped silently
		return nil, nil
	}
	var eventType string
	if rawType, ok := meta["e"]; ok {
		if err := json.Unmarshal(rawType, &eventType); err != nil {
			return nil, fmt.Errorf("parse binance ws event type: %w", err)
		}
	}
	if eventType == "" {
		eventType = inferStreamType(envelope.Stream)
	}
	
	// Skip non-data messages (subscription confirmations, etc)
	if eventType == "" {
		return nil, nil
	}

	switch strings.ToLower(eventType) {
	case "depthupdate":
		return p.parseDepthUpdate(ctx, envelope.Stream, envelope.Data, ingestTS)
	case "aggtrade":
		return p.parseAggTrade(ctx, envelope.Stream, envelope.Data, ingestTS)
	case "24hrticker", "ticker":
		return p.parseTicker(ctx, envelope.Stream, envelope.Data, ingestTS)
	case "kline":
		return p.parseKline(ctx, envelope.Stream, envelope.Data, ingestTS)
	case "executionreport":
		return p.parseExecutionReport(ctx, envelope.Stream, envelope.Data, ingestTS)
	default:
		return nil, fmt.Errorf("unsupported binance ws event %s", eventType)
	}
}

// ParseSnapshot converts a REST snapshot payload into canonical events based on the parser hint.
func (p *Parser) ParseSnapshot(ctx context.Context, name string, body []byte, ingestTS time.Time) ([]*schema.Event, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "orderbook":
		return p.parseOrderbookSnapshot(ctx, body, ingestTS)
	default:
		return nil, fmt.Errorf("unsupported rest parser %s", name)
	}
}

func (p *Parser) parseDepthUpdate(ctx context.Context, stream string, data []byte, ingestTS time.Time) ([]*schema.Event, error) {
	_ = stream
	var payload depthUpdate
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode depth update: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in depth update")
	}
	seq := uint64(payload.FinalUpdateID)
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	// Binance sends deltas, but we emit as snapshot metadata.
	// The BookAssembler will apply deltas and return full snapshots.
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeBookSnapshot, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeBookSnapshot
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	// Store delta as snapshot payload - provider will handle assembly
	// Include Binance-specific U and u for proper sequence validation
	evt.Payload = schema.BookSnapshotPayload{
		Bids:          toPriceLevels(payload.Bids),
		Asks:          toPriceLevels(payload.Asks),
		Checksum:      payload.Checksum,
		LastUpdate:    time.UnixMilli(payload.EventTime).UTC(),
		FirstUpdateID: payload.FirstUpdateID, // U - First update ID
		FinalUpdateID: payload.FinalUpdateID, // u - Final update ID
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) parseAggTrade(ctx context.Context, stream string, data []byte, ingestTS time.Time) ([]*schema.Event, error) {
	_ = stream
	var payload aggTrade
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode agg trade: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in agg trade")
	}
	seq := uint64(payload.TradeID)
	side := schema.TradeSideBuy
	if payload.IsBuyerMaker {
		side = schema.TradeSideSell
	}
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeTrade, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeTrade
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	evt.Payload = schema.TradePayload{
		TradeID:   fmt.Sprintf("%d", payload.TradeID),
		Side:      side,
		Price:     payload.Price,
		Quantity:  payload.Quantity,
		Timestamp: time.UnixMilli(payload.EventTime).UTC(),
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) parseTicker(ctx context.Context, stream string, data []byte, ingestTS time.Time) ([]*schema.Event, error) {
	_ = stream
	var payload ticker24hr
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode ticker: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in ticker")
	}
	if payload.EventTime < 0 {
		return nil, fmt.Errorf("negative ticker event time: %d", payload.EventTime)
	}
	seq := uint64(payload.EventTime)
	if seq == 0 {
		ingestNano := ingestTS.UnixNano()
		if ingestNano < 0 {
			return nil, fmt.Errorf("negative ingest timestamp: %d", ingestNano)
		}
		seq = uint64(ingestNano)
	}
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeTicker, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeTicker
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	evt.Payload = schema.TickerPayload{
		LastPrice: payload.LastPrice,
		BidPrice:  payload.BidPrice,
		AskPrice:  payload.AskPrice,
		Volume24h: payload.Volume,
		Timestamp: time.UnixMilli(payload.EventTime).UTC(),
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) parseOrderbookSnapshot(ctx context.Context, body []byte, ingestTS time.Time) ([]*schema.Event, error) {
	var payload orderbookSnapshot
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decode orderbook snapshot: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in orderbook snapshot")
	}
	seq := uint64(payload.LastUpdateID)
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeBookSnapshot, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeBookSnapshot
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	evt.Payload = schema.BookSnapshotPayload{
		Bids:          toPriceLevels(payload.Bids),
		Asks:          toPriceLevels(payload.Asks),
		Checksum:      payload.Checksum,
		LastUpdate:    time.UnixMilli(payload.EventTime).UTC(),
		FirstUpdateID: 0,                   // REST snapshots don't have FirstUpdateID (U)
		FinalUpdateID: payload.LastUpdateID, // REST snapshots have lastUpdateId as FinalUpdateID
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) parseKline(ctx context.Context, stream string, data []byte, ingestTS time.Time) ([]*schema.Event, error) {
	_ = stream
	var payload klineEvent
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode kline: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in kline")
	}
	if payload.EventTime < 0 {
		return nil, fmt.Errorf("negative kline event time: %d", payload.EventTime)
	}
	seq := uint64(payload.EventTime) //nolint:gosec // timestamp conversion is safe after validation
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeKlineSummary, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeKlineSummary
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	evt.Payload = schema.KlineSummaryPayload{
		OpenPrice:  payload.Kline.Open,
		ClosePrice: payload.Kline.Close,
		HighPrice:  payload.Kline.High,
		LowPrice:   payload.Kline.Low,
		Volume:     payload.Kline.Volume,
		OpenTime:   time.UnixMilli(payload.Kline.StartTime).UTC(),
		CloseTime:  time.UnixMilli(payload.Kline.CloseTime).UTC(),
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) parseExecutionReport(ctx context.Context, stream string, data []byte, ingestTS time.Time) ([]*schema.Event, error) {
	_ = stream
	var payload executionReport
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("decode execution report: %w", err)
	}
	symbol := canonicalInstrument(payload.Symbol)
	if symbol == "" {
		return nil, fmt.Errorf("missing symbol in execution report")
	}
	seq := uint64(payload.OrderID)
	
	// Map Binance order status to our ExecReportState
	var state schema.ExecReportState
	switch strings.ToUpper(payload.OrderStatus) {
	case "NEW":
		state = schema.ExecReportStateACK
	case "PARTIALLY_FILLED":
		state = schema.ExecReportStatePARTIAL
	case "FILLED":
		state = schema.ExecReportStateFILLED
	case "CANCELED", "CANCELLED":
		state = schema.ExecReportStateCANCELLED
	case "REJECTED":
		state = schema.ExecReportStateREJECTED
	case "EXPIRED":
		state = schema.ExecReportStateEXPIRED
	default:
		state = schema.ExecReportStateACK
	}
	
	// Map Binance side to our TradeSide
	var side schema.TradeSide
	if strings.ToUpper(payload.Side) == "BUY" {
		side = schema.TradeSideBuy
	} else {
		side = schema.TradeSideSell
	}
	
	// Map Binance order type to our OrderType
	var orderType schema.OrderType
	if strings.ToUpper(payload.OrderType) == "MARKET" {
		orderType = schema.OrderTypeMarket
	} else {
		orderType = schema.OrderTypeLimit
	}
	
	evt, err := p.acquireCanonicalEvent(ctx)
	if err != nil {
		return nil, err
	}
	evt.EventID = buildEventID(p.providerName, symbol, schema.EventTypeExecReport, seq)
	evt.Provider = p.providerName
	evt.Symbol = symbol
	evt.Type = schema.EventTypeExecReport
	evt.SeqProvider = seq
	evt.IngestTS = ingestTS
	evt.EmitTS = ingestTS
	
	var rejectReason *string
	if payload.OrderRejectReason != "" {
		rejectReason = &payload.OrderRejectReason
	}
	
	evt.Payload = schema.ExecReportPayload{
		ClientOrderID:   payload.ClientOrderID,
		ExchangeOrderID: fmt.Sprintf("%d", payload.OrderID),
		State:           state,
		Side:            side,
		OrderType:       orderType,
		Price:           payload.Price,
		Quantity:        payload.OrigQty,
		FilledQuantity:  payload.CumQty,
		RemainingQty:    payload.OrigQty, // Calculate remaining if needed
		AvgFillPrice:    payload.LastFilledPrice,
		Timestamp:       time.UnixMilli(payload.TransactionTime).UTC(),
		RejectReason:    rejectReason,
	}
	return []*schema.Event{evt}, nil
}

func (p *Parser) acquireCanonicalEvent(ctx context.Context) (*schema.Event, error) {
	if p.pools == nil {
		return nil, errors.New("event pool unavailable")
	}
	requestCtx := ctx
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	start := time.Now()
	obj, err := p.pools.Get(requestCtx, "Event")
	if err != nil {
		return nil, fmt.Errorf("acquire event: %w", err)
	}
	if waited := time.Since(start); waited >= parserAcquireWarnDelay {
		log.Printf("parser: waited %s for Event pool", waited)
	}
	evt, ok := obj.(*schema.Event)
	if !ok {
		p.pools.Put("Event", obj)
		return nil, errors.New("event pool returned unexpected type")
	}
	evt.Reset()
	return evt, nil
}

func canonicalInstrument(symbol string) string {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return ""
	}
	knownQuotes := []string{"USDT", "BUSD", "USDC", "BTC"}
	for _, quote := range knownQuotes {
		if strings.HasSuffix(symbol, quote) && len(symbol) > len(quote) {
			base := symbol[:len(symbol)-len(quote)]
			return fmt.Sprintf("%s-%s", base, quote)
		}
	}
	if len(symbol) > 3 {
		return fmt.Sprintf("%s-%s", symbol[:3], symbol[3:])
	}
	return symbol
}

func toPriceLevels(levels [][]string) []schema.PriceLevel {
	out := make([]schema.PriceLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		out = append(out, schema.PriceLevel{Price: level[0], Quantity: level[1]})
	}
	return out
}

func buildEventID(provider, symbol string, typ schema.EventType, seq uint64) string {
	return fmt.Sprintf("%s:%s:%s:%d", provider, symbol, string(typ), seq)
}

func inferStreamType(stream string) string {
	stream = strings.ToLower(stream)
	switch {
	case strings.Contains(stream, "depth"):
		return "depthupdate"
	case strings.Contains(stream, "aggtrade"):
		return "aggtrade"
	case strings.Contains(stream, "ticker"):
		return "24hrticker"
	default:
		return ""
	}
}

type wsEnvelope struct {
	Stream string          `json:"stream"`
	Data   json.RawMessage `json:"data"`
}

type depthUpdate struct {
	EventType      string     `json:"e"`
	EventTime      int64      `json:"E"`
	Symbol         string     `json:"s"`
	FirstUpdateID  uint64     `json:"U"` // First update ID in event
	FinalUpdateID  uint64     `json:"u"` // Final update ID in event
	Bids           [][]string `json:"b"`
	Asks           [][]string `json:"a"`
	Checksum       string     `json:"checksum"`
}

type aggTrade struct {
	EventType    string `json:"e"`
	EventTime    int64  `json:"E"`
	Symbol       string `json:"s"`
	TradeID      uint64 `json:"t"`
	Price        string `json:"p"`
	Quantity     string `json:"q"`
	IsBuyerMaker bool   `json:"m"`
}

type ticker24hr struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	LastPrice string `json:"c"`
	BidPrice  string `json:"b"`
	AskPrice  string `json:"a"`
	Volume    string `json:"v"`
}

type orderbookSnapshot struct {
	Symbol       string     `json:"s"`
	LastUpdateID uint64     `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
	Checksum     string     `json:"checksum"`
	EventTime    int64      `json:"E"`
}

type klineEvent struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	Kline     struct {
		StartTime int64  `json:"t"`
		CloseTime int64  `json:"T"`
		Symbol    string `json:"s"`
		Interval  string `json:"i"`
		Open      string `json:"o"`
		Close     string `json:"c"`
		High      string `json:"h"`
		Low       string `json:"l"`
		Volume    string `json:"v"`
		IsClosed  bool   `json:"x"`
	} `json:"k"`
}

type executionReport struct {
	EventType           string `json:"e"`
	EventTime           int64  `json:"E"`
	Symbol              string `json:"s"`
	ClientOrderID       string `json:"c"`
	Side                string `json:"S"`
	OrderType           string `json:"o"`
	OrderStatus         string `json:"X"`
	OrderRejectReason   string `json:"r"`
	OrderID             uint64 `json:"i"`
	LastFilledQty       string `json:"l"`
	CumQty              string `json:"z"`
	LastFilledPrice     string `json:"L"`
	Commission          string `json:"n"`
	CommissionAsset     string `json:"N"`
	TransactionTime     int64  `json:"T"`
	TradeID             uint64 `json:"t"`
	OrigQty             string `json:"q"`
	Price               string `json:"p"`
	StopPrice           string `json:"P"`
	IcebergQty          string `json:"F"`
	TimeInForce         string `json:"f"`
	CurrentOrderStatus  string `json:"x"`
}
