package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
)

// BufferedBookDepthEvent represents a depth update event that can be buffered
type BufferedBookDepthEvent struct {
	FirstUpdateID int64
	LastUpdateID  int64
	Bids          [][]string
	Asks          [][]string
	Time          time.Time
	Processed     bool
}

// OrderBookManager manages order book state for different symbols with Binance-recommended buffering.
type OrderBookManager struct {
	mu    sync.RWMutex
	books map[string]*OrderBook
}

// OrderBook represents the current state of an order book for a symbol with proper buffering and synchronization.
type OrderBook struct {
	Symbol          string
	Bids            map[string]*big.Rat // price -> quantity
	Asks            map[string]*big.Rat // price -> quantity
	LastUpdateID    int64               // Last processed update ID
	LastUpdate      time.Time
	Buffer          []BufferedBookDepthEvent // Buffered events waiting for snapshot
	FirstBufferedID int64                    // U of first buffered event
	SnapshotPending bool                     // Whether we're waiting for a snapshot
	mu              sync.RWMutex
}

// NewOrderBookManager creates a new order book manager.
func NewOrderBookManager() *OrderBookManager {
	return &OrderBookManager{
		books: make(map[string]*OrderBook),
	}
}

// GetOrCreateOrderBook gets an existing order book or creates a new one.
func (m *OrderBookManager) GetOrCreateOrderBook(symbol string) *OrderBook {
	m.mu.Lock()
	defer m.mu.Unlock()

	if book, exists := m.books[symbol]; exists {
		return book
	}

	book := &OrderBook{
		Symbol:          symbol,
		Bids:            make(map[string]*big.Rat),
		Asks:            make(map[string]*big.Rat),
		Buffer:          make([]BufferedBookDepthEvent, 0),
		SnapshotPending: true, // Start by requesting a snapshot
	}
	m.books[symbol] = book
	return book
}

// ProcessBookUpdate processes a depth update according to Binance guidelines
func (ob *OrderBook) ProcessBookUpdate(firstUpdateID, lastUpdateID int64, bids, asks [][]string, updateTime time.Time) error {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	// If this is the first event and we haven't buffered anything yet, start buffering
	if ob.FirstBufferedID == 0 && ob.SnapshotPending {
		ob.FirstBufferedID = firstUpdateID
	}

	// Buffer the event
	event := BufferedBookDepthEvent{
		FirstUpdateID: firstUpdateID,
		LastUpdateID:  lastUpdateID,
		Bids:          bids,
		Asks:          asks,
		Time:          updateTime,
		Processed:     false,
	}
	ob.Buffer = append(ob.Buffer, event)

	// If we have a snapshot and the first buffered event is valid, process buffered events
	if !ob.SnapshotPending && ob.LastUpdateID > 0 {
		return ob.processBufferedEvents()
	}

	return nil
}

// processBufferedEvents processes all buffered events according to Binance guidelines
func (ob *OrderBook) processBufferedEvents() error {
	// Discard any event where u is <= lastUpdateId of the snapshot
	filteredBuffer := make([]BufferedBookDepthEvent, 0)
	for _, event := range ob.Buffer {
		if event.LastUpdateID > ob.LastUpdateID {
			filteredBuffer = append(filteredBuffer, event)
		}
	}
	ob.Buffer = filteredBuffer

	// Process remaining buffered events
	for i := range ob.Buffer {
		if ob.Buffer[i].Processed {
			continue
		}

		event := &ob.Buffer[i]

		// Apply the official Binance update procedure
		if err := ob.applyUpdateProcedure(event); err != nil {
			return err
		}

		event.Processed = true
	}

	return nil
}

