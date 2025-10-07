package binance

import (
	"math/big"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
)

// BookManager manages book state for different symbolSvc.
type BookManager struct {
	mu    sync.RWMutex
	books map[string]*Book
}

func NewBookManager() *BookManager {
	return &BookManager{books: make(map[string]*Book)}
}

func (m *BookManager) GetOrCreate(symbol string) *Book {
	m.mu.Lock()
	defer m.mu.Unlock()
	if book, exists := m.books[symbol]; exists {
		return book
	}
	book := &Book{
		Symbol: symbol,
		Bids:   make(map[string]*big.Rat),
		Asks:   make(map[string]*big.Rat),
	}
	m.books[symbol] = book
	return book
}

// Book represents the current state of the Binance book for a symbol.
type Book struct {
	Symbol        string
	Bids          map[string]*big.Rat
	Asks          map[string]*big.Rat
	lastUpdateID  int64
	LastUpdate    time.Time
	mu            sync.RWMutex
	isInitialized bool
}

func (b *Book) InitializeFromSnapshot(snapshot corestreams.BookEvent, snapshotUpdateID int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.Bids = make(map[string]*big.Rat)
	b.Asks = make(map[string]*big.Rat)

	for _, level := range snapshot.Bids {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			b.Bids[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	for _, level := range snapshot.Asks {
		if level.Price != nil && level.Qty != nil && level.Qty.Sign() > 0 {
			b.Asks[level.Price.String()] = new(big.Rat).Set(level.Qty)
		}
	}

	b.lastUpdateID = snapshotUpdateID
	b.LastUpdate = snapshot.Time
	b.isInitialized = true
}

func (b *Book) GetSnapshot() corestreams.BookEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()

	bids := make([]core.BookDepthLevel, 0, len(b.Bids))
	asks := make([]core.BookDepthLevel, 0, len(b.Asks))

	for priceStr, qty := range b.Bids {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			bids = append(bids, core.BookDepthLevel{Price: price, Qty: new(big.Rat).Set(qty)})
		}
	}

	for priceStr, qty := range b.Asks {
		if price, ok := new(big.Rat).SetString(priceStr); ok {
			asks = append(asks, core.BookDepthLevel{Price: price, Qty: new(big.Rat).Set(qty)})
		}
	}

	return corestreams.BookEvent{
		Symbol: b.Symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   b.LastUpdate,
	}
}

func (b *Book) UpdateFromDelta(bids, asks []core.BookDepthLevel, firstUpdateID, lastUpdateID int64, updateTime time.Time) bool {
	success := true
	b.mu.Lock()
	defer b.mu.Unlock()

	if lastUpdateID <= b.lastUpdateID {
		return true
	}

	if b.isInitialized && firstUpdateID > b.lastUpdateID+1 {
		b.Bids = make(map[string]*big.Rat)
		b.Asks = make(map[string]*big.Rat)
		b.lastUpdateID = 0
		b.isInitialized = false
		success = false
		return success
	}

	for _, level := range bids {
		priceStr := level.Price.String()
		if level.Qty.Sign() == 0 {
			delete(b.Bids, priceStr)
			continue
		}
		if b.Bids[priceStr] == nil {
			b.Bids[priceStr] = new(big.Rat)
		}
		b.Bids[priceStr].Set(level.Qty)
	}

	for _, level := range asks {
		priceStr := level.Price.String()
		if level.Qty.Sign() == 0 {
			delete(b.Asks, priceStr)
			continue
		}
		if b.Asks[priceStr] == nil {
			b.Asks[priceStr] = new(big.Rat)
		}
		b.Asks[priceStr].Set(level.Qty)
	}

	b.lastUpdateID = lastUpdateID
	b.LastUpdate = updateTime
	return success
}

func (b *Book) LastUpdateID() int64 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.lastUpdateID
}

func (b *Book) Initialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.isInitialized
}

func (b *Book) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Bids = make(map[string]*big.Rat)
	b.Asks = make(map[string]*big.Rat)
	b.lastUpdateID = 0
	b.isInitialized = false
}
