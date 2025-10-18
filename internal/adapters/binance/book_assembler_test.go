package binance

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coachpo/meltica/internal/schema"
)

func TestBookAssembler_ApplyUpdate_NotInitialized(t *testing.T) {
	assembler := NewBookAssembler()
	
	bids := []schema.PriceLevel{{Price: "50000.00", Quantity: "1.0"}}
	asks := []schema.PriceLevel{{Price: "50001.00", Quantity: "0.5"}}
	
	_, err := assembler.ApplyUpdate(bids, asks, "", 1, 1)
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
	
	_, err = assembler.ApplyUpdate(bids, asks, "", 5, 5)
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
	
	result, err := assembler.ApplyUpdate(bids, asks, "", 2, 2)
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
	
	result, err := assembler.ApplyUpdate(bids, nil, "", 2, 2)
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
	
	result, err := assembler.ApplyUpdate(bids, nil, "", 2, 2)
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
	
	bids := make([]schema.PriceLevel, 20)
	for i := 0; i < 20; i++ {
		bids[i] = schema.PriceLevel{
			Price:    fmt.Sprintf("%d.00", 50000-i),
			Quantity: "1.0",
		}
	}
	
	asks := make([]schema.PriceLevel, 20)
	for i := 0; i < 20; i++ {
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
	
	if len(result.Bids) != bookDepthChecksum {
		t.Errorf("expected %d bids, got %d", bookDepthChecksum, len(result.Bids))
	}
	if len(result.Asks) != bookDepthChecksum {
		t.Errorf("expected %d asks, got %d", bookDepthChecksum, len(result.Asks))
	}
}

func TestNormaliseDecimal(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"50000.00", "50000"},
		{"50000.10", "50000.1"},
		{"50000.12300", "50000.123"},
		{"50000", "50000"},
		{"+50000.00", "50000"},
		{"0.00", "0"},
		{"  50000.00  ", "50000"},
		{"", ""},
		{"0.000", "0"},
		{"-0.00", "-0"},
	}
	
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normaliseDecimal(tt.input)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestVerifyChecksumValue_Valid(t *testing.T) {
	err := verifyChecksumValue("12345", 12345)
	if err != nil {
		t.Errorf("expected no error for valid checksum, got %v", err)
	}
}

func TestVerifyChecksumValue_Invalid(t *testing.T) {
	err := verifyChecksumValue("12345", 54321)
	if err == nil {
		t.Error("expected error for invalid checksum")
	}
	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected 'checksum mismatch' error, got: %v", err)
	}
}

func TestVerifyChecksumValue_ParseError(t *testing.T) {
	err := verifyChecksumValue("invalid", 12345)
	if err == nil {
		t.Error("expected error for invalid checksum format")
	}
	if !strings.Contains(err.Error(), "parse checksum") {
		t.Errorf("expected 'parse checksum' error, got: %v", err)
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
			_, _ = assembler.ApplyUpdate(bids, nil, "", uint64(seq+2), uint64(seq+2))
			done <- true
		}(i)
	}
	
	for i := 0; i < 10; i++ {
		<-done
	}
}
