package ws

import (
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

// OrderBook represents the current state of the Coinbase order book for a symbol.
type OrderBook struct {
	Symbol       string
	Bids         map[string]*big.Rat
	Asks         map[string]*big.Rat
	LastUpdateID int64
	LastUpdate   time.Time
	mu           sync.RWMutex
}

// WithWriteLock executes fn while holding the order book write lock.
func (ob *OrderBook) WithWriteLock(fn func(ob *OrderBook)) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	fn(ob)
}

// UpdateFromSnapshot updates the order book from a full snapshot.
func (ob *OrderBook) UpdateFromSnapshot(bids, asks []corews.DepthLevel, updateTime time.Time) {
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

// UpdateFromCoinbaseDelta applies Coinbase level2 deltas to the order book.
func UpdateFromCoinbaseDelta(orderBook *OrderBook, bids, asks []corews.DepthLevel, updateTime time.Time) {
	orderBook.WithWriteLock(func(ob *OrderBook) {
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

		ob.LastUpdate = updateTime
	})
}
