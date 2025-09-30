package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yourorg/meltica/providers/binance"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, _ := binance.New("", "")
	defer p.Close()

	lin := p.LinearFutures(ctx)
	syms, _ := lin.Instruments(ctx)
	fmt.Println("linear futures instruments:", len(syms))
	tk, _ := lin.Ticker(ctx, "BTC-USDT")
	fmt.Println("ticker symbol:", tk.Symbol)
}
