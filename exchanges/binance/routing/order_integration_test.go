package routing_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/exchanges/bootstrap"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/binance"
)

func TestSandboxOrderLifecycle(t *testing.T) {
	apiKey := strings.TrimSpace(os.Getenv("BINANCE_SANDBOX_API_KEY"))
	apiSecret := strings.TrimSpace(os.Getenv("BINANCE_SANDBOX_API_SECRET"))
	spotBase := strings.TrimSpace(os.Getenv("BINANCE_SANDBOX_SPOT_BASE_URL"))
	linearBase := strings.TrimSpace(os.Getenv("BINANCE_SANDBOX_LINEAR_BASE_URL"))
	inverseBase := strings.TrimSpace(os.Getenv("BINANCE_SANDBOX_INVERSE_BASE_URL"))

	if apiKey == "" || apiSecret == "" {
		t.Skip("BINANCE_SANDBOX_API_KEY/SECRET must be set with spot sandbox credentials")
	}
	if spotBase == "" {
		t.Skip("BINANCE_SANDBOX_SPOT_BASE_URL must be set to the Binance spot sandbox endpoint")
	}

	opts := []binance.Option{
		bootstrap.WithConfig(
			config.WithBinanceRESTEndpoints(spotBase, linearBase, inverseBase),
			config.WithBinanceHTTPTimeout(10*time.Second),
		),
	}

	ex, err := binance.New(apiKey, apiSecret, opts...)
	if err != nil {
		t.Fatalf("binance.New: %v", err)
	}
	t.Cleanup(func() { _ = ex.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	spot := ex.Spot(ctx)
	orderReq := core.OrderRequest{
		Symbol:      "BTC-USDT",
		Side:        core.SideBuy,
		Type:        core.TypeLimit,
		TimeInForce: core.GTC,
		Quantity:    big.NewRat(1, 1000), // 0.001 BTC
		Price:       big.NewRat(10, 1),   // $10 bid to avoid immediate fills
		ClientID:    fmt.Sprintf("it-%d", time.Now().UnixNano()),
	}

	placed, err := spot.PlaceOrder(ctx, orderReq)
	if err != nil {
		var e *errs.E
		if errors.As(err, &e) && e.Canonical == errs.CanonicalInsufficientBalance {
			t.Skip("sandbox account lacks funds; top up testnet balances and retry")
		}
		if errors.As(err, &e) && e.Canonical == errs.CanonicalRateLimited {
			t.Skipf("rate limited by sandbox API: %v", err)
		}
		t.Fatalf("PlaceOrder: %v", err)
	}
	if placed.ID == "" {
		t.Fatalf("expected order id, got %+v", placed)
	}

	queried, err := spot.GetOrder(ctx, orderReq.Symbol, placed.ID, orderReq.ClientID)
	if err != nil {
		t.Fatalf("GetOrder: %v", err)
	}
	if queried.ID != placed.ID {
		t.Fatalf("queried order id mismatch: want %s got %s", placed.ID, queried.ID)
	}

	if err := spot.CancelOrder(ctx, orderReq.Symbol, placed.ID, orderReq.ClientID); err != nil {
		t.Fatalf("CancelOrder: %v", err)
	}
}
