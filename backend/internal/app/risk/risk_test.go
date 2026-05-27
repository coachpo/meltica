package risk

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/coachpo/meltica/internal/domain/schema"
)

func TestManager_CheckOrder_Throttle(t *testing.T) {
	limits := Limits{
		MaxPositionSize:  decimal.NewFromInt(1_000),
		MaxNotionalValue: decimal.NewFromInt(1_000_000),
		OrderThrottle:    10,
		OrderBurst:       10,
	}
	manager := NewManager(limits)

	price := "1"
	req := &schema.OrderRequest{
		Provider:      "binance-spot",
		Symbol:        "BTC-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Price:         &price,
		Quantity:      "1",
		ClientOrderID: "ord-0",
	}

	for i := 0; i < 10; i++ {
		req.ClientOrderID = fmt.Sprintf("ord-%d", i)
		if err := manager.CheckOrder(context.Background(), req); err != nil {
			t.Fatalf("order %d should have passed, but got error: %v", i+1, err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req.ClientOrderID = "ord-11"
	if err := manager.CheckOrder(ctx, req); err == nil {
		t.Fatal("11th order should have been throttled, but it was not")
	} else {
		var breach *BreachError
		if !errors.As(err, &breach) {
			t.Fatalf("expected BreachError, got %v", err)
		}
		if breach.Type != BreachTypeRateLimit {
			t.Fatalf("expected breach type %s, got %s", BreachTypeRateLimit, breach.Type)
		}
	}
}

func TestManager_CheckOrder_PositionAndNotionalLimits(t *testing.T) {
	limits := Limits{
		MaxPositionSize:     decimal.NewFromInt(10),
		MaxNotionalValue:    decimal.NewFromInt(50),
		OrderThrottle:       100,
		OrderBurst:          5,
		MaxConcurrentOrders: 0,
	}
	manager := NewManager(limits)

	price := "10"
	firstOrder := &schema.OrderRequest{
		Provider:      "binance-spot",
		Symbol:        "ETH-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Price:         &price,
		Quantity:      "5",
		ClientOrderID: "ord-long",
	}

	if err := manager.CheckOrder(context.Background(), firstOrder); err != nil {
		t.Fatalf("expected initial order to pass: %v", err)
	}

	manager.HandleExecution(firstOrder.Symbol, schema.ExecReportPayload{
		ClientOrderID:  firstOrder.ClientOrderID,
		Side:           schema.TradeSideBuy,
		FilledQuantity: firstOrder.Quantity,
		AvgFillPrice:   *firstOrder.Price,
		State:          schema.ExecReportStateFILLED,
	})

	overPosition := &schema.OrderRequest{
		Provider:      "binance-spot",
		Symbol:        "ETH-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Price:         &price,
		Quantity:      "6",
		ClientOrderID: "ord-limit",
	}
	if err := manager.CheckOrder(context.Background(), overPosition); err == nil {
		t.Fatal("expected position limit breach")
	} else {
		var breach *BreachError
		if !errors.As(err, &breach) {
			t.Fatalf("expected BreachError, got %v", err)
		}
		if breach.Type != BreachTypePositionLimit {
			t.Fatalf("expected breach type %s, got %s", BreachTypePositionLimit, breach.Type)
		}
	}

	highPrice := "40"
	overNotional := &schema.OrderRequest{
		Provider:      "binance-spot",
		Symbol:        "ETH-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Price:         &highPrice,
		Quantity:      "1",
		ClientOrderID: "ord-notional",
	}
	if err := manager.CheckOrder(context.Background(), overNotional); err == nil {
		t.Fatal("expected notional limit breach")
	} else {
		var breach *BreachError
		if !errors.As(err, &breach) {
			t.Fatalf("expected BreachError, got %v", err)
		}
		if breach.Type != BreachTypeNotionalLimit {
			t.Fatalf("expected breach type %s, got %s", BreachTypeNotionalLimit, breach.Type)
		}
	}
}

func TestManager_KillSwitchEngagesAfterBreaches(t *testing.T) {
	limits := Limits{
		MaxPositionSize:   decimal.NewFromInt(5),
		MaxNotionalValue:  decimal.NewFromInt(100),
		OrderThrottle:     100,
		OrderBurst:        2,
		KillSwitchEnabled: true,
		MaxRiskBreaches:   2,
	}
	manager := NewManager(limits)

	price := "10"
	req := &schema.OrderRequest{
		Provider:      "binance-spot",
		Symbol:        "SOL-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderTypeLimit,
		Price:         &price,
		Quantity:      "500",
		ClientOrderID: "ord-risk",
	}

	for i := 0; i < 2; i++ {
		req.ClientOrderID = fmt.Sprintf("ord-risk-%d", i)
		if err := manager.CheckOrder(context.Background(), req); err == nil {
			t.Fatalf("expected risk breach on attempt %d", i+1)
		} else {
			var breach *BreachError
			if !errors.As(err, &breach) {
				t.Fatalf("expected BreachError, got %v", err)
			}
		}
	}

	req.ClientOrderID = "ord-after-breach"
	if err := manager.CheckOrder(context.Background(), req); !errors.Is(err, ErrKillSwitchEngaged) && !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Fatalf("expected kill switch error, got %v", err)
	}
}

func TestManager_AllowedOrderTypesCaseInsensitive(t *testing.T) {
	limits := Limits{
		MaxPositionSize:     decimal.NewFromInt(1_000),
		MaxNotionalValue:    decimal.NewFromInt(1_000_000),
		OrderThrottle:       100,
		OrderBurst:          100,
		PriceBandPercent:    0,
		AllowedOrderTypes:   []schema.OrderType{"Limit", "market", "STOP"},
		KillSwitchEnabled:   false,
		MaxRiskBreaches:     10,
		MaxConcurrentOrders: 10,
	}
	manager := NewManager(limits)

	cases := []schema.OrderType{
		schema.OrderType("limit"),
		schema.OrderType("LiMiT"),
		schema.OrderType("MARKET"),
		schema.OrderType("stop"),
	}

	price := "10"
	for idx, orderType := range cases {
		req := &schema.OrderRequest{
			ClientOrderID: fmt.Sprintf("ord-%d", idx),
			Provider:      "demo",
			Symbol:        "BTC-USDT",
			Side:          schema.TradeSideBuy,
			OrderType:     orderType,
			Price:         &price,
			Quantity:      "1",
		}
		if err := manager.CheckOrder(context.Background(), req); err != nil {
			t.Fatalf("expected order type %s to pass validation: %v", orderType, err)
		}
	}

	req := &schema.OrderRequest{
		ClientOrderID: "ord-invalid",
		Provider:      "demo",
		Symbol:        "BTC-USDT",
		Side:          schema.TradeSideBuy,
		OrderType:     schema.OrderType("iceberg"),
		Price:         &price,
		Quantity:      "1",
	}
	err := manager.CheckOrder(context.Background(), req)
	if err == nil {
		t.Fatal("expected iceberg order type to be rejected")
	}
	var breach *BreachError
	if !errors.As(err, &breach) {
		t.Fatalf("expected breach error, got %v", err)
	}
	if breach.Type != BreachTypeOrderType {
		t.Fatalf("expected breach type %s, got %s", BreachTypeOrderType, breach.Type)
	}
}

func TestParseRiskLimitsValid(t *testing.T) {
	limits, err := ParseLimits(LimitsConfig{
		MaxPositionSize:     " 10.5 ",
		MaxNotionalValue:    "1000.25",
		NotionalCurrency:    " USDT ",
		OrderThrottle:       5,
		OrderBurst:          2,
		MaxConcurrentOrders: 3,
		PriceBandPercent:    1.5,
		AllowedOrderTypes:   []string{" limit", "LIMIT", "Market "},
		KillSwitchEnabled:   true,
		MaxRiskBreaches:     4,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:   true,
			Threshold: 2,
			Cooldown:  "90s",
		},
	})
	if err != nil {
		t.Fatalf("ParseLimits returned error: %v", err)
	}
	if !limits.MaxPositionSize.Equal(decimal.RequireFromString("10.5")) {
		t.Fatalf("unexpected max position size %s", limits.MaxPositionSize)
	}
	if !limits.MaxNotionalValue.Equal(decimal.RequireFromString("1000.25")) {
		t.Fatalf("unexpected max notional value %s", limits.MaxNotionalValue)
	}
	if limits.NotionalCurrency != "USDT" {
		t.Fatalf("unexpected currency %q", limits.NotionalCurrency)
	}
	expectedTypes := []schema.OrderType{"limit", "Market"}
	if fmt.Sprint(limits.AllowedOrderTypes) != fmt.Sprint(expectedTypes) {
		t.Fatalf("expected allowed order types %v, got %v", expectedTypes, limits.AllowedOrderTypes)
	}
	if limits.CircuitBreaker.Cooldown != 90*time.Second {
		t.Fatalf("expected cooldown 90s, got %s", limits.CircuitBreaker.Cooldown)
	}
}

func TestParseRiskLimitsRejectsInvalid(t *testing.T) {
	base := LimitsConfig{
		MaxPositionSize:  "10",
		MaxNotionalValue: "1000",
		NotionalCurrency: "USDT",
		OrderThrottle:    5,
		OrderBurst:       2,
		CircuitBreaker: CircuitBreakerConfig{
			Enabled:   true,
			Threshold: 1,
			Cooldown:  "30s",
		},
	}

	tests := []struct {
		name    string
		mutate  func(*LimitsConfig)
		wantErr string
	}{
		{
			name: "invalid max position decimal",
			mutate: func(cfg *LimitsConfig) {
				cfg.MaxPositionSize = "ten"
			},
			wantErr: "maxPositionSize",
		},
		{
			name: "invalid cooldown duration",
			mutate: func(cfg *LimitsConfig) {
				cfg.CircuitBreaker.Cooldown = "soon"
			},
			wantErr: "circuitBreaker.cooldown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base
			tt.mutate(&cfg)
			_, err := ParseLimits(cfg)
			if err == nil {
				t.Fatal("expected ParseLimits to reject invalid input")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
