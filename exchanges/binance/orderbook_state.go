package binance

import (
	"math/big"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

// OrderBookManager manages order book state for different symbols.
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{books: make(map[string]*OrderBook)}
}

func (m *OrderBookManager) GetOrCreate(symbol string) *OrderBook {
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
	Symbol        string
	Bids          map[string]*big.Rat
	Asks          map[string]*big.Rat
	lastUpdateID  int64
	LastUpdate    time.Time
	mu            sync.RWMutex
	isInitialized bool
}

func (ob *OrderBook) InitializeFromSnapshot(snapshot corestreams.BookEvent, snapshotUpdateID int64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	ob.Bids = make(map[string]*big.Rat)
	ob.Asks = make(map[string]*big.Rat)

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

	ob.lastUpdateID = snapshotUpdateID
	ob.LastUpdate = snapshot.Time
	ob.isInitialized = true
}

func (ob *OrderBook) GetSnapshot() corestreams.BookEvent {
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

	return corestreams.BookEvent{
		Symbol: ob.Symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   ob.LastUpdate,
	}
}

func (ob *OrderBook) UpdateFromDelta(bids, asks []core.BookDepthLevel, firstUpdateID, lastUpdateID int64, updateTime time.Time) bool {
	success := true
	ob.mu.Lock()
	defer ob.mu.Unlock()

	if lastUpdateID <= ob.lastUpdateID {
		return true
	}

	if ob.isInitialized && firstUpdateID > ob.lastUpdateID+1 {
		ob.Bids = make(map[string]*big.Rat)
		ob.Asks = make(map[string]*big.Rat)
		ob.lastUpdateID = 0
		ob.isInitialized = false
		success = false
		return success
	}

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

	ob.lastUpdateID = lastUpdateID
	ob.LastUpdate = updateTime
	return success
}

func (ob *OrderBook) LastUpdateID() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.lastUpdateID
}

func (ob *OrderBook) Initialized() bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.isInitialized
}

func (ob *OrderBook) Reset() {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.Bids = make(map[string]*big.Rat)
	ob.Asks = make(map[string]*big.Rat)
	ob.lastUpdateID = 0
	ob.isInitialized = false
}