// applyUpdateProcedure applies the official Binance update procedure
func (ob *OrderBook) applyUpdateProcedure(event *BufferedBookDepthEvent) error {
	// Step 1: If the event u (last update ID) is < the update ID of your local order book, ignore the event
	if event.LastUpdateID < ob.LastUpdateID {
		return nil // Ignore the event
	}

	// Step 2: If the event U (first update ID) is > the update ID of your local order book, something went wrong
	if event.FirstUpdateID > ob.LastUpdateID {
		// Discard local order book and restart process
		ob.Bids = make(map[string]*big.Rat)
		ob.Asks = make(map[string]*big.Rat)
		ob.LastUpdateID = 0
		ob.SnapshotPending = true
		return fmt.Errorf("sequence mismatch: firstUpdateID %d > lastUpdateID %d, restarting order book",
			event.FirstUpdateID, ob.LastUpdateID)
	}

	// Step 3: Apply updates for each price level in bids and asks
	for _, bid := range event.Bids {
		if len(bid) < 2 {
			continue
		}
		priceStr := bid[0]
		qtyStr := bid[1]

		if qty, ok := parseDecimalToRat(qtyStr); ok {
			if qty.Sign() == 0 {
				// If quantity is zero, remove the price level
				delete(ob.Bids, priceStr)
			} else {
				// Insert or update the price level
				if ob.Bids[priceStr] == nil {
					ob.Bids[priceStr] = new(big.Rat)
				}
				ob.Bids[priceStr].Set(qty)
			}
		}
	}

	for _, ask := range event.Asks {
		if len(ask) < 2 {
			continue
		}
		priceStr := ask[0]
		qtyStr := ask[1]

		if qty, ok := parseDecimalToRat(qtyStr); ok {
			if qty.Sign() == 0 {
				// If quantity is zero, remove the price level
				delete(ob.Asks, priceStr)
			} else {
				// Insert or update the price level
				if ob.Asks[priceStr] == nil {
					ob.Asks[priceStr] = new(big.Rat)
				}
				ob.Asks[priceStr].Set(qty)
			}
		}
	}

	// Step 4: Set the order book update ID to the last update ID (u) in the processed event
	ob.LastUpdateID = event.LastUpdateID
	ob.LastUpdate = event.Time

	return nil
}

// GetSnapshotFromAPI retrieves a depth snapshot from Binance API
func (m *OrderBookManager) GetSnapshotFromAPI(ctx context.Context, symbol string) (*OrderBook, error) {
	// Convert canonical symbol to Binance format
	binanceSymbol := core.CanonicalToBinance(symbol)

	// Make HTTP request to get depth snapshot
	url := fmt.Sprintf("https://api.binance.com/api/v3/depth?symbol=%s&limit=5000", binanceSymbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	var apiResp struct {
		LastUpdateID int64      `json:"lastUpdateId"`
		Bids         [][]string `json:"bids"`
		Asks         [][]string `json:"asks"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, err
	}

	// Create or update order book with snapshot
	book := m.GetOrCreateOrderBook(symbol)

	book.mu.Lock()
	defer book.mu.Unlock()

	// Initialize order book from snapshot
	book.Bids = make(map[string]*big.Rat)
	book.Asks = make(map[string]*big.Rat)
	book.LastUpdateID = apiResp.LastUpdateID
	book.LastUpdate = time.Now()

	// Parse bids from snapshot
	for _, bid := range apiResp.Bids {
		if len(bid) < 2 {
			continue
		}
		if _, ok := parseDecimalToRat(bid[0]); ok {
			if qty, ok := parseDecimalToRat(bid[1]); ok && qty.Sign() > 0 {
				book.Bids[bid[0]] = qty
			}
		}
	}

	// Parse asks from snapshot
	for _, ask := range apiResp.Asks {
		if len(ask) < 2 {
			continue
		}
		if _, ok := parseDecimalToRat(ask[0]); ok {
			if qty, ok := parseDecimalToRat(ask[1]); ok && qty.Sign() > 0 {
				book.Asks[ask[0]] = qty
			}
		}
	}

	// Check if snapshot is valid according to Binance guidelines
	if book.FirstBufferedID > 0 && apiResp.LastUpdateID < book.FirstBufferedID {
		// Snapshot is too old, need to retry
		book.SnapshotPending = true
		return nil, fmt.Errorf("snapshot too old: lastUpdateId %d < firstBufferedId %d",
			apiResp.LastUpdateID, book.FirstBufferedID)
	}

	// Snapshot is valid, mark as no longer pending
	book.SnapshotPending = false

	return book, nil
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

// IsSnapshotPending returns whether a snapshot is needed
func (ob *OrderBook) IsSnapshotPending() bool {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.SnapshotPending
}

// GetLastUpdateID returns the last processed update ID
func (ob *OrderBook) GetLastUpdateID() int64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	return ob.LastUpdateID
}

// SetLastUpdateID sets the last update ID (used when initializing from snapshot).
func (ob *OrderBook) SetLastUpdateID(updateID int64) {
	ob.mu.Lock()
	defer ob.mu.Unlock()
	ob.LastUpdateID = updateID
}
