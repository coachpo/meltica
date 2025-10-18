// Package binance provides order book assembly functionality.
package binance

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/telemetry"
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
	mu     sync.Mutex
	bids   map[string]string
	asks   map[string]string
	seq    uint64
	ready  bool
	buffer []bufferedUpdate
	symbol string // For telemetry attribution

	// Telemetry
	gapDetectedCounter       metric.Int64Counter
	bufferSizeGauge          metric.Int64UpDownCounter
	staleUpdateCounter       metric.Int64Counter
	snapshotAppliedCounter   metric.Int64Counter
	updatesReplayedCounter   metric.Int64Counter
	coldStartDuration        metric.Float64Histogram
	coldStartTime            time.Time
}

// bufferedUpdate stores an update received before the initial snapshot.
type bufferedUpdate struct {
	bids          []schema.PriceLevel
	asks          []schema.PriceLevel
	firstUpdateID uint64
	finalUpdateID uint64
}

// NewBookAssembler constructs an empty order book assembler.
func NewBookAssembler() *BookAssembler {
	return NewBookAssemblerWithSymbol("")
}

// NewBookAssemblerWithSymbol constructs an order book assembler with telemetry for a specific symbol.
func NewBookAssemblerWithSymbol(symbol string) *BookAssembler {
	meter := otel.Meter("orderbook")
	
	//nolint:exhaustruct // Telemetry fields initialized below
	assembler := &BookAssembler{
		bids:   make(map[string]string),
		asks:   make(map[string]string),
		buffer: make([]bufferedUpdate, 0, 64),
		symbol: symbol,
		coldStartTime: time.Now(),
	}

	// Initialize telemetry
	assembler.gapDetectedCounter, _ = meter.Int64Counter(
		"orderbook.gap.detected",
		metric.WithDescription("Number of sequence gaps detected"),
		metric.WithUnit("{gap}"),
	)
	assembler.bufferSizeGauge, _ = meter.Int64UpDownCounter(
		"orderbook.buffer.size",
		metric.WithDescription("Number of updates currently buffered"),
		metric.WithUnit("{update}"),
	)
	assembler.staleUpdateCounter, _ = meter.Int64Counter(
		"orderbook.update.stale",
		metric.WithDescription("Number of stale updates rejected"),
		metric.WithUnit("{update}"),
	)
	assembler.snapshotAppliedCounter, _ = meter.Int64Counter(
		"orderbook.snapshot.applied",
		metric.WithDescription("Number of snapshots applied"),
		metric.WithUnit("{snapshot}"),
	)
	assembler.updatesReplayedCounter, _ = meter.Int64Counter(
		"orderbook.updates.replayed",
		metric.WithDescription("Number of buffered updates replayed"),
		metric.WithUnit("{update}"),
	)
	assembler.coldStartDuration, _ = meter.Float64Histogram(
		"orderbook.coldstart.duration",
		metric.WithDescription("Time from first update to snapshot ready"),
		metric.WithUnit("ms"),
	)

	return assembler
}

