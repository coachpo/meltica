package binance

import (
	"fmt"
	"testing"

	"github.com/coachpo/meltica/internal/schema"
)

func TestBookAssembler_ApplyUpdate_NotInitialized(t *testing.T) {
	assembler := NewBookAssembler()
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: "1.0"}}
	asks := []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}}
	
	_, err := assembler.ApplyUpdate(bids, asks, 1, 1)
	if err != ErrBookNotInitialized {
		t.Errorf("expected ErrBookNotInitialized, got %v", err)
	}
	
	// Verify update was buffered
	if len(assembler.buffer) != 1 {
		t.Errorf("expected 1 buffered update, got %d", len(assembler.buffer))
	}
}

func TestBookAssembler_BufferAndReplay(t *testing.T) {
	assembler := NewBookAssembler()
	
	// Apply updates before snapshot arrives (cold start)
	bids1 := []schema.PriceLevel{{Price: "50000.00", Quantity: "1.0"}}
	asks1 := []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}}
	_, err := assembler.ApplyUpdate(bids1, asks1, 1001, 1003)
	if err != ErrBookNotInitialized {
		t.Errorf("expected ErrBookNotInitialized, got %v", err)
	}
	
	bids2 := []schema.PriceLevel{{Price: "49999.00", Quantity: "2.0"}}
	asks2 := []schema.PriceLevel{{Price: "50002.00", Quantity: "1.0"}}
	_, err = assembler.ApplyUpdate(bids2, asks2, 1004, 1006)
	if err != ErrBookNotInitialized {
		t.Errorf("expected ErrBookNotInitialized, got %v", err)
	}
	
	// Verify both updates are buffered
	if len(assembler.buffer) != 2 {
		t.Fatalf("expected 2 buffered updates, got %d", len(assembler.buffer))
	}
	
	// Apply snapshot with lastUpdateId=1000
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "5.0"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "3.0"}},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1000)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	// Verify buffer was cleared after replay
	if len(assembler.buffer) != 0 {
		t.Errorf("expected buffer to be cleared, got %d items", len(assembler.buffer))
	}
	
	// Verify sequence is now at the final buffered update
	if assembler.seq != 1006 {
		t.Errorf("expected seq=1006 after replay, got %d", assembler.seq)
	}
	
	// Verify the result includes replayed updates
	if len(result.Bids) == 0 || len(result.Asks) == 0 {
		t.Error("expected non-empty result after replay")
	}
	
	// The top bid should be 50000.00 with quantity 1.0 (from replayed update)
	if result.Bids[0].Price != "50000.00" {
		t.Errorf("expected top bid price 50000.00, got %s", result.Bids[0].Price)
	}
	if result.Bids[0].Quantity != "1.0" {
		t.Errorf("expected top bid quantity 1.0 (from replay), got %s", result.Bids[0].Quantity)
	}
}

func TestBookAssembler_BufferReplay_DiscardStaleUpdates(t *testing.T) {
	assembler := NewBookAssembler()
	
	// Buffer updates with various sequences
	_, _ = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "50000.00", Quantity: "1.0"}},
		[]schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}},
		500, 800, // This should be discarded (u=800 <= snapshot seq=1000)
	)
	
	_, _ = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "49999.00", Quantity: "2.0"}},
		[]schema.PriceLevel{{Price: "50002.00", Quantity: "1.0"}},
		1001, 1003, // This should be replayed
	)
	
	// Apply snapshot with lastUpdateId=1000
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "5.0"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "3.0"}},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1000)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	// Verify sequence advanced to 1003 (only the second update was replayed)
	if assembler.seq != 1003 {
		t.Errorf("expected seq=1003 after replay, got %d", assembler.seq)
	}
	
	// Verify second update was applied (49999.00 bid should exist)
	found := false
	for _, bid := range result.Bids {
		if bid.Price == "49999.00" && bid.Quantity == "2.0" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected replayed update (49999.00 bid) to be present")
	}
}

func TestBookAssembler_BufferReplay_SortsBySequence(t *testing.T) {
	assembler := NewBookAssembler()
	
	// Buffer updates in non-sequential order
	_, _ = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "49998.00", Quantity: "3.0"}},
		nil,
		1007, 1009,
	)
	
	_, _ = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "49999.00", Quantity: "2.0"}},
		nil,
		1004, 1006,
	)
	
	_, _ = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "50000.00", Quantity: "1.0"}},
		nil,
		1001, 1003,
	)
	
	// Apply snapshot
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "5.0"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "3.0"}},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1000)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	// Verify final sequence is from last replayed update
	if assembler.seq != 1009 {
		t.Errorf("expected seq=1009 after sorted replay, got %d", assembler.seq)
	}
	
	// Verify all three price levels are present
	expectedPrices := map[string]string{
		"50000.00": "1.0",
		"49999.00": "2.0",
		"49998.00": "3.0",
	}
	
	for price, qty := range expectedPrices {
		found := false
		for _, bid := range result.Bids {
			if bid.Price == price && bid.Quantity == qty {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected bid %s:%s to be present after replay", price, qty)
		}
	}
}

