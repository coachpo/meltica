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
