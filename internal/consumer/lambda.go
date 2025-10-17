package consumer

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync/atomic"
	"time"

	json "github.com/goccy/go-json"

	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
	"github.com/sourcegraph/conc"
)

// LambdaConfig defines configuration for a lambda trading bot instance.
type LambdaConfig struct {
	Symbol   string
	Provider string
}

// OrderSubmitter defines the interface for submitting orders to a provider.
type OrderSubmitter interface {
	SubmitOrder(ctx context.Context, req schema.OrderRequest) error
}

// Lambda is a trading bot that subscribes to market data for a specific symbol
// and executes predefined trading logic.
type Lambda struct {
	id            string
	config        LambdaConfig
	bus           databus.Bus
	control       controlbus.Bus
	orderSubmitter OrderSubmitter
	pools         *pool.PoolManager
	logger        *log.Logger
	
	lastPrice     atomic.Value
	bidPrice      atomic.Value
	askPrice      atomic.Value
	tradingActive atomic.Bool
	orderCount    atomic.Int64
}

type marketState struct {
	lastPrice float64
	bidPrice  float64
	askPrice  float64
	timestamp time.Time
}

// NewLambda creates a new trading lambda for a specific symbol and provider.
func NewLambda(id string, config LambdaConfig, bus databus.Bus, control controlbus.Bus, orderSubmitter OrderSubmitter, pools *pool.PoolManager, logger *log.Logger) *Lambda {
	if id == "" {
		id = fmt.Sprintf("lambda-%s", config.Symbol)
	}
	
	lambda := &Lambda{
		id:             id,
		config:         config,
		bus:            bus,
		control:        control,
		orderSubmitter: orderSubmitter,
		pools:          pools,
		logger:         logger,
	}
	
	lambda.lastPrice.Store(float64(0))
	lambda.bidPrice.Store(float64(0))
	lambda.askPrice.Store(float64(0))
	lambda.tradingActive.Store(false)
	
	return lambda
}

// Start begins consuming market data and executing trading logic.
func (l *Lambda) Start(ctx context.Context) (<-chan error, error) {
	if l.bus == nil {
		return nil, fmt.Errorf("lambda %s: data bus required", l.id)
	}
	if l.control == nil {
		return nil, fmt.Errorf("lambda %s: control bus required", l.id)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	
	eventTypes := []schema.EventType{
		schema.EventTypeTrade,
		schema.EventTypeTicker,
		schema.EventTypeBookSnapshot,
		schema.EventTypeBookUpdate,
		schema.EventTypeExecReport,
	}
	
	errs := make(chan error, len(eventTypes))
	subs := make([]subscription, 0, len(eventTypes))
	
	for _, typ := range eventTypes {
		subID, ch, err := l.bus.Subscribe(ctx, typ)
		if err != nil {
			close(errs)
			for _, sub := range subs {
				l.bus.Unsubscribe(sub.id)
			}
			return nil, fmt.Errorf("subscribe to %s: %w", typ, err)
		}
		subs = append(subs, subscription{id: subID, typ: typ, ch: ch})
	}
	
	go l.consume(ctx, subs, errs)
	
	l.logger.Printf("[%s] started for symbol=%s provider=%s", l.id, l.config.Symbol, l.config.Provider)
	return errs, nil
}

type subscription struct {
	id  databus.SubscriptionID
	typ schema.EventType
	ch  <-chan *schema.Event
}

func (l *Lambda) consume(ctx context.Context, subs []subscription, errs chan<- error) {
	defer close(errs)
	
	var wg conc.WaitGroup
	for _, sub := range subs {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case evt, ok := <-sub.ch:
					if !ok {
						return
					}
					l.handleEvent(ctx, sub.typ, evt)
				}
			}
		})
	}
	
	wg.Wait()
	
	for _, sub := range subs {
		l.bus.Unsubscribe(sub.id)
	}
}

func (l *Lambda) handleEvent(ctx context.Context, typ schema.EventType, evt *schema.Event) {
	if evt == nil {
		return
	}
	
	defer l.recycleEvent(evt)
	
	if !l.matchesSymbol(evt) {
		return
	}
	
	if !l.matchesProvider(evt) {
		return
	}
	
	switch typ {
	case schema.EventTypeTrade:
		l.handleTrade(ctx, evt)
	case schema.EventTypeTicker:
		l.handleTicker(ctx, evt)
	case schema.EventTypeBookSnapshot:
		l.handleBookSnapshot(ctx, evt)
	case schema.EventTypeBookUpdate:
		l.handleBookUpdate(ctx, evt)
	case schema.EventTypeExecReport:
		l.handleExecReport(ctx, evt)
	}
}

