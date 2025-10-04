package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"time"

	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance"
)

const (
	nativeSymbol    = "BTCUSDT"
	canonicalSymbol = "BTC-USDT"
	displayLevels   = 30
	refreshInterval = 10 * time.Millisecond
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")
	fmt.Printf("Monitoring %s depth via provider pipelines...\n", nativeSymbol)

	provider, err := binance.New("", "")
	if err != nil {
		log.Fatalf("failed to create Binance provider: %v", err)
	}
	defer provider.Close()

	snapshots, errs, err := provider.OrderBookSnapshots(ctx, canonicalSymbol)
	if err != nil {
		log.Fatalf("failed to start order book stream: %v", err)
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	var latest corews.BookEvent
	var hasSnapshot bool
	var lastRender time.Time

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Stopping order book monitor...")
			return
		case err := <-errs:
			if err != nil {
				log.Fatalf("order book stream error: %v", err)
			}
			return
		case snap, ok := <-snapshots:
			if !ok {
				fmt.Println("Order book stream ended")
				return
			}
			latest = snap
			hasSnapshot = true
			renderSnapshot(latest)
			lastRender = time.Now()
		case <-ticker.C:
			if hasSnapshot && time.Since(lastRender) >= refreshInterval {
				renderSnapshot(latest)
				lastRender = time.Now()
			}
		}
	}
}

func renderSnapshot(event corews.BookEvent) {
	bids := topLevels(event.Bids, displayLevels, true)
	asks := topLevels(event.Asks, displayLevels, false)

	fmt.Print("\033[2J\033[H")
	fmt.Printf("Binance %s Depth | Update at %s | Bids %d | Asks %d\n",
		nativeSymbol, event.Time.Format(time.RFC3339Nano), len(event.Bids), len(event.Asks))
	fmt.Printf("%-18s %-18s | %-18s %-18s\n", "BidQty", "BidPrice", "AskPrice", "AskQty")

	rows := displayLevels
	if len(bids) > rows {
		rows = len(bids)
	}
	if len(asks) > rows {
		rows = len(asks)
	}

	for i := 0; i < rows; i++ {
		bidQty, bidPrice := "-", "-"
		askPrice, askQty := "-", "-"
		if i < len(bids) {
			bidQty = formatRat(bids[i].qty, 6)
			bidPrice = formatRat(bids[i].price, 2)
		}
		if i < len(asks) {
			askPrice = formatRat(asks[i].price, 2)
			askQty = formatRat(asks[i].qty, 6)
		}
		fmt.Printf("%-18s %-18s | %-18s %-18s\n", bidQty, bidPrice, askPrice, askQty)
	}
}

type bookLevel struct {
	price *big.Rat
	qty   *big.Rat
}

func topLevels(levels []corews.DepthLevel, limit int, desc bool) []bookLevel {
	filtered := make([]bookLevel, 0, len(levels))
	for _, level := range levels {
		if level.Price == nil || level.Qty == nil || level.Qty.Sign() == 0 {
			continue
		}
		filtered = append(filtered, bookLevel{
			price: new(big.Rat).Set(level.Price),
			qty:   new(big.Rat).Set(level.Qty),
		})
	}
	sort.Slice(filtered, func(i, j int) bool {
		cmp := filtered[i].price.Cmp(filtered[j].price)
		if desc {
			return cmp > 0
		}
		return cmp < 0
	})
	if len(filtered) > limit {
		filtered = filtered[:limit]
	}
	return filtered
}

func formatRat(r *big.Rat, precision int) string {
	if r == nil {
		return "0"
	}
	return r.FloatString(precision)
}