// ApplySnapshot ingests a full depth snapshot and resets internal state.
// It then replays any buffered updates that were received before the snapshot.
func (a *BookAssembler) ApplySnapshot(snapshot schema.BookSnapshotPayload, seq uint64) (schema.BookSnapshotPayload, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("environment", telemetry.Environment()),
		attribute.String("provider", "binance"),
		attribute.String("symbol", a.symbol),
		attribute.String("event_type", "book_snapshot"),
		attribute.Int64("sequence", safeUint64ToInt64(seq)),
	}

	// Initialize book with snapshot
	a.resetBooks()
	for _, level := range snapshot.Bids {
		a.upsertLevel(a.bids, level)
	}
	for _, level := range snapshot.Asks {
		a.upsertLevel(a.asks, level)
	}

	a.seq = seq
	wasReady := a.ready
	a.ready = true

	// Record snapshot applied
	if a.snapshotAppliedCounter != nil {
		a.snapshotAppliedCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
	}

	// Record cold start duration if this is the first snapshot
	if !wasReady && a.coldStartDuration != nil {
		duration := time.Since(a.coldStartTime).Milliseconds()
		a.coldStartDuration.Record(ctx, float64(duration), metric.WithAttributes(attrs...))
	}

	// Replay buffered updates
	if len(a.buffer) > 0 {
		bufferedCount := len(a.buffer)
		// Discard buffered updates with u <= snapshot's lastUpdateId
		validUpdates := make([]bufferedUpdate, 0, len(a.buffer))
		for _, upd := range a.buffer {
			if upd.finalUpdateID > seq {
				validUpdates = append(validUpdates, upd)
			}
		}

		// Sort by firstUpdateID to ensure correct replay order
		sort.Slice(validUpdates, func(i, j int) bool {
			return validUpdates[i].firstUpdateID < validUpdates[j].firstUpdateID
		})

		// Replay each buffered update
		for _, upd := range validUpdates {
			// Verify continuity: U should be <= seq+1 <= u
			if upd.firstUpdateID > a.seq+1 {
				// Gap detected during replay - this shouldn't happen but handle it
				a.buffer = nil
				return schema.BookSnapshotPayload{}, fmt.Errorf("%w: gap during replay U=%d currentSeq=%d", ErrBookSequenceGap, upd.firstUpdateID, a.seq)
			}

			// Apply the buffered update
			for _, level := range upd.bids {
				a.upsertLevel(a.bids, level)
			}
			for _, level := range upd.asks {
				a.upsertLevel(a.asks, level)
			}
			a.seq = upd.finalUpdateID
		}

		// Record replayed updates count
		replayedCount := len(validUpdates)
		if a.updatesReplayedCounter != nil && replayedCount > 0 {
			a.updatesReplayedCounter.Add(ctx, int64(replayedCount), metric.WithAttributes(
				append(attrs, 
					attribute.Int("buffered", bufferedCount),
					attribute.Int("valid", replayedCount),
				)...))
		}

		// Clear buffer after successful replay
		a.buffer = nil

		// Update buffer gauge
		if a.bufferSizeGauge != nil {
			a.bufferSizeGauge.Add(ctx, -int64(bufferedCount), metric.WithAttributes(attrs...))
		}
	}

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
// - If snapshot not received yet: buffer the update for later replay
// - If u < currentUpdateId: ignore (stale)
// - If U > currentUpdateId + 1: gap detected, restart required
// - Otherwise: apply the update
func (a *BookAssembler) ApplyUpdate(bids, asks []schema.PriceLevel, firstUpdateID, finalUpdateID uint64) (schema.BookSnapshotPayload, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	ctx := context.Background()
	attrs := []attribute.KeyValue{
		attribute.String("environment", telemetry.Environment()),
		attribute.String("provider", "binance"),
		attribute.String("symbol", a.symbol),
		attribute.String("event_type", "book_snapshot"),
		attribute.Int64("first_seq", safeUint64ToInt64(firstUpdateID)),
		attribute.Int64("final_seq", safeUint64ToInt64(finalUpdateID)),
		attribute.Int64("current_seq", safeUint64ToInt64(a.seq)),
	}

	if !a.ready {
		// Cold start: buffer updates until snapshot arrives
		a.buffer = append(a.buffer, bufferedUpdate{
			bids:          clonePriceLevels(bids),
			asks:          clonePriceLevels(asks),
			firstUpdateID: firstUpdateID,
			finalUpdateID: finalUpdateID,
		})

		// Update buffer size gauge
		if a.bufferSizeGauge != nil {
			a.bufferSizeGauge.Add(ctx, 1, metric.WithAttributes(attrs...))
		}

		return schema.BookSnapshotPayload{}, ErrBookNotInitialized
	}
	
	// If u < currentUpdateId: ignore (stale event)
	if finalUpdateID <= a.seq {
		// Record stale update
		if a.staleUpdateCounter != nil {
			a.staleUpdateCounter.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		return schema.BookSnapshotPayload{}, ErrBookStaleUpdate
	}
	
	// If U > currentUpdateId + 1: gap detected, restart required
	if firstUpdateID > a.seq+1 {
		gap := firstUpdateID - a.seq - 1

		// Record gap detection
		if a.gapDetectedCounter != nil {
			a.gapDetectedCounter.Add(ctx, 1, metric.WithAttributes(
				append(attrs, 
					attribute.Int64("gap_size", safeUint64ToInt64(gap)),
					attribute.String("recovery_action", "snapshot_restart"),
				)...))
		}

		// Buffer this update before resetting (it will be replayed after new snapshot)
		a.buffer = append(a.buffer, bufferedUpdate{
			bids:          clonePriceLevels(bids),
			asks:          clonePriceLevels(asks),
			firstUpdateID: firstUpdateID,
			finalUpdateID: finalUpdateID,
		})

		// Update buffer size gauge
		if a.bufferSizeGauge != nil {
			a.bufferSizeGauge.Add(ctx, 1, metric.WithAttributes(attrs...))
		}
		
		// Reset to cold start state to rebuffer
		a.ready = false
		a.resetBooks()
		a.seq = 0
		a.coldStartTime = time.Now() // Start tracking recovery time
		// Note: buffer is NOT cleared - we keep the gap-triggering update
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

// safeUint64ToInt64 safely converts uint64 to int64, clamping at max int64 to prevent overflow.
func safeUint64ToInt64(v uint64) int64 {
	const maxInt64 = int64(^uint64(0) >> 1)
	if v > uint64(maxInt64) {
		return maxInt64
	}
	return int64(v)
}

// clonePriceLevels creates a deep copy of price levels to safely buffer them.
func clonePriceLevels(levels []schema.PriceLevel) []schema.PriceLevel {
	if len(levels) == 0 {
		return nil
	}
	clone := make([]schema.PriceLevel, len(levels))
	copy(clone, levels)
	return clone
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
