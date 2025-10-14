package consumer

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/recycler"
	"github.com/coachpo/meltica/internal/schema"
)

// Lambda streams events from the data bus and applies a lambda handler before
// recycling them.
type Lambda struct {
	id        string
	bus       databus.Bus
	rec       recycler.Interface
	logger    *log.Logger
	formatter func(schema.EventType, *schema.Event) string

	version atomic.Int64
}

// NewLambda constructs a lambda consumer that prints received events.
func NewLambda(id string, bus databus.Bus, rec recycler.Interface, logger *log.Logger) *Lambda {
	if id == "" {
		id = "lambda-consumer"
	}
	return &Lambda{
		id:     id,
		bus:    bus,
		rec:    rec,
		logger: logger,
	}
}

// NewTradePrinter constructs a lambda that prints trade payloads.
func NewTradePrinter(bus databus.Bus, rec recycler.Interface, logger *log.Logger) *Lambda {
	l := NewLambda("trade-printer", bus, rec, logger)
	l.formatter = tradeFormatter
	return l
}

// NewTickerPrinter constructs a lambda that prints ticker payloads.
func NewTickerPrinter(bus databus.Bus, rec recycler.Interface, logger *log.Logger) *Lambda {
	l := NewLambda("ticker-printer", bus, rec, logger)
	l.formatter = tickerFormatter
	return l
}

// NewOrderbookPrinter constructs a lambda that prints orderbook payloads.
func NewOrderbookPrinter(bus databus.Bus, rec recycler.Interface, logger *log.Logger) *Lambda {
	l := NewLambda("orderbook-printer", bus, rec, logger)
	l.formatter = orderbookFormatter
	return l
}

