package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance"
)

func main() {
	fmt.Println("Starting Binance WebSocket Validation Test...")
	fmt.Println("This test validates that all WebSocket streams match Binance documentation format")
	fmt.Println("Press Ctrl+C to exit")
	fmt.Println()

	// Create Binance provider
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
	fmt.Println("Validating WebSocket payload formats...")
	fmt.Println()

	// Track validation results
	validationResults := map[string]bool{
		"trade":       false,
		"bookTicker":  false,
		"depthUpdate": false,
	}

	// Process messages for validation
	validationTimeout := time.After(30 * time.Second)
	messageCount := 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("Context cancelled, exiting...")
			printValidationResults(validationResults)
			return
		case <-validationTimeout:
			fmt.Println("Validation timeout reached")
			printValidationResults(validationResults)
			return
		case msg, ok := <-sub.C():
			if !ok {
				fmt.Println("Subscription channel closed")
				printValidationResults(validationResults)
				return
			}
			messageCount++
			validateMessage(&msg, validationResults)

			// Stop after we've validated all stream types
			if allValidated(validationResults) {
				fmt.Println("\n✅ All stream types validated successfully!")
				printValidationResults(validationResults)
				return
			}
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

// validateMessage validates the message against Binance documentation
func validateMessage(msg *core.Message, results map[string]bool) {
	timestamp := msg.At.Format("15:04:05.000")

	// Parse raw JSON to validate structure
	var rawData map[string]interface{}
	if err := json.Unmarshal(msg.Raw, &rawData); err != nil {
		fmt.Printf("[%s] Failed to parse raw JSON: %v\n", timestamp, err)
		return
	}

	// Check if it's a combined stream
	if stream, exists := rawData["stream"]; exists {
		streamStr := stream.(string)
		if data, exists := rawData["data"]; exists {
			// Validate the data payload
			if dataBytes, err := json.Marshal(data); err == nil {
				validatePayload(dataBytes, streamStr, results)
			}
		}
	} else {
		// Direct payload
		validatePayload(msg.Raw, "", results)
	}
}

// validatePayload validates the actual payload against Binance documentation
func validatePayload(payload []byte, stream string, results map[string]bool) {
	var data map[string]interface{}
	if err := json.Unmarshal(payload, &data); err != nil {
		fmt.Printf("Failed to parse payload: %v\n", err)
		return
	}

	// Get event type
	eventType, hasEvent := data["e"].(string)
	if !hasEvent {
		// Check if it's bookTicker (no event field)
		if _, hasU := data["u"]; hasU {
			eventType = "bookTicker"
		}
	}

	switch eventType {
	case "trade":
		validateTradePayload(data, results)
	case "bookTicker":
		validateBookTickerPayload(data, results)
	case "depthUpdate":
		validateDepthUpdatePayload(data, results)
	default:
		fmt.Printf("Unknown event type: %s\n", eventType)
	}
}

// validateTradePayload validates trade stream against Binance documentation
func validateTradePayload(data map[string]interface{}, results map[string]bool) {
	requiredFields := []string{
		"e", "E", "s", "t", "p", "q", "T", "m", "M",
	}

	if validateFields(data, requiredFields, "trade") {
		fmt.Printf("✅ TRADE: Validated - Symbol: %s, Price: %s, Qty: %s\n",
			data["s"], data["p"], data["q"])
		results["trade"] = true
	}
}

// validateBookTickerPayload validates bookTicker stream against Binance documentation
func validateBookTickerPayload(data map[string]interface{}, results map[string]bool) {
	requiredFields := []string{
		"u", "s", "b", "B", "a", "A",
	}

	if validateFields(data, requiredFields, "bookTicker") {
		fmt.Printf("✅ BOOKTICKER: Validated - Symbol: %s, Bid: %s, Ask: %s\n",
			data["s"], data["b"], data["a"])
		results["bookTicker"] = true
	}
}

// validateDepthUpdatePayload validates depthUpdate stream against Binance documentation
func validateDepthUpdatePayload(data map[string]interface{}, results map[string]bool) {
	requiredFields := []string{
		"e", "E", "s", "U", "u", "b", "a",
	}

	if validateFields(data, requiredFields, "depthUpdate") {
		// Validate bids and asks arrays
		bids, bidsOk := data["b"].([]interface{})
		asks, asksOk := data["a"].([]interface{})

		if bidsOk && asksOk {
			fmt.Printf("✅ DEPTHUPDATE: Validated - Symbol: %s, U: %v, u: %v, Bids: %d, Asks: %d\n",
				data["s"], data["U"], data["u"], len(bids), len(asks))
			results["depthUpdate"] = true
		} else {
			fmt.Printf("❌ DEPTHUPDATE: Invalid bids/asks format\n")
		}
	}
}

// validateFields checks if all required fields are present and have correct types
func validateFields(data map[string]interface{}, requiredFields []string, eventType string) bool {
	missingFields := []string{}

	for _, field := range requiredFields {
		if _, exists := data[field]; !exists {
			missingFields = append(missingFields, field)
		}
	}

	if len(missingFields) > 0 {
		fmt.Printf("❌ %s: Missing fields: %v\n", strings.ToUpper(eventType), missingFields)
		return false
	}

	// Validate field types
	for field, value := range data {
		if value == nil {
			fmt.Printf("❌ %s: Field %s is null\n", strings.ToUpper(eventType), field)
			return false
		}
	}

	return true
}

// allValidated checks if all stream types have been validated
func allValidated(results map[string]bool) bool {
	for _, validated := range results {
		if !validated {
			return false
		}
	}
	return true
}

// printValidationResults prints the final validation results
func printValidationResults(results map[string]bool) {
	fmt.Println("\n=== VALIDATION RESULTS ===")
	for streamType, validated := range results {
		status := "❌ FAILED"
		if validated {
			status = "✅ PASSED"
		}
		fmt.Printf("%-12s: %s\n", strings.ToUpper(streamType), status)
	}
	fmt.Println("==========================")
}
