package consumer

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
)

func TestNewLambda(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test-lambda", config, mockDataBus, mockControlBus, mockOrderSubmitter, pm, logger)

	if lambda == nil {
		t.Fatal("expected non-nil lambda")
	}
	if lambda.id != "test-lambda" {
		t.Errorf("expected id 'test-lambda', got %s", lambda.id)
	}
	if lambda.config.Symbol != "BTC-USDT" {
		t.Errorf("expected symbol 'BTC-USDT', got %s", lambda.config.Symbol)
	}
	if lambda.config.Provider != "fake" {
		t.Errorf("expected provider 'fake', got %s", lambda.config.Provider)
	}
}

func TestNewLambdaDefaultID(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "ETH-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	if lambda.id != "lambda-ETH-USDT" {
		t.Errorf("expected default id 'lambda-ETH-USDT', got %s", lambda.id)
	}
}

func TestLambdaStartWithNilBus(t *testing.T) {
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test", config, nil, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil data bus")
	}
}

func TestLambdaStartWithNilControlBus(t *testing.T) {
	mockDataBus := NewMockDataBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test", config, mockDataBus, nil, NewMockOrderSubmitter(), pm, logger)

	ctx := context.Background()
	_, err := lambda.Start(ctx)

	if err == nil {
		t.Error("expected error when starting with nil control bus")
	}
}

func TestLambdaStartAndSubscribe(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test-lambda", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh, err := lambda.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start lambda: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	subCount := mockDataBus.GetActiveSubscriptions()
	if subCount != 5 {
		t.Errorf("expected 5 subscriptions (TRADE, TICKER, BOOK_SNAPSHOT, BOOK_UPDATE, EXEC_REPORT), got %d", subCount)
	}

	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	cancel()
}

func TestLambdaEnableTrading(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	if lambda.tradingActive.Load() {
		t.Error("expected trading to be disabled initially")
	}

	lambda.EnableTrading(true)

	if !lambda.tradingActive.Load() {
		t.Error("expected trading to be enabled after EnableTrading(true)")
	}

	lambda.EnableTrading(false)

	if lambda.tradingActive.Load() {
		t.Error("expected trading to be disabled after EnableTrading(false)")
	}
}

func TestLambdaMatchesSymbol(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"matching symbol", "BTC-USDT", true},
		{"non-matching symbol", "ETH-USDT", false},
		{"empty symbol", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &schema.Event{Symbol: tt.symbol}
			result := lambda.matchesSymbol(evt)
			if result != tt.expected {
				t.Errorf("expected matchesSymbol=%v for symbol %s, got %v", tt.expected, tt.symbol, result)
			}
		})
	}
}

func TestLambdaMatchesProvider(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "fake",
	}

	lambda := NewLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	tests := []struct {
		name     string
		provider string
		expected bool
	}{
		{"matching provider", "fake", true},
		{"non-matching provider", "binance", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evt := &schema.Event{Provider: tt.provider}
			result := lambda.matchesProvider(evt)
			if result != tt.expected {
				t.Errorf("expected matchesProvider=%v for provider %s, got %v", tt.expected, tt.provider, result)
			}
		})
	}
}

func TestLambdaMatchesProviderEmpty(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	pm := pool.NewPoolManager()
	logger := log.New(os.Stdout, "[test] ", log.LstdFlags)

	config := LambdaConfig{
		Symbol:   "BTC-USDT",
		Provider: "",
	}

	lambda := NewLambda("test", config, mockDataBus, mockControlBus, NewMockOrderSubmitter(), pm, logger)

	evt := &schema.Event{Provider: "any-provider"}
	if !lambda.matchesProvider(evt) {
		t.Error("expected empty provider config to match any provider")
	}
}
