package main

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/providers/binance"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	p, _ := binance.New("", "")
	defer p.Close()

	sub, err := p.WS().SubscribePublic(ctx, "trades:BTC-USDT")
	if err != nil {
		panic(err)
	}
	for {
		select {
		case m := <-sub.C():
			fmt.Println("msg bytes:", len(m.Raw))
		case err := <-sub.Err():
			fmt.Println("ws err:", err)
			return
		case <-ctx.Done():
			_ = sub.Close()
			return
		}
	}
}
