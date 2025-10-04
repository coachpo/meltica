package routing

import (
	"math/big"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
)

// OrderBookManager tracks order book state per symbol for Coinbase order book updates.
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

// NewOrderBookManager constructs an order book manager instance.
func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{books: make(map[string]*OrderBook)}
}

// GetOrCreateOrderBook returns an existing order book or creates a new one.
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

// OrderBook represents the current Coinbase order book state for a symbol.
type OrderBook struct {
	Symbol     string
	Bids       map[string]*big.Rat
	Asks       map[string]*big.Rat
	LastUpdate time.Time
	mu         sync.RWMutex
}

// WithWriteLock executes fn while holding the order book write lock.
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

// GetSnapshot returns the current order book snapshot as a BookEvent.
func (ob *OrderBook) GetSnapshot() coreprovider.BookEvent {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]core.BookDepthLevel, 0, len(ob.Bids))
	asks := make([]core.BookDepthLevel, 0, len(ob.Asks))

	for priceStr, qty := range ob.Bids {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			bids = append(bids, core.BookDepthLevel{
				Price: price,
				Qty:   new(big.Rat).Set(qty),
			})
		}
	}

	for priceStr, qty := range ob.Asks {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			asks = append(asks, core.BookDepthLevel{
				Price: price,
				Qty:   new(big.Rat).Set(qty),
			})
		}
	}

	return coreprovider.BookEvent{
		Symbol: ob.Symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   ob.LastUpdate,
	}
}

// UpdateFromCoinbaseDelta applies Level 2 delta updates to the order book.
func UpdateFromCoinbaseDelta(orderBook *OrderBook, bids, asks []core.BookDepthLevel, updateTime time.Time) {
	orderBook.WithWriteLock(func(ob *OrderBook) {
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

		ob.LastUpdate = updateTime
	})
}