func TestBookAssembler_GapDetection_ResetsToBuffering(t *testing.T) {
	assembler := NewBookAssembler()
	
	// Initialize with snapshot
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "1.5"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1000)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	// Apply valid update
	_, err = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "50000.00", Quantity: "2.0"}},
		nil,
		1001, 1001,
	)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	
	// Apply update with gap (firstUpdateID=1010 > seq+1=1002)
	_, err = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "50000.00", Quantity: "3.0"}},
		nil,
		1010, 1012,
	)
	if err != ErrBookSequenceGap {
		t.Errorf("expected ErrBookSequenceGap, got %v", err)
	}
	
	// Verify assembler reset to cold start state
	if assembler.ready {
		t.Error("expected ready=false after gap detection")
	}
	if assembler.seq != 0 {
		t.Errorf("expected seq=0 after reset, got %d", assembler.seq)
	}
	
	// CRITICAL: Verify gap-triggering update was buffered
	if len(assembler.buffer) != 1 {
		t.Errorf("expected 1 buffered update (the gap-triggering one), got %d", len(assembler.buffer))
	}
	if len(assembler.buffer) > 0 && assembler.buffer[0].firstUpdateID != 1010 {
		t.Errorf("expected buffered update U=1010, got U=%d", assembler.buffer[0].firstUpdateID)
	}
	
	// Verify subsequent updates are also buffered
	_, err = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "50000.00", Quantity: "4.0"}},
		nil,
		1013, 1015,
	)
	if err != ErrBookNotInitialized {
		t.Errorf("expected ErrBookNotInitialized after reset, got %v", err)
	}
	if len(assembler.buffer) != 2 {
		t.Errorf("expected 2 buffered updates after reset, got %d", len(assembler.buffer))
	}
}

func TestBookAssembler_GapDetection_MissingUpdate(t *testing.T) {
	// Scenario: seq=1006, then 1009 arrives (1007, 1008 never arrive)
	assembler := NewBookAssembler()
	
	// Initialize with snapshot at seq=1006
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "1.5"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1006)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	// Update 1009 arrives (missing 1007, 1008)
	_, err = assembler.ApplyUpdate(
		[]schema.PriceLevel{{Price: "49999.00", Quantity: "2.0"}},
		nil,
		1009, 1009,
	)
	if err != ErrBookSequenceGap {
		t.Errorf("expected ErrBookSequenceGap for missing updates, got %v", err)
	}
	
	// Verify update 1009 was buffered (not lost!)
	if len(assembler.buffer) != 1 {
		t.Fatalf("expected 1 buffered update (1009), got %d", len(assembler.buffer))
	}
	if assembler.buffer[0].firstUpdateID != 1009 {
		t.Errorf("expected buffered update U=1009, got U=%d", assembler.buffer[0].firstUpdateID)
	}
	if assembler.buffer[0].bids[0].Price != "49999.00" {
		t.Errorf("expected buffered update to preserve data")
	}
	
	// Now apply a new snapshot at seq=1008
	newSnapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "5.0"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "1.0"}},
	}
	
	result, err := assembler.ApplySnapshot(newSnapshot, 1008)
	if err != nil {
		t.Fatalf("apply new snapshot failed: %v", err)
	}
	
	// Verify update 1009 was replayed
	if assembler.seq != 1009 {
		t.Errorf("expected seq=1009 after replay, got %d", assembler.seq)
	}
	
	// Verify the data from update 1009 is in the result
	found := false
	for _, bid := range result.Bids {
		if bid.Price == "49999.00" && bid.Quantity == "2.0" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected replayed update 1009 data to be present in result")
	}
}

func TestBookAssembler_ApplyUpdate_StaleUpdate(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "1.5"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 10)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: "2.0"}}
	asks := []schema.PriceLevel{{Price: "50001.00", Quantity: "1.0"}}
	
	_, err = assembler.ApplyUpdate(bids, asks, 5, 5)
	if err != ErrBookStaleUpdate {
		t.Errorf("expected ErrBookStaleUpdate, got %v", err)
	}
}

func TestBookAssembler_ApplyUpdate_Success(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "50000.00", Quantity: "1.5"},
			{Price: "49999.00", Quantity: "2.0"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50001.00", Quantity: "0.5"},
			{Price: "50002.00", Quantity: "1.0"},
		},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: "2.5"}}
	asks := []schema.PriceLevel{{Price: "50001.00", Quantity: "1.5"}}
	
	result, err := assembler.ApplyUpdate(bids, asks, 2, 2)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	
	if len(result.Bids) == 0 {
		t.Error("expected non-empty bids")
	}
	if len(result.Asks) == 0 {
		t.Error("expected non-empty asks")
	}
	
	if result.Bids[0].Price != "50000.00" {
		t.Errorf("expected top bid 50000.00, got %s", result.Bids[0].Price)
	}
	if result.Bids[0].Quantity != "2.5" {
		t.Errorf("expected top bid quantity 2.5, got %s", result.Bids[0].Quantity)
	}
}

