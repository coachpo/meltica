package main

import (
	"context"
	"fmt"
	"time"

	"github.com/yourorg/meltica/providers/okx"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	p, _ := okx.New("", "", "")
	defer p.Close()

	sub, err := p.WS().SubscribePublic(ctx, "books5:BTC-USDT")
	if err != nil {
		panic(err)
	}
	for {
		select {
		case m := <-sub.C():
			fmt.Println("okx depth msg bytes:", len(m.Raw))
		case err := <-sub.Err():
			fmt.Println("ws err:", err)
			return
		case <-ctx.Done():
			_ = sub.Close()
			return
		}
	}
}
