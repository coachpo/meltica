package consumer

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/recycler"
	"github.com/coachpo/meltica/internal/schema"
)

// TradeLambda specializes in processing trade events.
type TradeLambda struct {
	id     string
	bus    databus.Bus
	rec    recycler.Interface
	logger *log.Logger
}

// NewTradeLambda creates a lambda that processes only trade events.
func NewTradeLambda(id string, bus databus.Bus, rec recycler.Interface, logger *log.Logger) *TradeLambda {
	if id == "" {
		id = "trade-lambda"
	}
	return &TradeLambda{
		id:     id,
		bus:    bus,
		rec:    rec,
		logger: logger,
	}
}

// Start begins consuming trade events from the data bus.
func (l *TradeLambda) Start(ctx context.Context) (<-chan error, error) {
	if l.bus == nil {
		return nil, fmt.Errorf("trade lambda %s: bus required", l.id)
	}
	if ctx == nil {
		ctx = context.Background()
	}

	errs := make(chan error, 1)
	id, ch, err := l.bus.Subscribe(ctx, schema.EventTypeTrade)
	if err != nil {
		close(errs)
		return nil, err
	}

	go l.consume(ctx, databus.SubscriptionID(id), ch, errs)
	return errs, nil
}

func (l *TradeLambda) consume(ctx context.Context, subscriptionID databus.SubscriptionID, events <-chan *schema.Event, errs chan<- error) {
	defer close(errs)
	defer l.bus.Unsubscribe(subscriptionID)

	for {
		select {
		case <-ctx.Done():
			return
		case evt, ok := <-events:
			if !ok {
				return
			}
			l.handleTradeEvent(evt)
		}
	}
}

func (l *TradeLambda) handleTradeEvent(evt *schema.Event) {
	if evt == nil || evt.Type != schema.EventTypeTrade {
		if l.rec != nil && evt != nil {
			l.rec.RecycleEvent(evt)
		}
		return
	}

	payload, ok := evt.Payload.(schema.TradePayload)
	if !ok {
		if l.rec != nil {
			l.rec.RecycleEvent(evt)
		}
		return
	}

	l.printTrade(evt, payload)

	if l.rec != nil {
		l.rec.RecycleEvent(evt)
	}
}

func (l *TradeLambda) printTrade(evt *schema.Event, payload schema.TradePayload) {
	ts := evt.EmitTS
	if ts.IsZero() {
		ts = payload.Timestamp
	}
	if ts.IsZero() {
		ts = time.Now().UTC()
	}

	sideStr := "BUY"
	if payload.Side == schema.TradeSideSell {
		sideStr = "SELL"
	}

	line := fmt.Sprintf(
		"[TRADE:%s] %s %s %s@%s ID:%s Time:%s",
		l.id,
		evt.Symbol,
		sideStr,
		payload.Quantity,
		payload.Price,
		payload.TradeID,
		ts.Format("15:04:05.000"),
	)

	if l.logger != nil {
		l.logger.Println(line)
	} else {
		fmt.Println(line)
	}
}