func TestBookAssembler_RemoveLevel_ZeroQuantity(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "50000.00", Quantity: "1.5"},
			{Price: "49999.00", Quantity: "2.0"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50001.00", Quantity: "0.5"},
		},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: "0"}}
	
	result, err := assembler.ApplyUpdate(bids, nil, 2, 2)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	
	for _, bid := range result.Bids {
		if bid.Price == "50000.00" {
			t.Error("level with zero quantity should be removed")
		}
	}
}

func TestBookAssembler_RemoveLevel_EmptyQuantity(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "50000.00", Quantity: "1.5"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50001.00", Quantity: "0.5"},
		},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: ""}}
	
	result, err := assembler.ApplyUpdate(bids, nil, 2, 2)
	if err != nil {
		t.Fatalf("apply update failed: %v", err)
	}
	
	for _, bid := range result.Bids {
		if bid.Price == "50000.00" {
			t.Error("level with empty quantity should be removed")
		}
	}
}

func TestBookAssembler_BidSorting(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "49999.00", Quantity: "2.0"},
			{Price: "50000.00", Quantity: "1.5"},
			{Price: "49998.00", Quantity: "3.0"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50001.00", Quantity: "0.5"},
		},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	if len(result.Bids) < 3 {
		t.Fatalf("expected at least 3 bids, got %d", len(result.Bids))
	}
	
	if result.Bids[0].Price != "50000.00" {
		t.Errorf("expected highest bid first (50000.00), got %s", result.Bids[0].Price)
	}
	if result.Bids[1].Price != "49999.00" {
		t.Errorf("expected second highest bid (49999.00), got %s", result.Bids[1].Price)
	}
	if result.Bids[2].Price != "49998.00" {
		t.Errorf("expected third highest bid (49998.00), got %s", result.Bids[2].Price)
	}
}

func TestBookAssembler_AskSorting(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "50000.00", Quantity: "1.5"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50002.00", Quantity: "1.0"},
			{Price: "50001.00", Quantity: "0.5"},
			{Price: "50003.00", Quantity: "2.0"},
		},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	if len(result.Asks) < 3 {
		t.Fatalf("expected at least 3 asks, got %d", len(result.Asks))
	}
	
	if result.Asks[0].Price != "50001.00" {
		t.Errorf("expected lowest ask first (50001.00), got %s", result.Asks[0].Price)
	}
	if result.Asks[1].Price != "50002.00" {
		t.Errorf("expected second lowest ask (50002.00), got %s", result.Asks[1].Price)
	}
	if result.Asks[2].Price != "50003.00" {
		t.Errorf("expected third lowest ask (50003.00), got %s", result.Asks[2].Price)
	}
}

func TestBookAssembler_DepthLimit(t *testing.T) {
	assembler := NewBookAssembler()
	
	// Create more levels than the output limit
	bids := make([]schema.PriceLevel, 1500)
	for i := 0; i < 1500; i++ {
		bids[i] = schema.PriceLevel{
			Price:    fmt.Sprintf("%d.00", 50000-i),
			Quantity: "1.0",
		}
	}
	
	asks := make([]schema.PriceLevel, 1500)
	for i := 0; i < 1500; i++ {
		asks[i] = schema.PriceLevel{
			Price:    fmt.Sprintf("%d.00", 50001+i),
			Quantity: "1.0",
		}
	}
	
	snapshot := schema.BookSnapshotPayload{
		Bids: bids,
		Asks: asks,
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	if len(result.Bids) != maxOutputDepth {
		t.Errorf("expected %d bids, got %d", maxOutputDepth, len(result.Bids))
	}
	if len(result.Asks) != maxOutputDepth {
		t.Errorf("expected %d asks, got %d", maxOutputDepth, len(result.Asks))
	}
}

func TestBookAssembler_EmptySnapshot(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{},
		Asks: []schema.PriceLevel{},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	if result.Bids != nil && len(result.Bids) > 0 {
		t.Error("expected empty bids")
	}
	if result.Asks != nil && len(result.Asks) > 0 {
		t.Error("expected empty asks")
	}
}

func TestBookAssembler_Concurrent(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{{Price: "50000.00", Quantity: "1.5"}},
		Asks: []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}},
	}
	
	_, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(seq int) {
			bids := []schema.PriceLevel{{Price: "50000.00", Quantity: fmt.Sprintf("%d.0", seq)}}
			_, _ = assembler.ApplyUpdate(bids, nil, uint64(seq+2), uint64(seq+2))
			done <- true
		}(i)
	}
	
	for i := 0; i < 10; i++ {
		<-done
	}
}
