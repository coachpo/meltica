package main

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/providers/binance"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, _ := binance.New("", "")
	defer p.Close()

	spot := p.Spot(ctx)
	t, _ := spot.ServerTime(ctx)
	fmt.Println("server time:", t)
	syms, _ := spot.Instruments(ctx)
	fmt.Println("instruments count:", len(syms))
	tk, _ := spot.Ticker(ctx, "BTC-USDT")
	fmt.Println("ticker symbol:", tk.Symbol)
}
