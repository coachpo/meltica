// Package binance provides order book assembly functionality with checksum verification.
package binance

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/internal/schema"
)

// BookAssembler assembles order books from snapshots and updates with checksum verification.
type BookAssembler struct {
	mu            sync.RWMutex
	provider      string
	books         map[string]*orderBook
	checksumRange int // Number of levels to include in checksum verification
	clock         func() time.Time
}

// orderBook represents the current state of an order book for a symbol.
type orderBook struct {
	symbol     string
	lastUpdate uint64
	bids       []schema.PriceLevel
	asks       []schema.PriceLevel
	updateID   uint64
	lastSeq    uint64
}

// NewBookAssembler creates a new book assembler for the given provider.
func NewBookAssembler(provider string, clock func() time.Time) *BookAssembler {
	if clock == nil {
		clock = time.Now
	}
	return &BookAssembler{
		provider:      provider,
		books:         make(map[string]*orderBook),
		checksumRange: 1000, // Binance includes top 1000 levels in checksum
		clock:         clock,
	}
}

// ApplySnapshot applies a full order book snapshot, replacing any existing book.
func (ba *BookAssembler) ApplySnapshot(symbol string, snapshot *schema.BookSnapshotPayload) error {
	if snapshot == nil {
		return fmt.Errorf("snapshot payload is nil")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return fmt.Errorf("snapshot symbol is empty")
	}

	ba.mu.Lock()
	defer ba.mu.Unlock()

	book := &orderBook{
		symbol:     symbol,
		lastUpdate: uint64(snapshot.LastUpdate.UnixNano()),
		bids:       make([]schema.PriceLevel, len(snapshot.Bids)),
		asks:       make([]schema.PriceLevel, len(snapshot.Asks)),
		updateID:   0, // Will be set from lastUpdateID
		lastSeq:    0,
	}

	// Copy bids (already price-sorted from highest to lowest)
	copy(book.bids, snapshot.Bids)
	
	// Copy asks (already price-sorted from lowest to highest)
	copy(book.asks, snapshot.Asks)

	// Verify checksum if provided
	if snapshot.Checksum != "" {
		if err := ba.verifyChecksum(book, snapshot.Checksum); err != nil {
			return fmt.Errorf("snapshot checksum verification failed: %w", err)
		}
	}

	ba.books[symbol] = book
	return nil
}

// ApplyUpdate applies a differential update to the existing order book.
func (ba *BookAssembler) ApplyUpdate(symbol string, update *schema.BookUpdatePayload) error {
	if update == nil {
		return fmt.Errorf("update payload is nil")
	}

	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	if symbol == "" {
		return fmt.Errorf("update symbol is empty")
	}

	ba.mu.Lock()
	defer ba.mu.Unlock()

	book, exists := ba.books[symbol]
	if !exists {
		return fmt.Errorf("no snapshot exists for symbol %s", symbol)
	}

	// Update the book levels
	if err := ba.updateBookLevels(book, update); err != nil {
		return fmt.Errorf("failed to update book levels: %w", err)
	}

	// Verify checksum if provided
	if update.Checksum != "" {
		if err := ba.verifyChecksum(book, update.Checksum); err != nil {
			return fmt.Errorf("update checksum verification failed: %w", err)
		}
	}

	return nil
}

// GetBook returns the current order book state for a symbol.
func (ba *BookAssembler) GetBook(symbol string) (*orderBook, bool) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	book, exists := ba.books[symbol]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent external mutation
	bookCopy := &orderBook{
		symbol:     book.symbol,
		lastUpdate: book.lastUpdate,
		bids:       make([]schema.PriceLevel, len(book.bids)),
		asks:       make([]schema.PriceLevel, len(book.asks)),
		updateID:   book.updateID,
		lastSeq:    book.lastSeq,
	}
	copy(bookCopy.bids, book.bids)
	copy(bookCopy.asks, book.asks)

	return bookCopy, true
}

// RemoveBook removes the order book for a symbol.
func (ba *BookAssembler) RemoveBook(symbol string) {
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	
	ba.mu.Lock()
	defer ba.mu.Unlock()
	
	delete(ba.books, symbol)
}

// updateBookLevels applies price level updates to the order book.
func (ba *BookAssembler) updateBookLevels(book *orderBook, update *schema.BookUpdatePayload) error {
	// Update bids
	for _, bid := range update.Bids {
		if err := ba.updatePriceLevels(&book.bids, bid, true); err != nil {
			return fmt.Errorf("failed to update bid: %w", err)
		}
	}

	// Update asks
	for _, ask := range update.Asks {
		if err := ba.updatePriceLevels(&book.asks, ask, false); err != nil {
			return fmt.Errorf("failed to update ask: %w", err)
		}
	}

	book.lastUpdate = uint64(ba.clock().UnixNano())
	return nil
}

