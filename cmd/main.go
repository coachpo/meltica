package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"time"

	"github.com/coachpo/meltica/core"
	coreexchange "github.com/coachpo/meltica/core/exchange"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance"
)

const (
	nativeSymbol    = "BTCUSDT"
	canonicalSymbol = "BTC-USDT"
)

type marketEventKind string

const (
	marketEventTrade  marketEventKind = "trade"
	marketEventTicker marketEventKind = "ticker"
	marketEventBook   marketEventKind = "book"
	marketEventError  marketEventKind = "error"
	marketEventSystem marketEventKind = "system"
)

type marketEvent struct {
	Kind    marketEventKind
	Topic   string
	Payload interface{}
	Err     error
	Time    time.Time
	Message string
}

type marketCommand interface{}

type marketSubscribeCommand struct {
	Topics     []string
	BookSymbol string
}

type marketUnsubscribeCommand struct{}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")
	fmt.Printf("Monitoring %s depth via exchange pipelines...\n", nativeSymbol)

	exchange, err := binance.New("", "")
	if err != nil {
		log.Fatalf("failed to create Binance exchange: %v", err)
	}
	defer exchange.Close()

	eventLoop := newMarketEventLoop(exchange)
	go eventLoop.Run(ctx)

	command := marketSubscribeCommand{
		Topics: []string{
			coretopics.Trade(canonicalSymbol),
			coretopics.Ticker(canonicalSymbol),
		},
		BookSymbol: canonicalSymbol,
	}
	eventLoop.Publish(command)

	var lastBookLog time.Time

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping order book monitor...")
			return
		case evt, ok := <-eventLoop.Events():
			if !ok {
				fmt.Println("Event loop terminated")
				return
			}
			handleEvent(evt, &lastBookLog)
		}
	}
}

func handleEvent(evt marketEvent, lastBookLog *time.Time) {
	switch evt.Kind {
	case marketEventTrade:
		trade, ok := evt.Payload.(*coreexchange.TradeEvent)
		if !ok || trade == nil {
			log.Printf("trade event payload mismatch: %#v", evt.Payload)
			return
		}
		log.Printf("[WS] trade topic=%s price=%s qty=%s at=%s",
			evt.Topic,
			formatRat(trade.Price, 2),
			formatRat(trade.Quantity, 6),
			trade.Time.Format(time.RFC3339Nano),
		)
	case marketEventTicker:
		ticker, ok := evt.Payload.(*coreexchange.TickerEvent)
		if !ok || ticker == nil {
			log.Printf("ticker event payload mismatch: %#v", evt.Payload)
			return
		}
		log.Printf("[WS] ticker topic=%s bid=%s ask=%s at=%s",
			evt.Topic,
			formatRat(ticker.Bid, 2),
			formatRat(ticker.Ask, 2),
			ticker.Time.Format(time.RFC3339Nano),
		)
	case marketEventBook:
		book, ok := evt.Payload.(*coreexchange.BookEvent)
		if !ok || book == nil {
			log.Printf("book event payload mismatch: %#v", evt.Payload)
			return
		}
		if time.Since(*lastBookLog) < 200*time.Millisecond {
			return
		}
		*lastBookLog = time.Now()
		topBid := bestLevel(book.Bids, true)
		topAsk := bestLevel(book.Asks, false)
		log.Printf("[WS] book topic=%s bids=%d asks=%d topBidQty=%s topBidPx=%s topAskPx=%s topAskQty=%s at=%s",
			evt.Topic,
			len(book.Bids),
			len(book.Asks),
			formatRat(topBid.qty, 4),
			formatRat(topBid.price, 2),
			formatRat(topAsk.price, 2),
			formatRat(topAsk.qty, 4),
			book.Time.Format(time.RFC3339Nano),
		)
	case marketEventError:
		if evt.Err != nil {
			log.Printf("event loop error: %v", evt.Err)
		} else {
			log.Printf("event loop error: %s", evt.Message)
		}
	case marketEventSystem:
		if evt.Message != "" {
			log.Printf("event loop: %s", evt.Message)
		}
	default:
		log.Printf("unhandled event kind=%s topic=%s", evt.Kind, evt.Topic)
	}
}

type bookLevel struct {
	price *big.Rat
	qty   *big.Rat
}

