package consumer

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/pool"
)

// TradeLambda Tests

func TestNewTradeLambda(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTradeLambda("test-trade", mockBus, pm, logger)

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.id != "test-trade" {
		t.Errorf("expected id 'test-trade', got %s", lambda.id)
	}
}

func TestNewTradeLambdaDefaultID(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTradeLambda("", mockBus, pm, logger)

	if lambda.id != "trade-lambda" {
		t.Errorf("expected default id 'trade-lambda', got %s", lambda.id)
	}
}

func TestTradeLambdaStartWithNilBus(t *testing.T) {
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTradeLambda("test", nil, pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil bus")
	}
}

func TestTradeLambdaStart(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTradeLambda("test-trade", mockBus, pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if errCh == nil {
		t.Fatal("expected non-nil error channel")
	}

	// Give time for subscription
	time.Sleep(50 * time.Millisecond)

	// Should have 1 subscription for TRADE events
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 1 {
		t.Errorf("expected 1 subscription, got %d", subCount)
	}
}

// TickerLambda Tests

func TestNewTickerLambda(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTickerLambda("test-ticker", mockBus, pm, logger)

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.id != "test-ticker" {
		t.Errorf("expected id 'test-ticker', got %s", lambda.id)
	}
}

func TestNewTickerLambdaDefaultID(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTickerLambda("", mockBus, pm, logger)

	if lambda.id != "ticker-lambda" {
		t.Errorf("expected default id 'ticker-lambda', got %s", lambda.id)
	}
}

func TestTickerLambdaStartWithNilBus(t *testing.T) {
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTickerLambda("test", nil, pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil bus")
	}
}

func TestTickerLambdaStart(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewTickerLambda("test-ticker", mockBus, pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if errCh == nil {
		t.Fatal("expected non-nil error channel")
	}

	// Give time for subscription
	time.Sleep(50 * time.Millisecond)

	// Should have 1 subscription for TICKER events
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 1 {
		t.Errorf("expected 1 subscription, got %d", subCount)
	}
}

// OrderBookLambda Tests

func TestNewOrderBookLambda(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewOrderBookLambda("test-book", mockBus, pm, logger)

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.id != "test-book" {
		t.Errorf("expected id 'test-book', got %s", lambda.id)
	}
}

func TestNewOrderBookLambdaDefaultID(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewOrderBookLambda("", mockBus, pm, logger)

	if lambda.id != "orderbook-lambda" {
		t.Errorf("expected default id 'orderbook-lambda', got %s", lambda.id)
	}
}

func TestOrderBookLambdaStartWithNilBus(t *testing.T) {
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewOrderBookLambda("test", nil, pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil bus")
	}
}

func TestOrderBookLambdaStart(t *testing.T) {
	mockBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	lambda := NewOrderBookLambda("test-book", mockBus, pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	if errCh == nil {
		t.Fatal("expected non-nil error channel")
	}

	// Give time for subscriptions
	time.Sleep(50 * time.Millisecond)

	// Should have 2 subscriptions (BOOK_SNAPSHOT and BOOK_UPDATE)
	subCount := mockBus.GetActiveSubscriptions()
	if subCount != 2 {
		t.Errorf("expected 2 subscriptions, got %d", subCount)
	}
}
