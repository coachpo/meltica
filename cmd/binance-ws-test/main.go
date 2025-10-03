package main

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance"
)

func main() {
	fmt.Println("Starting Binance WebSocket Test Client...")
	fmt.Println("Supported symbols: BTC-USDT")
	fmt.Println("Supported topics: trade, ticker, book")
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	// Create Binance provider (no API key needed for public streams)
	provider, err := binance.New("", "")
	if err != nil {
		log.Fatalf("Failed to create Binance provider: %v", err)
	}

	// Get WebSocket handler
	ws := provider.WS()

	// Define topics to subscribe to
	topics := []string{
		corews.TradeTopic("BTC-USDT"),
		corews.TickerTopic("BTC-USDT"),
		corews.BookTopic("BTC-USDT"),
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal, closing...")
		cancel()
	}()

	// Subscribe to public streams
	sub, err := ws.SubscribePublic(ctx, topics...)
	if err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}
	defer sub.Close()

	fmt.Printf("Subscribed to topics: %v\n", topics)
	fmt.Println("Waiting for messages...")
	fmt.Println()

	// Process messages
	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, exiting...")
			return
		case msg, ok := <-sub.C():
			if !ok {
				fmt.Println("Subscription channel closed")
				return
			}
			printMessage(&msg)
		case err, ok := <-sub.Err():
			if !ok {
				fmt.Println("Error channel closed")
				return
			}
			fmt.Printf("Error: %v\n", err)
			return
		}
	}
}

// printMessage formats and prints different types of messages
func printMessage(msg *core.Message) {
	timestamp := msg.At.Format("15:04:05.000")

	switch msg.Event {
	case corews.TopicTrade:
		if trade, ok := msg.Parsed.(*corews.TradeEvent); ok {
			fmt.Printf("[%s] TRADE %s: Price=%s, Qty=%s\n",
				timestamp, trade.Symbol,
				formatRat(trade.Price), formatRat(trade.Quantity))
		}
	case corews.TopicTicker:
		if ticker, ok := msg.Parsed.(*corews.TickerEvent); ok {
			fmt.Printf("[%s] TICKER %s: Bid=%s, Ask=%s\n",
				timestamp, ticker.Symbol,
				formatRat(ticker.Bid), formatRat(ticker.Ask))
		}
	case corews.TopicBook:
		if book, ok := msg.Parsed.(*corews.BookEvent); ok {
			fmt.Printf("[%s] BOOK %s: Bids=%d, Asks=%d\n",
				timestamp, book.Symbol, len(book.Bids), len(book.Asks))
			// Print top 3 bids and asks
			if len(book.Bids) > 0 {
				fmt.Printf("  Top Bid: %s @ %s\n",
					formatRat(book.Bids[0].Qty), formatRat(book.Bids[0].Price))
			}
			if len(book.Asks) > 0 {
				fmt.Printf("  Top Ask: %s @ %s\n",
					formatRat(book.Asks[0].Qty), formatRat(book.Asks[0].Price))
			}
		}
	default:
		// Print raw message for unknown event types
		fmt.Printf("[%s] UNKNOWN %s: %s\n",
			timestamp, msg.Topic, string(msg.Raw))
	}
}

// formatRat formats a rational number for display
func formatRat(r *big.Rat) string {
	if r == nil {
		return "nil"
	}
	// Convert to float64 for display
	f, _ := r.Float64()
	return fmt.Sprintf("%.8f", f)
}
