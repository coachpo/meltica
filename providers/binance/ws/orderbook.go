package ws

import (
	"fmt"
	"math/big"
	"sync"
	"time"

	corews "github.com/coachpo/meltica/core/ws"
)

// OrderBookManager manages order book state for different symbols.
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

// NewOrderBookManager creates a new order book manager.
func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{books: make(map[string]*OrderBook)}
}

// GetOrCreateOrderBook gets an existing order book or creates a new one.
func (m *OrderBookManager) GetOrCreateOrderBook(symbol string) *OrderBook {
	m.mu.Lock()
	defer m.mu.Unlock()

	if book, exists := m.books[symbol]; exists {
		return book
	}

	book := &OrderBook{
		Symbol: symbol,
		Bids:   make(map[string]*big.Rat),
		Asks:   make(map[string]*big.Rat),
	}
	m.books[symbol] = book
	return book
}

// OrderBook represents the current state of the Binance order book for a symbol.
type OrderBook struct {
	Symbol       string
	Bids         map[string]*big.Rat
	Asks         map[string]*big.Rat
	LastUpdateID int64
	LastUpdate   time.Time
	mu           sync.RWMutex

	// Buffering state for initialization
	isInitialized  bool
	bufferedEvents []*BufferedDepthEvent
}

// BufferedDepthEvent represents a depth event that's buffered during initialization
type BufferedDepthEvent struct {
	FirstUpdateID int64
	LastUpdateID  int64
	Bids          []corews.DepthLevel
	Asks          []corews.DepthLevel
	EventTime     time.Time
}

// InitializeFromSnapshot initializes the order book from a snapshot and applies buffered events
func (ob *OrderBook) InitializeFromSnapshot(snapshot corews.BookEvent, snapshotUpdateID int64) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// Reset the order book
	ob.Bids = make(map[string]*big.Rat)
	ob.Asks = make(map[string]*big.Rat)

	// Apply snapshot
	for _, level := range snapshot.Bids {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			ob.Bids[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	for _, level := range snapshot.Asks {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			ob.Asks[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	ob.LastUpdateID = snapshotUpdateID
	ob.LastUpdate = snapshot.Time

	// Apply buffered events that are newer than the snapshot
	for _, event := range ob.bufferedEvents {
		if event.LastUpdateID <= snapshotUpdateID {
			// Discard events that are older than our snapshot
			continue
		}
		if event.FirstUpdateID > snapshotUpdateID+1 {
			// Gap detected - we need to restart
			ob.isInitialized = false
			ob.bufferedEvents = nil
			return fmt.Errorf("order book initialization failed: gap detected between snapshot and buffered events")
		}

		// Apply the event
		for _, level := range event.Bids {
			priceStr := level.Price.String()
			if level.Qty.Sign() == 0 {
				delete(ob.Bids, priceStr)
				continue
			}
			if ob.Bids[priceStr] == nil {
				ob.Bids[priceStr] = new(big.Rat)
			}
			ob.Bids[priceStr].Set(level.Qty)
		}

		for _, level := range event.Asks {
			priceStr := level.Price.String()
			if level.Qty.Sign() == 0 {
				delete(ob.Asks, priceStr)
				continue
			}
			if ob.Asks[priceStr] == nil {
				ob.Asks[priceStr] = new(big.Rat)
			}
			ob.Asks[priceStr].Set(level.Qty)
		}

		ob.LastUpdateID = event.LastUpdateID
		ob.LastUpdate = event.EventTime
	}

	ob.isInitialized = true
	ob.bufferedEvents = nil // Clear buffered events after successful initialization
	return nil
}

// BufferEvent buffers a depth event during initialization
func (ob *OrderBook) BufferEvent(firstUpdateID, lastUpdateID int64, bids, asks []corews.DepthLevel, eventTime time.Time) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if ob.isInitialized {
		// Already initialized, apply directly
		UpdateFromBinanceDelta(ob, bids, asks, firstUpdateID, lastUpdateID, eventTime)
		return
	}

	// Buffer the event
	ob.bufferedEvents = append(ob.bufferedEvents, &BufferedDepthEvent{
		FirstUpdateID: firstUpdateID,
		LastUpdateID:  lastUpdateID,
		Bids:          bids,
		Asks:          asks,
		EventTime:     eventTime,
	})
}

// IsInitialized returns whether the order book has been initialized with a snapshot
func (ob *OrderBook) IsInitialized() bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.isInitialized
}

// WithWriteLock executes fn while holding the order book write lock.
func (ob *OrderBook) WithWriteLock(fn func(ob *OrderBook)) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	fn(ob)
}

