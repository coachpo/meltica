package routing

import (
	"math/big"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
)

// OrderBookManager manages OKX order book state keyed by symbol.
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

// NewOrderBookManager constructs an order book manager.
func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{books: make(map[string]*OrderBook)}
}

// GetOrCreateOrderBook returns an existing order book or creates a new one.
func (m *OrderBookManager) GetOrCreateOrderBook(symbol string) *OrderBook {
	m.mu.Lock()
	defer m.mu.Unlock()

	if book, ok := m.books[symbol]; ok {
		return book
	}
	book := &OrderBook{Symbol: symbol, Bids: map[string]*big.Rat{}, Asks: map[string]*big.Rat{}}
	m.books[symbol] = book
	return book
}

// OrderBook represents the current order book snapshot for a symbol.
type OrderBook struct {
	Symbol       string
	Bids         map[string]*big.Rat
	Asks         map[string]*big.Rat
	LastUpdateID int64
	LastUpdate   time.Time
	mu           sync.RWMutex
}

// WithWriteLock executes fn while the order book write lock is held.
func (ob *OrderBook) WithWriteLock(fn func(ob *OrderBook)) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	fn(ob)
}

// UpdateFromSnapshot replaces the order book contents with a snapshot payload.
func (ob *OrderBook) UpdateFromSnapshot(bids, asks []core.BookDepthLevel, updateTime time.Time) {
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

// GetSnapshot returns the order book snapshot as a BookEvent.
func (ob *OrderBook) GetSnapshot() coreprovider.BookEvent {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]core.BookDepthLevel, 0, len(ob.Bids))
	asks := make([]core.BookDepthLevel, 0, len(ob.Asks))

	for priceStr, qty := range ob.Bids {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			bids = append(bids, core.BookDepthLevel{Price: price, Qty: new(big.Rat).Set(qty)})
		}
	}

	for priceStr, qty := range ob.Asks {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			asks = append(asks, core.BookDepthLevel{Price: price, Qty: new(big.Rat).Set(qty)})
		}
	}

	return coreprovider.BookEvent{Symbol: ob.Symbol, Bids: bids, Asks: asks, Time: ob.LastUpdate}
}

// SetLastUpdateID records the most recent sequence ID.
func (ob *OrderBook) SetLastUpdateID(updateID int64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.LastUpdateID = updateID
}

// GetLastUpdateID returns the last recorded sequence ID.
func (ob *OrderBook) GetLastUpdateID() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.LastUpdateID
}

// UpdateFromOKXDelta applies incremental updates while ensuring sequence continuity.
func UpdateFromOKXDelta(orderBook *OrderBook, bids, asks []core.BookDepthLevel, prevSeqID, seqID int64, updateTime time.Time) bool {
	success := true
	orderBook.WithWriteLock(func(ob *OrderBook) {
		if ob.LastUpdateID > 0 && prevSeqID != ob.LastUpdateID {
			ob.Bids = make(map[string]*big.Rat)
			ob.Asks = make(map[string]*big.Rat)
			ob.LastUpdateID = 0
			success = false
			return
		}

		for _, level := range bids {
			priceStr := level.Price.String()
			if level.Qty == nil || level.Qty.Sign() == 0 {
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
			if level.Qty == nil || level.Qty.Sign() == 0 {
				delete(ob.Asks, priceStr)
				continue
			}
			if ob.Asks[priceStr] == nil {
				ob.Asks[priceStr] = new(big.Rat)
			}
			ob.Asks[priceStr].Set(level.Qty)
		}

		ob.LastUpdateID = seqID
		ob.LastUpdate = updateTime
	})

	return success
}
