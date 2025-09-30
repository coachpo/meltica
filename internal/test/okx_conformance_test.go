package test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/yourorg/meltica/core"
)

func TestConformance_OKX_SpotTime(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	p, err := core.New("okx", struct{ APIKey, Secret, Passphrase string }{os.Getenv("OKX_KEY"), os.Getenv("OKX_SECRET"), os.Getenv("OKX_PASSPHRASE")})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := p.Spot(ctx).ServerTime(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestConformance_OKX_PublicBasics(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	p, err := core.New("okx", struct{ APIKey, Secret, Passphrase string }{"", "", ""})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spot := p.Spot(ctx)
	syms, err := spot.Instruments(ctx)
	if err != nil || len(syms) == 0 {
		t.Fatalf("instruments err=%v count=%d", err, len(syms))
	}
	if _, err := spot.Ticker(ctx, "BTC-USDT"); err != nil {
		t.Fatal(err)
	}
}

func TestConformance_OKX_PrivateOrderFlow(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	key, sec, pass := os.Getenv("OKX_KEY"), os.Getenv("OKX_SECRET"), os.Getenv("OKX_PASSPHRASE")
	if key == "" || sec == "" || pass == "" {
		t.Skip("no OKX creds")
	}
	p, err := core.New("okx", struct{ APIKey, Secret, Passphrase string }{key, sec, pass})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spot := p.Spot(ctx)
	// Dry-run: just ensure signed endpoint reachable by calling get order with nonexistent id
	if _, err := spot.GetOrder(ctx, "BTC-USDT", "0", ""); err != nil {
		t.Logf("signed call error (expected on nonexistent id): %v", err)
	}
}

func TestConformance_OKX_WS_Public(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	p, err := core.New("okx", struct{ APIKey, Secret, Passphrase string }{"", "", ""})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	sub, err := p.WS().SubscribePublic(ctx, "trades:BTC-USDT")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-sub.C():
		fmt.Printf("ws msg bytes=%d\n", len(m.Raw))
	case e := <-sub.Err():
		t.Fatalf("ws error: %v", e)
	case <-ctx.Done():
		t.Fatal("ws no message before timeout")
	}
	_ = sub.Close()

	// Also validate tickers channel quickly
	ctx2, cancel2 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel2()
	sub2, err := p.WS().SubscribePublic(ctx2, "tickers:BTC-USDT")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-sub2.C():
		fmt.Printf("ws tickers msg bytes=%d\n", len(m.Raw))
	case e := <-sub2.Err():
		t.Fatalf("ws error: %v", e)
	case <-ctx2.Done():
		t.Fatal("ws tickers no message before timeout")
	}
	_ = sub2.Close()

	// Depth channel (books5)
	ctx3, cancel3 := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel3()
	sub3, err := p.WS().SubscribePublic(ctx3, "books5:BTC-USDT")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-sub3.C():
		fmt.Printf("ws books5 msg bytes=%d\n", len(m.Raw))
	case e := <-sub3.Err():
		t.Fatalf("ws error: %v", e)
	case <-ctx3.Done():
		t.Fatal("ws books5 no message before timeout")
	}
	_ = sub3.Close()
}
func TestConformance_OKX_WS_Private(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	key, sec, pass := os.Getenv("OKX_KEY"), os.Getenv("OKX_SECRET"), os.Getenv("OKX_PASSPHRASE")
	if key == "" || sec == "" || pass == "" {
		t.Skip("no OKX creds")
	}
	p, err := core.New("okx", struct{ APIKey, Secret, Passphrase string }{key, sec, pass})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sub, err := p.WS().SubscribePrivate(ctx, "orders")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-sub.C():
		if m.Event != "order" {
			t.Fatalf("unexpected event: %s", m.Event)
		}
		if _, ok := m.Parsed.(*core.OrderEvent); !ok {
			t.Fatalf("expected OrderEvent, got %T", m.Parsed)
		}
	case e := <-sub.Err():
		t.Fatalf("ws error: %v", e)
	case <-ctx.Done():
		t.Fatal("no private message before timeout")
	}
	_ = sub.Close()
}

func TestConformance_Binance_WS_Private(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	key, sec := os.Getenv("BINANCE_KEY"), os.Getenv("BINANCE_SECRET")
	if key == "" || sec == "" {
		t.Skip("no binance creds")
	}
	p, err := core.New("binance", struct{ APIKey, Secret string }{key, sec})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	sub, err := p.WS().SubscribePrivate(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case m := <-sub.C():
		if m.Event != "order" {
			t.Fatalf("unexpected event: %s", m.Event)
		}
		if _, ok := m.Parsed.(*core.OrderEvent); !ok {
			t.Fatalf("expected OrderEvent, got %T", m.Parsed)
		}
	case e := <-sub.Err():
		t.Fatalf("ws error: %v", e)
	case <-ctx.Done():
		t.Fatal("no private message before timeout")
	}
	_ = sub.Close()
}