func (l *Lambda) matchesSymbol(evt *schema.Event) bool {
	return evt.Symbol == l.config.Symbol
}

func (l *Lambda) matchesProvider(evt *schema.Event) bool {
	if l.config.Provider == "" {
		return true
	}
	return evt.Provider == l.config.Provider
}

func (l *Lambda) handleTrade(ctx context.Context, evt *schema.Event) {
	payload, ok := evt.Payload.(schema.TradePayload)
	if !ok {
		return
	}
	
	price, err := strconv.ParseFloat(payload.Price, 64)
	if err != nil {
		return
	}
	
	l.lastPrice.Store(price)
	
	l.logger.Printf("[%s] TRADE %s %s@%s side=%s", 
		l.id, evt.Symbol, payload.Quantity, payload.Price, payload.Side)
	
	l.evaluateTradingLogic(ctx, price)
}

func (l *Lambda) handleTicker(ctx context.Context, evt *schema.Event) {
	payload, ok := evt.Payload.(schema.TickerPayload)
	if !ok {
		return
	}
	
	lastPrice, _ := strconv.ParseFloat(payload.LastPrice, 64)
	bidPrice, _ := strconv.ParseFloat(payload.BidPrice, 64)
	askPrice, _ := strconv.ParseFloat(payload.AskPrice, 64)
	
	l.lastPrice.Store(lastPrice)
	l.bidPrice.Store(bidPrice)
	l.askPrice.Store(askPrice)
	
	l.logger.Printf("[%s] TICKER %s last=%s bid=%s ask=%s vol=%s", 
		l.id, evt.Symbol, payload.LastPrice, payload.BidPrice, payload.AskPrice, payload.Volume24h)
}

func (l *Lambda) handleBookSnapshot(ctx context.Context, evt *schema.Event) {
	payload, ok := evt.Payload.(schema.BookSnapshotPayload)
	if !ok {
		return
	}
	
	if len(payload.Bids) > 0 {
		bidPrice, _ := strconv.ParseFloat(payload.Bids[0].Price, 64)
		l.bidPrice.Store(bidPrice)
	}
	
	if len(payload.Asks) > 0 {
		askPrice, _ := strconv.ParseFloat(payload.Asks[0].Price, 64)
		l.askPrice.Store(askPrice)
	}
	
	l.logger.Printf("[%s] BOOK_SNAPSHOT %s bids=%d asks=%d", 
		l.id, evt.Symbol, len(payload.Bids), len(payload.Asks))
}

func (l *Lambda) handleBookUpdate(ctx context.Context, evt *schema.Event) {
	payload, ok := evt.Payload.(schema.BookUpdatePayload)
	if !ok {
		return
	}
	
	if len(payload.Bids) > 0 {
		bidPrice, _ := strconv.ParseFloat(payload.Bids[0].Price, 64)
		if bidPrice > 0 {
			l.bidPrice.Store(bidPrice)
		}
	}
	
	if len(payload.Asks) > 0 {
		askPrice, _ := strconv.ParseFloat(payload.Asks[0].Price, 64)
		if askPrice > 0 {
			l.askPrice.Store(askPrice)
		}
	}
	
	l.logger.Printf("[%s] BOOK_UPDATE %s bids=%d asks=%d", 
		l.id, evt.Symbol, len(payload.Bids), len(payload.Asks))
}

func (l *Lambda) handleExecReport(ctx context.Context, evt *schema.Event) {
	payload, ok := evt.Payload.(schema.ExecReportPayload)
	if !ok {
		return
	}
	
	// Only process ExecReports for orders submitted by this lambda
	if !l.isMyOrder(payload.ClientOrderID) {
		return
	}
	
	l.logger.Printf("[%s] EXEC_REPORT %s order_id=%s state=%s filled=%s avg_price=%s", 
		l.id, evt.Symbol, payload.ClientOrderID, payload.State, payload.FilledQuantity, payload.AvgFillPrice)
	
	switch payload.State {
	case schema.ExecReportStateFILLED:
		l.logger.Printf("[%s] ORDER FILLED order_id=%s qty=%s price=%s", 
			l.id, payload.ClientOrderID, payload.FilledQuantity, payload.AvgFillPrice)
	case schema.ExecReportStateREJECTED:
		reason := ""
		if payload.RejectReason != nil {
			reason = *payload.RejectReason
		}
		l.logger.Printf("[%s] ORDER REJECTED order_id=%s reason=%s", 
			l.id, payload.ClientOrderID, reason)
	}
}

