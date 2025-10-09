package binance

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
	"github.com/coachpo/meltica/internal/numeric"
)

// BookService is a Level-3 service that manages book state
// by consuming raw DepthDelta events from Level 2 routers and provides
// REST snapshot functionality.
type BookService struct {
	router     wsRouter
	restRouter routingrest.RESTDispatcher
	mu         sync.RWMutex
	books      map[string]*Book
	symbols    *symbolService
}

func newBookService(router wsRouter, restRouter routingrest.RESTDispatcher, symbols *symbolService) *BookService {
	return &BookService{
		router:     router,
		restRouter: restRouter,
		books:      make(map[string]*Book),
		symbols:    symbols,
	}
}

// Subscribe subscribes to book updates for a symbol and returns a channel of BookEvents.
func (s *BookService) Subscribe(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	// Subscribe to raw depth delta events
	sub, err := s.router.SubscribePublic(ctx, bnrouting.OrderBook(symbol))
	if err != nil {
		return nil, nil, internal.WrapExchange(err, "subscribe depth stream")
	}

	// Get or create book
	s.mu.Lock()
	book, exists := s.books[symbol]
	if !exists {
		book = &Book{
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

// processDepthDeltas processes incoming DepthDelta events and maintains book state.
func (s *BookService) processDepthDeltas(
	ctx context.Context,
	sub bnrouting.Subscription,
	book *Book,
	events chan<- corestreams.BookEvent,
	errs chan<- error,
	symbol string,
) {
	defer close(events)
	defer close(errs)
	defer sub.Close()

	// Buffer for events received before initialization
	var buffer []*bnrouting.DepthDelta

	// Initialize book with snapshot
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
func (s *BookService) emitSnapshot(ctx context.Context, book *Book, events chan<- corestreams.BookEvent) bool {
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

// applyDelta applies a DepthDelta to the book.
func (s *BookService) applyDelta(
	book *Book,
	delta *bnrouting.DepthDelta,
	errs chan<- error,
) {
	success := book.UpdateFromDelta(delta.Bids, delta.Asks, delta.FirstUpdateID, delta.LastUpdateID, delta.EventTime, delta.VenueSymbol)
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

// initializeOrderBook initializes an book with a snapshot from the REST API.
func (s *BookService) initializeOrderBook(ctx context.Context, book *Book, symbol string) error {
	const maxRetries = 5
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "book initialization cancelled")
		default:
		}

		initCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		snapshot, lastUpdateID, err := s.Snapshot(initCtx, symbol, 5000)
		cancel()

		if err == nil {
			book.InitializeFromSnapshot(snapshot, lastUpdateID)
			log.Printf("Initialized book for %s (update ID: %d)", symbol, lastUpdateID)
			return nil
		}

		lastErr = err
		backoff := time.Duration(attempt+1) * time.Second
		select {
		case <-ctx.Done():
			return internal.WrapNetwork(ctx.Err(), "book initialization cancelled")
		case <-time.After(backoff):
		}
	}

	if lastErr == nil {
		lastErr = internal.Exchange("failed to initialize book")
	}
	return lastErr
}

// Snapshot fetches a REST depth snapshot for the given symbol.
func (s *BookService) Snapshot(ctx context.Context, symbol string, limit int) (corestreams.BookEvent, int64, error) {
	if s.restRouter == nil {
		return corestreams.BookEvent{}, 0, internal.Invalid("depth snapshot: rest router unavailable")
	}
	native, err := s.symbols.nativeForMarkets(ctx, symbol, core.MarketSpot)
	if err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	params := map[string]string{"symbol": native, "limit": fmt.Sprintf("%d", limit)}
	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/depth", Query: params}
	if err := s.restRouter.Dispatch(ctx, msg, &resp); err != nil {
		return corestreams.BookEvent{}, 0, err
	}
	bids := convertDepthLevels(resp.Bids)
	asks := convertDepthLevels(resp.Asks)
	event := corestreams.BookEvent{Symbol: symbol, VenueSymbol: native, Bids: bids, Asks: asks, Time: time.Now()}
	return event, resp.LastUpdateID, nil
}

func convertDepthLevels(pairs [][]interface{}) []core.BookDepthLevel {
	levels := make([]core.BookDepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var priceStr, qtyStr string
		switch v := pair[0].(type) {
		case string:
			priceStr = v
		default:
			priceStr = fmt.Sprint(v)
		}
		switch v := pair[1].(type) {
		case string:
			qtyStr = v
		default:
			qtyStr = fmt.Sprint(v)
		}
		price, _ := numeric.Parse(priceStr)
		qty, _ := numeric.Parse(qtyStr)
		levels = append(levels, core.BookDepthLevel{Price: price, Qty: qty})
	}
	return levels
}