// Start subscribes to the requested event types and prints them to stdout.
func (l *Lambda) Start(ctx context.Context, types []schema.EventType) (<-chan error, error) {
	if l.bus == nil {
		return nil, fmt.Errorf("lambda consumer %s: bus required", l.id)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	errs := make(chan error, len(types))
	subs := make([]subscription, 0, len(types))
	for _, typ := range types {
		id, ch, err := l.bus.Subscribe(ctx, typ)
		if err != nil {
			close(errs)
			for _, sub := range subs {
				l.bus.Unsubscribe(sub.id)
			}
			return nil, err
		}
		subs = append(subs, subscription{id: id, typ: typ, ch: ch})
	}
	go l.consume(ctx, subs, errs)
	return errs, nil
}

func (l *Lambda) consume(ctx context.Context, subs []subscription, errs chan<- error) {
	defer close(errs)
	var wg sync.WaitGroup
	wg.Add(len(subs))
	for _, sub := range subs {
		sub := sub
		go func() {
			defer wg.Done()
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
		}()
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
	if rv := int64(evt.RoutingVersion); rv > 0 {
		for {
			current := l.version.Load()
			if rv <= current {
				break
			}
			if l.version.CompareAndSwap(current, rv) {
				break
			}
		}
	}
	if l.shouldIgnore(typ, evt) {
		if l.rec != nil {
			l.rec.RecycleEvent(evt)
		}
		return
	}
	l.printEvent(typ, evt)
	if l.rec != nil {
		l.rec.RecycleEvent(evt)
	}
	_ = ctx
}

func (l *Lambda) printEvent(typ schema.EventType, evt *schema.Event) {
	if evt == nil {
		return
	}
	line := ""
	if l.formatter != nil {
		line = l.formatter(typ, evt)
	}
	if line == "" {
		line = defaultFormatter(l.id, typ, evt)
	}
	if l.logger != nil {
		l.logger.Println(line)
		return
	}
	fmt.Println(line)
}

func (l *Lambda) shouldIgnore(typ schema.EventType, evt *schema.Event) bool {
	if evt == nil {
		return true
	}
	if isCritical(typ) {
		return false
	}
	if !isMarketData(typ) {
		return false
	}
	active := l.version.Load()
	if active == 0 {
		return false
	}
	return int64(evt.RoutingVersion) < active
}

func defaultFormatter(id string, typ schema.EventType, evt *schema.Event) string {
	if evt == nil {
		return ""
	}
	ts := evt.EmitTS
	if ts.IsZero() {
		ts = time.Now().UTC()
	}
	payload := fmt.Sprintf("%v", evt.Payload)
	return fmt.Sprintf(
		"[consumer:%s] type=%s provider=%s symbol=%s seq=%d route=%d trace=%s payload=%s",
		id,
		typ,
		evt.Provider,
		evt.Symbol,
		evt.SeqProvider,
		evt.RoutingVersion,
		evt.TraceID,
		payload,
	)
}

func tradeFormatter(_ schema.EventType, evt *schema.Event) string {
	payload, ok := asTradePayload(evt)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"[trade] %s %s qty=%s price=%s ts=%s rv=%d",
		evt.Symbol,
		payload.Side,
		payload.Quantity,
		payload.Price,
		payload.Timestamp.UTC().Format(time.RFC3339Nano),
		evt.RoutingVersion,
	)
}

func tickerFormatter(_ schema.EventType, evt *schema.Event) string {
	payload, ok := asTickerPayload(evt)
	if !ok {
		return ""
	}
	return fmt.Sprintf(
		"[ticker] %s last=%s bid=%s ask=%s vol24h=%s ts=%s rv=%d",
		evt.Symbol,
		payload.LastPrice,
		payload.BidPrice,
		payload.AskPrice,
		payload.Volume24h,
		payload.Timestamp.UTC().Format(time.RFC3339Nano),
		evt.RoutingVersion,
	)
}

func orderbookFormatter(typ schema.EventType, evt *schema.Event) string {
	switch typ {
	case schema.EventTypeBookSnapshot:
		snap, ok := asBookSnapshotPayload(evt)
		if !ok {
			return ""
		}
		return fmt.Sprintf(
			"[orderbook:snapshot] %s bids=%d asks=%d checksum=%s ts=%s rv=%d",
			evt.Symbol,
			len(snap.Bids),
			len(snap.Asks),
			snap.Checksum,
			snap.LastUpdate.UTC().Format(time.RFC3339Nano),
			evt.RoutingVersion,
		)
	case schema.EventTypeBookUpdate:
		update, ok := asBookUpdatePayload(evt)
		if !ok {
			return ""
		}
		return fmt.Sprintf(
			"[orderbook:update] %s bids=%d asks=%d checksum=%s rv=%d",
			evt.Symbol,
			len(update.Bids),
			len(update.Asks),
			update.Checksum,
			evt.RoutingVersion,
		)
	default:
		return ""
	}
}

func isMarketData(typ schema.EventType) bool {
	switch typ {
	case schema.EventTypeBookSnapshot,
		schema.EventTypeBookUpdate,
		schema.EventTypeTicker,
		schema.EventTypeKlineSummary,
		schema.EventTypeTrade:
		return true
	default:
		return false
	}
}

func isCritical(typ schema.EventType) bool {
	return typ == schema.EventTypeExecReport
}

func asTradePayload(evt *schema.Event) (schema.TradePayload, bool) {
	if evt == nil {
		return schema.TradePayload{}, false
	}
	switch payload := evt.Payload.(type) {
	case schema.TradePayload:
		return payload, true
	case *schema.TradePayload:
		if payload == nil {
			return schema.TradePayload{}, false
		}
		return *payload, true
	default:
		return schema.TradePayload{}, false
	}
}

func asTickerPayload(evt *schema.Event) (schema.TickerPayload, bool) {
	if evt == nil {
		return schema.TickerPayload{}, false
	}
	switch payload := evt.Payload.(type) {
	case schema.TickerPayload:
		return payload, true
	case *schema.TickerPayload:
		if payload == nil {
			return schema.TickerPayload{}, false
		}
		return *payload, true
	default:
		return schema.TickerPayload{}, false
	}
}

func asBookSnapshotPayload(evt *schema.Event) (schema.BookSnapshotPayload, bool) {
	if evt == nil {
		return schema.BookSnapshotPayload{}, false
	}
	switch payload := evt.Payload.(type) {
	case schema.BookSnapshotPayload:
		return payload, true
	case *schema.BookSnapshotPayload:
		if payload == nil {
			return schema.BookSnapshotPayload{}, false
		}
		return *payload, true
	default:
		return schema.BookSnapshotPayload{}, false
	}
}

func asBookUpdatePayload(evt *schema.Event) (schema.BookUpdatePayload, bool) {
	if evt == nil {
		return schema.BookUpdatePayload{}, false
	}
	switch payload := evt.Payload.(type) {
	case schema.BookUpdatePayload:
		return payload, true
	case *schema.BookUpdatePayload:
		if payload == nil {
			return schema.BookUpdatePayload{}, false
		}
		return *payload, true
	default:
		return schema.BookUpdatePayload{}, false
	}
}