func (l *Lambda) evaluateTradingLogic(ctx context.Context, currentPrice float64) {
	if !l.tradingActive.Load() {
		return
	}
	
	bidPrice := l.bidPrice.Load().(float64)
	askPrice := l.askPrice.Load().(float64)
	
	if bidPrice <= 0 || askPrice <= 0 {
		return
	}
	
	spread := askPrice - bidPrice
	spreadPct := (spread / bidPrice) * 100
	
	if spreadPct > 0.5 {
		l.logger.Printf("[%s] STRATEGY spread=%.2f%% (wide spread detected)", l.id, spreadPct)
	}
	
	orderNum := l.orderCount.Add(1)
	
	if orderNum%100 == 0 {
		l.logger.Printf("[%s] STRATEGY price=%.2f bid=%.2f ask=%.2f spread=%.2f%% (sample every 100 trades)", 
			l.id, currentPrice, bidPrice, askPrice, spreadPct)
	}
}

func (l *Lambda) SubmitOrder(ctx context.Context, side schema.TradeSide, quantity string, price *string) error {
	if l.orderSubmitter == nil {
		return fmt.Errorf("order submitter not configured")
	}
	if l.pools == nil {
		return fmt.Errorf("pool manager not configured")
	}
	
	orderID := fmt.Sprintf("%s-%d-%d", l.id, time.Now().UnixNano(), l.orderCount.Load())
	
	priceStr := ""
	if price != nil {
		priceStr = *price
	}
	
	orderReq, release, err := pool.AcquireOrderRequest(ctx, l.pools)
	if err != nil {
		return fmt.Errorf("acquire order request from pool: %w", err)
	}
	defer release()
	
	orderReq.ClientOrderID = orderID
	orderReq.ConsumerID = l.id
	orderReq.Provider = l.config.Provider
	orderReq.Symbol = l.config.Symbol
	orderReq.Side = side
	orderReq.OrderType = schema.OrderTypeLimit
	orderReq.Price = price
	orderReq.Quantity = quantity
	orderReq.TIF = "GTC"
	orderReq.Timestamp = time.Now().UTC()
	
	if err := l.orderSubmitter.SubmitOrder(ctx, *orderReq); err != nil {
		return fmt.Errorf("submit order: %w", err)
	}
	
	l.logger.Printf("[%s] ORDER SUBMITTED order_id=%s side=%s qty=%s price=%s", 
		l.id, orderID, side, quantity, priceStr)
	
	return nil
}

func (l *Lambda) sendControlMessage(ctx context.Context, typ schema.ControlMessageType, payload any) (schema.ControlAcknowledgement, error) {
	if l.control == nil {
		return schema.ControlAcknowledgement{}, fmt.Errorf("control bus unavailable")
	}
	
	body, err := marshalPayload(payload)
	if err != nil {
		return schema.ControlAcknowledgement{}, err
	}
	
	msg := schema.ControlMessage{
		MessageID:  fmt.Sprintf("%s-%d", l.id, time.Now().UTC().UnixNano()),
		ConsumerID: l.id,
		Type:       typ,
		Payload:    body,
		Timestamp:  time.Now().UTC(),
	}
	
	return l.control.Send(ctx, msg)
}

func (l *Lambda) EnableTrading(enabled bool) {
	l.tradingActive.Store(enabled)
	status := "DISABLED"
	if enabled {
		status = "ENABLED"
	}
	l.logger.Printf("[%s] Trading %s", l.id, status)
}

// isMyOrder checks if the ClientOrderID belongs to this lambda instance.
// Order IDs are formatted as: "{lambda-id}-{timestamp}-{count}"
func (l *Lambda) isMyOrder(clientOrderID string) bool {
	if clientOrderID == "" {
		return false
	}
	// Check if the order ID starts with this lambda's ID
	prefix := l.id + "-"
	return len(clientOrderID) > len(prefix) && clientOrderID[:len(prefix)] == prefix
}

func (l *Lambda) recycleEvent(evt *schema.Event) {
	if evt == nil {
		return
	}
	if l.pools != nil {
		l.pools.ReturnEventInst(evt)
	}
}

func marshalPayload(payload any) ([]byte, error) {
	if payload == nil {
		return []byte("{}"), nil
	}
	
	switch v := payload.(type) {
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		b, err := marshalJSON(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal payload: %w", err)
		}
		return b, nil
	}
}

func marshalJSON(v any) ([]byte, error) {
	return json.Marshal(v)
}
