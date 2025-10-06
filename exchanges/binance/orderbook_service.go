package binance

import (
	"context"
	"log"
	"math/big"
	"sync"
	"time"

	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
)

// OrderBookService is a Level-3 service that manages order book state
// by consuming raw DepthDelta events from Level 2 routers.
type OrderBookService struct {
	router  wsRouter
	depths  *depthSnapshotService
	mu      sync.RWMutex
	books   map[string]*OrderBook
	symbols *symbolService
}

func newOrderBookService(router wsRouter, depths *depthSnapshotService, symbols *symbolService) *OrderBookService {
	return &OrderBookService{
		router:  router,
		depths:  depths,
		books:   make(map[string]*OrderBook),
		symbols: symbols,
	}
}

// Subscribe subscribes to order book updates for a symbol and returns a channel of BookEvents.
func (s *OrderBookService) Subscribe(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	// Subscribe to raw depth delta events
	sub, err := s.router.SubscribePublic(ctx, coretopics.Book(symbol))
	if err != nil {
		return nil, nil, internal.WrapExchange(err, "subscribe depth stream")
	}

	// Get or create order book
	s.mu.Lock()
	book, exists := s.books[symbol]
	if !exists {
		book = &OrderBook{
			Symbol: symbol,
			Bids:   make(map[string]*big.Rat),
			Asks:   make(map[string]*big.Rat),
		}
		s.books[symbol] = book
	}
	s.mu.Unlock()

	events := make(chan corestreams.BookEvent, 32)
	errs := make(chan error, 1)

	go s.processDepthDeltas(ctx, sub, book, events, errs, symbol)

	return events, errs, nil
}

// processDepthDeltas processes incoming DepthDelta events and maintains order book state.
func (s *OrderBookService) processDepthDeltas(
	ctx context.Context,
	sub bnrouting.Subscription,
	book *OrderBook,
	events chan<- corestreams.BookEvent,
	errs chan<- error,
	symbol string,
) {
	defer close(events)
	defer close(errs)
	defer sub.Close()

	// Buffer for events received before initialization
	var buffer []*bnrouting.DepthDelta

	// Initialize order book with snapshot
	if err := s.initializeOrderBook(ctx, book, symbol); err != nil {
		select {
		case errs <- err:
		default:
		}
		return
	}

	// Replay buffered events
	if len(buffer) > 0 {
		for _, delta := range buffer {
			if delta.LastUpdateID <= book.LastUpdateID() {
				continue // Skip old events
			}
			s.applyDelta(book, delta, errs)
			// Send updated snapshot
			s.emitSnapshot(ctx, book, events)
		}
		buffer = nil
	}

	// Periodic snapshots heartbeat
	// Process incoming events
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// Periodically send snapshots
			if book.Initialized() {
				s.emitSnapshot(ctx, book, events)
			}

		case msg, ok := <-sub.C():
			if !ok {
				return
			}

			// Parse DepthDelta
			delta, ok := msg.Parsed.(*bnrouting.DepthDelta)
			if !ok {
				continue
			}

			if !book.Initialized() {
				// Buffer events until initialized
				buffer = append(buffer, delta)
				if len(buffer) > 1000 {
					// Too many buffered events, reinitialize
					if err := s.initializeOrderBook(ctx, book, symbol); err != nil {
						select {
						case errs <- err:
						default:
						}
						return
					}
					buffer = nil
				}
				continue
			}

			s.applyDelta(book, delta, errs)
			// Send updated snapshot
			s.emitSnapshot(ctx, book, events)

		case err, ok := <-sub.Err():
			if !ok {
				return
			}
			if err != nil {
				select {
				case errs <- err:
				default:
				}
				return
			}
		}
	}
}

// emitSnapshot obtains the current snapshot and performs a non-blocking send.
// Returns true if the snapshot was successfully delivered, false otherwise.
func (s *OrderBookService) emitSnapshot(ctx context.Context, book *OrderBook, events chan<- corestreams.BookEvent) bool {
	snapshot := book.GetSnapshot()
	select {
	case events <- snapshot:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

// applyDelta applies a DepthDelta to the order book.
func (s *OrderBookService) applyDelta(
	book *OrderBook,
	delta *bnrouting.DepthDelta,
	errs chan<- error,
) {
	success := book.UpdateFromDelta(delta.Bids, delta.Asks, delta.FirstUpdateID, delta.LastUpdateID, delta.EventTime)
	if !success {
		// Gap detected, need to reinitialize
		log.Printf("Order book gap detected for %s, reinitializing", delta.Symbol)
		initCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.initializeOrderBook(initCtx, book, delta.Symbol); err != nil {
			select {
			case errs <- err:
			default:
			}
		}
		return
	}
}

// initializeOrderBook initializes an order book with a snapshot from the REST API.
func (s *OrderBookService) initializeOrderBook(ctx context.Context, book *OrderBook, symbol string) error {
	const maxRetries = 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "order book initialization cancelled")
		default:
		}

		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		snapshot, lastUpdateID, err := s.depths.Snapshot(initCtx, symbol, 5000)
		cancel()

		if err == nil {
			book.InitializeFromSnapshot(snapshot, lastUpdateID)
			log.Printf("Initialized order book for %s (update ID: %d)", symbol, lastUpdateID)
			return nil
		}

		lastErr = err
		backoff := time.Duration(attempt+1) * time.Second
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "order book initialization cancelled")
		case <-time.After(backoff):
		}
	}

	if lastErr == nil {
		lastErr = internal.Exchange("failed to initialize order book")
	}
	return lastErr
}

// Snapshot returns the current snapshot for a symbol if initialized.
func (s *OrderBookService) Snapshot(symbol string) (corestreams.BookEvent, bool) {
	s.mu.RLock()
	book, exists := s.books[symbol]
	s.mu.RUnlock()

	if !exists || !book.Initialized() {
		return corestreams.BookEvent{}, false
	}

	return book.GetSnapshot(), true
}
