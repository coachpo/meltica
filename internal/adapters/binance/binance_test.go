package binance

import (
	"testing"

	"github.com/coachpo/meltica/internal/schema"
)

// Test basic construction

func TestNewParser(t *testing.T) {
	parser := NewParser()
	if parser == nil {
		t.Fatal("expected non-nil parser")
	}
}

func TestNewBookAssembler(t *testing.T) {
	assembler := NewBookAssembler()
	if assembler == nil {
		t.Fatal("expected non-nil book assembler")
	}
}

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5)
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}
}

// Test BookAssembler basic operations

func TestBookAssembler_ApplySnapshot(t *testing.T) {
	assembler := NewBookAssembler()
	
	snapshot := schema.BookSnapshotPayload{
		Bids: []schema.PriceLevel{
			{Price: "50000.00", Quantity: "1.5"},
		},
		Asks: []schema.PriceLevel{
			{Price: "50001.00", Quantity: "0.5"},
		},
	}
	
	result, err := assembler.ApplySnapshot(snapshot, 1)
	if err != nil {
		t.Fatalf("apply snapshot failed: %v", err)
	}
	
	if len(result.Bids) == 0 {
		t.Error("expected non-empty bids")
	}
	if len(result.Asks) == 0 {
		t.Error("expected non-empty asks")
	}
}

// Test RateLimiter

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2)
	
	// Should allow 2 immediate requests
	if !rl.Allow() {
		t.Error("first request should be allowed")
	}
	if !rl.Allow() {
		t.Error("second request should be allowed")
	}
	
	// Third request should be denied
	if rl.Allow() {
		t.Error("third request should be denied")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(1)
	
	// Consume the token
	rl.Allow()
	
	// Should be denied
	if rl.Allow() {
		t.Error("should be denied before reset")
	}
	
	// Reset and try again
	rl.Reset()
	if !rl.Allow() {
		t.Error("should be allowed after reset")
	}
}

// Test RESTFetcher construction

func TestNewRESTFetcher(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test_key", "test_secret", true)
	if fetcher == nil {
		t.Fatal("expected non-nil REST fetcher")
	}
}

// Test WSProvider construction

func TestNewWSProvider(t *testing.T) {
	provider := NewBinanceWSProvider(WSProviderConfig{
		UseTestnet:    true,
		APIKey:        "test_key",
		MaxReconnects: 10,
	})
	if provider == nil {
		t.Fatal("expected non-nil WS provider")
	}
}

// Test error constants

func TestErrorConstants(t *testing.T) {
	if ErrTooManyStreams == nil {
		t.Error("ErrTooManyStreams should not be nil")
	}
	if ErrRateLimitExceeded == nil {
		t.Error("ErrRateLimitExceeded should not be nil")
	}
	if ErrConnectionClosed == nil {
		t.Error("ErrConnectionClosed should not be nil")
	}
	if ErrReconnectFailed == nil {
		t.Error("ErrReconnectFailed should not be nil")
	}
}
