// Package binance provides order book assembly functionality.
package binance

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/internal/schema"
)

const (
	// maxOutputDepth limits the number of price levels returned in snapshots.
	// Binance REST API supports up to 5000 levels per side, but we cap output
	// to balance bandwidth and usefulness.
	maxOutputDepth = 1000
)

var (
	// ErrBookNotInitialized is returned when updates arrive before the first snapshot.
	ErrBookNotInitialized = fmt.Errorf("binance book assembler: snapshot required before updates")
	// ErrBookStaleUpdate is returned when an update with an older sequence is applied.
	ErrBookStaleUpdate = fmt.Errorf("binance book assembler: stale update")
	// ErrBookSequenceGap is returned when there's a gap in update sequence.
	ErrBookSequenceGap = fmt.Errorf("binance book assembler: sequence gap detected - restart required")
)

// BookAssembler keeps a canonical representation of the Binance order book
// and produces normalised payloads for downstream consumers.
type BookAssembler struct {
	mu    sync.Mutex
	bids  map[string]string
	asks  map[string]string
	seq   uint64
	ready bool
}

// NewBookAssembler constructs an empty order book assembler.
func NewBookAssembler() *BookAssembler {
	//nolint:exhaustruct // zero values for mu, seq, ready are intentional
	return &BookAssembler{
		bids: make(map[string]string),
		asks: make(map[string]string),
	}
}

// ApplySnapshot ingests a full depth snapshot and resets internal state.
func (a *BookAssembler) ApplySnapshot(snapshot schema.BookSnapshotPayload, seq uint64) (schema.BookSnapshotPayload, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.resetBooks()
	for _, level := range snapshot.Bids {
		a.upsertLevel(a.bids, level)
	}
	for _, level := range snapshot.Asks {
		a.upsertLevel(a.asks, level)
	}

	a.seq = seq
	a.ready = true

	//nolint:exhaustruct // FirstUpdateID and FinalUpdateID not needed for output snapshots
	sanitised := schema.BookSnapshotPayload{
		Bids:       a.topNLevelsLocked(a.bids, true, maxOutputDepth),
		Asks:       a.topNLevelsLocked(a.asks, false, maxOutputDepth),
		Checksum:   "",
		LastUpdate: snapshot.LastUpdate,
	}
	return sanitised, nil
}

// ApplyUpdate applies a delta update to the maintained order book and returns a full snapshot.
// This enforces the architecture principle that adapters emit only full snapshots, not deltas.
// Per Binance documentation:
// - If u < currentUpdateId: ignore (stale)
// - If U > currentUpdateId + 1: gap detected, restart required
// - Otherwise: apply the update
func (a *BookAssembler) ApplyUpdate(bids, asks []schema.PriceLevel, firstUpdateID, finalUpdateID uint64) (schema.BookSnapshotPayload, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.ready {
		return schema.BookSnapshotPayload{}, ErrBookNotInitialized
	}
	
	// If u < currentUpdateId: ignore (stale event)
	if finalUpdateID <= a.seq {
		return schema.BookSnapshotPayload{}, ErrBookStaleUpdate
	}
	
	// If U > currentUpdateId + 1: gap detected, restart required
	if firstUpdateID > a.seq+1 {
		return schema.BookSnapshotPayload{}, ErrBookSequenceGap
	}

	// Apply delta to internal state
	for _, level := range bids {
		a.upsertLevel(a.bids, level)
	}
	for _, level := range asks {
		a.upsertLevel(a.asks, level)
	}

	// Update sequence to finalUpdateID (u)
	a.seq = finalUpdateID

	// Return full snapshot (not delta)
	//nolint:exhaustruct // FirstUpdateID and FinalUpdateID not needed for output snapshots
	payload := schema.BookSnapshotPayload{
		Bids:       a.topNLevelsLocked(a.bids, true, maxOutputDepth),
		Asks:       a.topNLevelsLocked(a.asks, false, maxOutputDepth),
		Checksum:   "",
		LastUpdate: time.Now().UTC(),
	}
	return payload, nil
}

func (a *BookAssembler) upsertLevel(book map[string]string, level schema.PriceLevel) {
	price := strings.TrimSpace(level.Price)
	qty := strings.TrimSpace(level.Quantity)
	if price == "" {
		return
	}
	if qty == "" {
		delete(book, price)
		return
	}
	value, err := strconv.ParseFloat(qty, 64)
	if err != nil {
		book[price] = qty
		return
	}
	if value == 0 {
		delete(book, price)
		return
	}
	book[price] = qty
}

func (a *BookAssembler) resetBooks() {
	a.bids = make(map[string]string, len(a.bids))
	a.asks = make(map[string]string, len(a.asks))
}

func (a *BookAssembler) topNLevelsLocked(book map[string]string, desc bool, limit int) []schema.PriceLevel {
	if len(book) == 0 {
		return nil
	}
	type priceLevel struct {
		price float64
		raw   schema.PriceLevel
	}
	levels := make([]priceLevel, 0, len(book))
	for price, qty := range book {
		fv, err := strconv.ParseFloat(price, 64)
		if err != nil {
			continue
		}
		levels = append(levels, priceLevel{
			price: fv,
			raw: schema.PriceLevel{
				Price:    price,
				Quantity: qty,
			},
		})
	}
	sort.Slice(levels, func(i, j int) bool {
		if desc {
			return levels[i].price > levels[j].price
		}
		return levels[i].price < levels[j].price
	})

	if len(levels) < limit {
		limit = len(levels)
	}
	out := make([]schema.PriceLevel, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, levels[i].raw)
	}
	return out
}
