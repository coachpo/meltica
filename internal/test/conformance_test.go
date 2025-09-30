package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/yourorg/meltica/core"
)

func TestConformance_SpotTime(t *testing.T) {
	if os.Getenv("MELTICA_CONFORMANCE") == "" {
		t.Skip("set MELTICA_CONFORMANCE=1 to run")
	}
	p, err := core.New("binance", struct{ APIKey, Secret string }{os.Getenv("BINANCE_KEY"), os.Getenv("BINANCE_SECRET")})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	spot := p.Spot(ctx)
	if _, err := spot.ServerTime(ctx); err != nil {
		t.Fatal(err)
	}
}