// UpdateFromSnapshot updates the order book from a full snapshot.
func (ob *OrderBook) UpdateFromSnapshot(bids, asks []corews.DepthLevel, lastUpdateID int64, updateTime time.Time) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Bids = make(map[string]*big.Rat)
	ob.Asks = make(map[string]*big.Rat)

	for _, level := range bids {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			ob.Bids[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	for _, level := range asks {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			ob.Asks[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	ob.LastUpdateID = lastUpdateID
	ob.LastUpdate = updateTime
}

// GetSnapshot returns the current order book as a core BookEvent.
func (ob *OrderBook) GetSnapshot() corews.BookEvent {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]corews.DepthLevel, 0, len(ob.Bids))
	asks := make([]corews.DepthLevel, 0, len(ob.Asks))

	for priceStr, qty := range ob.Bids {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			bids = append(bids, corews.DepthLevel{
				Price: price,
				Qty:   new(big.Rat).Set(qty),
			})
		}
	}

	for priceStr, qty := range ob.Asks {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			asks = append(asks, corews.DepthLevel{
				Price: price,
				Qty:   new(big.Rat).Set(qty),
			})
		}
	}

	return corews.BookEvent{
		Symbol: ob.Symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   ob.LastUpdate,
	}
}

// GetLastUpdateID returns the last processed update ID.
func (ob *OrderBook) GetLastUpdateID() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.LastUpdateID
}

// SetLastUpdateID sets the last update ID.
func (ob *OrderBook) SetLastUpdateID(updateID int64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.LastUpdateID = updateID
}

// UpdateFromBinanceDelta applies Binance incremental book updates to the order book.
// According to Binance documentation:
// - If event u (last update ID) is < the update ID of your local order book, ignore the event
// - If event U (first update ID) is > the update ID of your local order book, something went wrong. Discard your local order book and restart
// - For each price level in bids (b) and asks (a), set the new quantity in the order book
// - If the quantity is zero, remove the price level from the order book
// - Set the order book update ID to the last update ID (u) in the processed event
func UpdateFromBinanceDelta(orderBook *OrderBook, bids, asks []corews.DepthLevel, firstUpdateID, lastUpdateID int64, updateTime time.Time) bool {
	success := true

	orderBook.WithWriteLock(func(ob *OrderBook) {
		// If event u (last update ID) is < the update ID of your local order book, ignore the event
		if lastUpdateID <= ob.LastUpdateID {
			return
		}

		// If event U (first update ID) is > the update ID of your local order book, something went wrong
		if firstUpdateID > ob.LastUpdateID+1 {
			// Discard local order book - caller should restart the process
			ob.Bids = make(map[string]*big.Rat)
			ob.Asks = make(map[string]*big.Rat)
			ob.LastUpdateID = 0
			success = false
			return
		}

		// Apply bid updates
		for _, level := range bids {
			priceStr := level.Price.String()
			if level.Qty.Sign() == 0 {
				delete(ob.Bids, priceStr)
				continue
			}
			if ob.Bids[priceStr] == nil {
				ob.Bids[priceStr] = new(big.Rat)
			}
			ob.Bids[priceStr].Set(level.Qty)
		}

		// Apply ask updates
		for _, level := range asks {
			priceStr := level.Price.String()
			if level.Qty.Sign() == 0 {
				delete(ob.Asks, priceStr)
				continue
			}
			if ob.Asks[priceStr] == nil {
				ob.Asks[priceStr] = new(big.Rat)
			}
			ob.Asks[priceStr].Set(level.Qty)
		}

		// Set the order book update ID to the last update ID (u) in the processed event
		ob.LastUpdateID = lastUpdateID
		ob.LastUpdate = updateTime
	})

	return success
}