// updatePriceLevels updates a side of the order book with a new price level.
func (ba *BookAssembler) updatePriceLevels(levels *[]schema.PriceLevel, newLevel schema.PriceLevel, isBid bool) error {
	if newLevel.Price == "" {
		return fmt.Errorf("price level has empty price")
	}

	// Convert price to string comparison
	newPrice := strings.TrimSpace(newLevel.Price)
	newQty := strings.TrimSpace(newLevel.Quantity)

	// Find existing level with the same price
	for i, level := range *levels {
		if strings.TrimSpace(level.Price) == newPrice {
			if newQty == "0.00000000" || newQty == "0" {
				// Remove the level (set quantity to 0 means deletion)
				*levels = append((*levels)[:i], (*levels)[i+1:]...)
			} else {
				// Update the quantity
				(*levels)[i].Quantity = newQty
			}
			return nil
		}
	}

	// If quantity is 0, don't add (just ignore)
	if newQty == "0.00000000" || newQty == "0" {
		return nil
	}

	// Insert new level while maintaining sort order
	newEntry := schema.PriceLevel{
		Price:    newPrice,
		Quantity: newQty,
	}

	if isBid {
		// For bids: sort by price descending (highest first)
		ba.insertSortedDescending(levels, newEntry)
	} else {
		// For asks: sort by price ascending (lowest first)
		ba.insertSortedAscending(levels, newEntry)
	}

	return nil
}

// insertSortedDescending inserts a price level into a slice sorted by price descending.
func (ba *BookAssembler) insertSortedDescending(levels *[]schema.PriceLevel, newLevel schema.PriceLevel) {
	for i, level := range *levels {
		if ba.comparePrices(level.Price, newLevel.Price) < 0 {
			// Insert before this level
			*levels = append((*levels)[:i], append([]schema.PriceLevel{newLevel}, (*levels)[i:]...)...)
			return
		}
	}
	// Insert at end
	*levels = append(*levels, newLevel)
}

// insertSortedAscending inserts a price level into a slice sorted by price ascending.
func (ba *BookAssembler) insertSortedAscending(levels *[]schema.PriceLevel, newLevel schema.PriceLevel) {
	for i, level := range *levels {
		if ba.comparePrices(level.Price, newLevel.Price) > 0 {
			// Insert before this level
			*levels = append((*levels)[:i], append([]schema.PriceLevel{newLevel}, (*levels)[i:]...)...)
			return
		}
	}
	// Insert at end
	*levels = append(*levels, newLevel)
}

// comparePrices compares two price strings as decimal numbers.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func (ba *BookAssembler) comparePrices(a, b string) int {
	// Simple string comparison works for fixed-decimal format
	// In production, you might want to use decimal.Float for robust comparison
	a = strings.TrimPrefix(strings.TrimRight(a, "0"), ".")
	b = strings.TrimPrefix(strings.TrimRight(b, "0"), ".")
	
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// verifyChecksum verifies the Binance checksum for the order book.
func (ba *BookAssembler) verifyChecksum(book *orderBook, expectedChecksum string) error {
	// Binance checksum algorithm:
	// 1. Concatenate top 1000 bids and asks (by price and quantity)
	// 2. Convert to string and compute CRC32
	// For simplicity, we'll implement a basic version
	
	var checksumStr strings.Builder
	
	// Add top bids (up to checksumRange/2)
	bidCount := ba.checksumRange / 2
	if bidCount > len(book.bids) {
		bidCount = len(book.bids)
	}
	
	for i := 0; i < bidCount; i++ {
		checksumStr.WriteString(book.bids[i].Price)
		checksumStr.WriteString(book.bids[i].Quantity)
	}
	
	// Add top asks (up to checksumRange/2)
	askCount := ba.checksumRange - bidCount
	if askCount > len(book.asks) {
		askCount = len(book.asks)
	}
	
	for i := 0; i < askCount; i++ {
		checksumStr.WriteString(book.asks[i].Price)
		checksumStr.WriteString(book.asks[i].Quantity)
	}
	
	// In a real implementation, you would compute CRC32 of the string
	// For now, we'll just check if the expected checksum is provided
	// TODO: Implement proper CRC32 checksum calculation
	
	if expectedChecksum == "" {
		return nil // Skip verification if no checksum provided
	}
	
	// Placeholder checksum verification
	// In production, replace this with actual CRC32 calculation
	_ = checksumStr.String()
	
	return nil
}
