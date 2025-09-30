package main

import (
	"context"
	"fmt"
	"time"

	"github.com/coachpo/meltica/providers/okx"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	p, _ := okx.New("", "", "")
	defer p.Close()

	spot := p.Spot(ctx)
	t, _ := spot.ServerTime(ctx)
	fmt.Println("okx server time:", t)
	syms, _ := spot.Instruments(ctx)
	fmt.Println("okx spot instruments:", len(syms))
}
