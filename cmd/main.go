package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	registry "github.com/coachpo/meltica/core/registry"
	registrybinance "github.com/coachpo/meltica/core/registry/binance"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
)

const (
	defaultBase  = "BTC"
	defaultQuote = "USDT"
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

// MarketManager manages concurrent market data streams
type MarketManager struct {
	exchange   core.Exchange
	orderBooks orderBookProvider
	events     chan marketEvent
	wg         sync.WaitGroup
	mu         sync.RWMutex
	cancel     context.CancelFunc

	// Active subscriptions
	sub        core.Subscription
	bookCancel context.CancelFunc
}

type orderBookProvider interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

func NewMarketManager(exchange core.Exchange) *MarketManager {
	orderBook, _ := exchange.(orderBookProvider)
	return &MarketManager{
		exchange:   exchange,
		orderBooks: orderBook,
		events:     make(chan marketEvent, 256),
	}
}

func (m *MarketManager) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	// Start the event router
	m.wg.Add(1)
	go m.eventRouter(ctx)
}

func (m *MarketManager) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	close(m.events)
}

func (m *MarketManager) Events() <-chan marketEvent {
	return m.events
}

func (m *MarketManager) Subscribe(cmd marketSubscribeCommand) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Clean up existing subscriptions
	m.cleanupSubscriptions()

	// Start new subscriptions
	if len(cmd.Topics) > 0 {
		m.startWebSocketSubscription(cmd.Topics)
	}
	if cmd.BookSymbol != "" {
		m.startOrderBookSubscription(cmd.BookSymbol)
	}

	m.emit(marketEvent{
		Kind:    marketEventSystem,
		Message: fmt.Sprintf("subscribed topics=%v book=%s", cmd.Topics, cmd.BookSymbol),
	})
}

func (m *MarketManager) Unsubscribe() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleanupSubscriptions()
	m.emit(marketEvent{
		Kind:    marketEventSystem,
		Message: "unsubscribed",
	})
}

func (m *MarketManager) cleanupSubscriptions() {
	if m.sub != nil {
		_ = m.sub.Close()
		m.sub = nil
	}
	if m.bookCancel != nil {
		m.bookCancel()
		m.bookCancel = nil
	}
}

func (m *MarketManager) startWebSocketSubscription(topics []string) {
	ctx := context.Background() // Will be managed by the manager's context

	sub, err := m.exchange.WS().SubscribePublic(ctx, topics...)
	if err != nil {
		m.emit(marketEvent{
			Kind:    marketEventError,
			Message: "subscribe public",
			Err:     err,
		})
		return
	}

	m.sub = sub

	// Start goroutine to handle websocket messages
	m.wg.Add(1)
	go m.handleWebSocketMessages(sub.C(), sub.Err())
}

func (m *MarketManager) startOrderBookSubscription(symbol string) {
	if m.orderBooks == nil {
		m.emit(marketEvent{
			Kind:    marketEventError,
			Message: "order book snapshots not supported",
		})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.bookCancel = cancel

	snapshots, errs, err := m.orderBooks.OrderBookSnapshots(ctx, symbol)
	if err != nil {
		cancel()
		m.emit(marketEvent{
			Kind:    marketEventError,
			Message: "order book initialization",
			Err:     err,
		})
		return
	}

	// Start goroutine to handle order book snapshots
	m.wg.Add(1)
	go m.handleOrderBookSnapshots(snapshots, errs)
}

func (m *MarketManager) handleWebSocketMessages(msgCh <-chan core.Message, errCh <-chan error) {
	defer m.wg.Done()

	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			m.handleWSMessage(msg)
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				m.emit(marketEvent{
					Kind:    marketEventError,
					Message: "websocket subscription error",
					Err:     err,
				})
			}
		}
	}
}

func (m *MarketManager) handleOrderBookSnapshots(snapshots <-chan corestreams.BookEvent, errs <-chan error) {
	defer m.wg.Done()

	for {
		select {
		case book, ok := <-snapshots:
			if !ok {
				return
			}
			bookCopy := book
			m.emit(marketEvent{
				Kind:    marketEventBook,
				Topic:   coretopics.Book(book.Symbol),
				Payload: &bookCopy,
				Time:    book.Time,
			})
		case err, ok := <-errs:
			if !ok {
				return
			}
			if err != nil {
				m.emit(marketEvent{
					Kind:    marketEventError,
					Message: "order book snapshot error",
					Err:     err,
				})
			}
		}
	}
}

func (m *MarketManager) handleWSMessage(msg core.Message) {
	switch evt := msg.Parsed.(type) {
	case *corestreams.TradeEvent:
		m.emit(marketEvent{
			Kind:    marketEventTrade,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	case *corestreams.TickerEvent:
		m.emit(marketEvent{
			Kind:    marketEventTicker,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	case *corestreams.BookEvent:
		m.emit(marketEvent{
			Kind:    marketEventBook,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	default:
		m.emit(marketEvent{
			Kind:    marketEventSystem,
			Topic:   msg.Topic,
			Message: fmt.Sprintf("unhandled message route=%s", msg.Event),
			Time:    msg.At,
		})
	}
}

func (m *MarketManager) eventRouter(ctx context.Context) {
	defer m.wg.Done()

	// This goroutine just ensures we properly close the events channel
	// when the context is cancelled
	<-ctx.Done()
}

func (m *MarketManager) emit(evt marketEvent) {
	select {
	case m.events <- evt:
	default:
		// Drop event if channel is full to prevent blocking
		log.Printf("event channel full, dropping event: %v", evt.Kind)
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")

	exchange, err := registry.Resolve(registrybinance.Name)
	if err != nil {
		log.Fatalf("failed to create Binance exchange: %v", err)
	}
	defer exchange.Close()

	canonicalSymbol := core.CanonicalSymbol(defaultBase, defaultQuote)
	nativeSymbol, err := core.NativeSymbol(registrybinance.Name, canonicalSymbol)
	if err != nil {
		log.Printf("failed to resolve native symbol for %s: %v", canonicalSymbol, err)
		nativeSymbol = canonicalSymbol
	}

	fmt.Printf("Monitoring %s depth via exchange pipelines...\n", nativeSymbol)

	manager := NewMarketManager(exchange)
	manager.Start(ctx)
	defer manager.Stop()

	// Subscribe to market data
	manager.Subscribe(marketSubscribeCommand{
		Topics: []string{
			coretopics.Trade(canonicalSymbol),
			coretopics.Ticker(canonicalSymbol),
		},
		BookSymbol: canonicalSymbol,
	})

	var lastBookLog time.Time

	// Main event processing loop
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping order book monitor...")
			return
		case evt, ok := <-manager.Events():
			if !ok {
				fmt.Println("Event channel closed")
				return
			}
			handleEvent(evt, &lastBookLog)
		}
	}
}

func handleEvent(evt marketEvent, lastBookLog *time.Time) {
	switch evt.Kind {
	case marketEventTrade:
		trade, ok := evt.Payload.(*corestreams.TradeEvent)
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
		ticker, ok := evt.Payload.(*corestreams.TickerEvent)
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
		book, ok := evt.Payload.(*corestreams.BookEvent)
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
