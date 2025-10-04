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

	"github.com/coachpo/meltica/core"
	coreexchange "github.com/coachpo/meltica/core/exchange"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/exchanges/binance"
)

const (
	nativeSymbol    = "BTCUSDT"
	canonicalSymbol = "BTC-USDT"
	displayLevels   = 20
	refreshInterval = 10 * time.Millisecond
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")
	fmt.Printf("Monitoring %s depth via exchange pipelines...\n", nativeSymbol)

	exchange, err := binance.New("", "")
	if err != nil {
		log.Fatalf("failed to create Binance exchange: %v", err)
	}
	defer exchange.Close()

	validateRESTLayer(ctx, exchange)
	validateWSRouting(ctx, exchange)

	snapshots, errs, err := exchange.OrderBookSnapshots(ctx, canonicalSymbol)
	if err != nil {
		log.Fatalf("failed to start order book stream: %v", err)
	}

	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	var latest coreexchange.BookEvent
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

func validateRESTLayer(ctx context.Context, exchange *binance.Exchange) {
	const snapshotDepth = 100
	snapshot, updateID, err := exchange.DepthSnapshot(ctx, canonicalSymbol, snapshotDepth)
	if err != nil {
		log.Printf("REST validation failed: %v", err)
		return
	}
	fmt.Printf("REST layer OK: snapshot %d levels fetched (LastUpdateID=%d)\n", len(snapshot.Bids)+len(snapshot.Asks), updateID)
	topBid := bestLevel(snapshot.Bids, true)
	topAsk := bestLevel(snapshot.Asks, false)
	fmt.Printf("  Top of book via REST -> Bid %s / %s, Ask %s / %s\n",
		formatRat(topBid.qty, 6), formatRat(topBid.price, 2),
		formatRat(topAsk.price, 2), formatRat(topAsk.qty, 6))
}

func validateWSRouting(ctx context.Context, exchange *binance.Exchange) {
	ws := exchange.WS()
	sub, err := ws.SubscribePublic(ctx, corews.BookTopic(canonicalSymbol))
	if err != nil {
		log.Printf("WS routing validation failed: %v", err)
		return
	}
	go func() {
		defer sub.Close()
		eventsValidated := 0
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				if err != nil {
					log.Printf("WS routing error: %v", err)
				}
				return
			case msg, ok := <-sub.C():
				if !ok {
					return
				}
				if book, ok := msg.Parsed.(*coreexchange.BookEvent); ok {
					fmt.Printf("WS routing OK: received depth delta with %d/%d levels\n", len(book.Bids), len(book.Asks))
					eventsValidated++
				}
				if tickerEvt, ok := msg.Parsed.(*coreexchange.TickerEvent); ok {
					fmt.Printf("WS routing OK: ticker update bid %s ask %s\n", formatRat(tickerEvt.Bid, 2), formatRat(tickerEvt.Ask, 2))
					eventsValidated++
				}
				if eventsValidated >= 2 {
					return
				}
			}
		}
	}()
}

func renderSnapshot(event coreexchange.BookEvent) {
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

func topLevels(levels []core.BookDepthLevel, limit int, desc bool) []bookLevel {
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

func bestLevel(levels []core.BookDepthLevel, desc bool) bookLevel {
	filtered := topLevels(levels, 1, desc)
	if len(filtered) == 0 {
		return bookLevel{}
	}
	return filtered[0]
}