func topLevels(levels []core.BookDepthLevel, limit int, desc bool) []bookLevel {
	filtered := make([]bookLevel, 0, len(levels))
	for _, level := range levels {
		if level.Price == nil || level.Qty == nil || level.Qty.Sign() == 0 {
			continue
		}
		filtered = append(filtered, bookLevel{
			price: new(big.Rat).Set(level.Price),
			qty:   new(big.Rat).Set(level.Qty),
		})
	}
	sort.Slice(filtered, func(i, j int) bool {
		cmp := filtered[i].price.Cmp(filtered[j].price)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func formatRat(r *big.Rat, precision int) string {
	if r == nil {
		return "0"
	}
	return r.FloatString(precision)
}

func bestLevel(levels []core.BookDepthLevel, desc bool) bookLevel {
	filtered := topLevels(levels, 1, desc)
	if len(filtered) == 0 {
		return bookLevel{}
	}
	return filtered[0]
}

type loopState struct {
	sub        core.Subscription
	wsMsgCh    <-chan core.Message
	wsErrCh    <-chan error
	bookCh     <-chan coreexchange.BookEvent
	bookErrCh  <-chan error
	bookCancel context.CancelFunc
}

func (s *loopState) shutdown() {
	if s.sub != nil {
		_ = s.sub.Close()
		s.sub = nil
	}
	if s.bookCancel != nil {
		s.bookCancel()
		s.bookCancel = nil
	}
	s.wsMsgCh = nil
	s.wsErrCh = nil
	s.bookCh = nil
	s.bookErrCh = nil
}

type marketEventLoop struct {
	exchange *binance.Exchange
	commands chan marketCommand
	events   chan marketEvent
}

func newMarketEventLoop(exchange *binance.Exchange) *marketEventLoop {
	return &marketEventLoop{
		exchange: exchange,
		commands: make(chan marketCommand, 16),
		events:   make(chan marketEvent, 256),
	}
}

func (l *marketEventLoop) Publish(cmd marketCommand) {
	select {
	case l.commands <- cmd:
	default:
		go func() { l.commands <- cmd }()
	}
}

func (l *marketEventLoop) Events() <-chan marketEvent {
	return l.events
}

func (l *marketEventLoop) Run(ctx context.Context) {
	defer close(l.events)

	state := loopState{}

	for {
		select {
		case <-ctx.Done():
			state.shutdown()
			return
		case cmd := <-l.commands:
			state = l.handleCommand(ctx, cmd, state)
		case msg, ok := <-state.wsMsgCh:
			if !ok {
				state.wsMsgCh = nil
				continue
			}
			l.handleWSMessage(ctx, msg)
		case err, ok := <-state.wsErrCh:
			if !ok {
				state.wsErrCh = nil
				continue
			}
			if err != nil {
				l.emit(ctx, marketEvent{Kind: marketEventError, Message: "websocket subscription error", Err: err})
			}
			if state.sub != nil {
				_ = state.sub.Close()
				state.sub = nil
			}
			state.wsMsgCh = nil
			state.wsErrCh = nil
		case book, ok := <-state.bookCh:
			if !ok {
				state.bookCh = nil
				continue
			}
			bookCopy := book
			l.emit(ctx, marketEvent{
				Kind:    marketEventBook,
				Topic:   coretopics.Book(book.Symbol),
				Payload: &bookCopy,
				Time:    book.Time,
			})
		case err, ok := <-state.bookErrCh:
			if !ok {
				state.bookErrCh = nil
				continue
			}
			if err != nil {
				l.emit(ctx, marketEvent{Kind: marketEventError, Message: "order book snapshot error", Err: err})
			}
		}
	}
}

func (l *marketEventLoop) handleCommand(ctx context.Context, cmd marketCommand, state loopState) loopState {
	switch c := cmd.(type) {
	case marketSubscribeCommand:
		state.shutdown()
		state = loopState{}
		if len(c.Topics) > 0 {
			sub, err := l.exchange.WS().SubscribePublic(ctx, c.Topics...)
			if err != nil {
				l.emit(ctx, marketEvent{Kind: marketEventError, Message: "subscribe public", Err: err})
				return state
			}
			state.sub = sub
			state.wsMsgCh = sub.C()
			state.wsErrCh = sub.Err()
		}
		if c.BookSymbol != "" {
			bookCtx, cancel := context.WithCancel(ctx)
			snapshots, errs, err := l.exchange.OrderBookSnapshots(bookCtx, c.BookSymbol)
			if err != nil {
				cancel()
				l.emit(ctx, marketEvent{Kind: marketEventError, Message: "order book initialization", Err: err})
			} else {
				state.bookCancel = cancel
				state.bookCh = snapshots
				state.bookErrCh = errs
			}
		}
		l.emit(ctx, marketEvent{
			Kind:    marketEventSystem,
			Message: fmt.Sprintf("subscribed topics=%v book=%s", c.Topics, c.BookSymbol),
		})
		return state
	case marketUnsubscribeCommand:
		state.shutdown()
		l.emit(ctx, marketEvent{Kind: marketEventSystem, Message: "unsubscribed"})
		return loopState{}
	default:
		l.emit(ctx, marketEvent{Kind: marketEventSystem, Message: fmt.Sprintf("unknown command %T", cmd)})
		return state
	}
}

func (l *marketEventLoop) handleWSMessage(ctx context.Context, msg core.Message) {
	switch evt := msg.Parsed.(type) {
	case *coreexchange.TradeEvent:
		l.emit(ctx, marketEvent{Kind: marketEventTrade, Topic: msg.Topic, Payload: evt, Time: evt.Time})
	case *coreexchange.TickerEvent:
		l.emit(ctx, marketEvent{Kind: marketEventTicker, Topic: msg.Topic, Payload: evt, Time: evt.Time})
	case *coreexchange.BookEvent:
		l.emit(ctx, marketEvent{Kind: marketEventBook, Topic: msg.Topic, Payload: evt, Time: evt.Time})
	default:
		l.emit(ctx, marketEvent{Kind: marketEventSystem, Topic: msg.Topic, Message: fmt.Sprintf("unhandled message route=%s", msg.Event), Time: msg.At})
	}
}

func (l *marketEventLoop) emit(ctx context.Context, evt marketEvent) {
	select {
	case <-ctx.Done():
		return
	case l.events <- evt:
	}
}
