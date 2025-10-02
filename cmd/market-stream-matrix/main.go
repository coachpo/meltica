package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	_ "github.com/coachpo/meltica/providers/binance"
	_ "github.com/coachpo/meltica/providers/coinbase"
	_ "github.com/coachpo/meltica/providers/kraken"
	_ "github.com/coachpo/meltica/providers/okx"
)

func main() {
	var (
		exchangeFlag = flag.String("exchange", "binance,okx,coinbase,kraken", "Comma-separated list of exchanges to connect to")
		symbolFlag   = flag.String("symbol", "BTC-USDT,BTC-USD,ETH-USDT,ETH-USD", "Comma-separated list of trading symbols in BASE-QUOTE form")
		channelFlag  = flag.String("channel", "ticker,trade,depth", "Comma-separated list of market data streams to subscribe to")
		durationFlag = flag.Duration("duration", 3*time.Second, "Streaming duration for each combination")
	)

	flag.Parse()

	exchanges := normalizeList(*exchangeFlag, strings.ToLower)
	if len(exchanges) == 0 {
		fatal("at least one exchange is required")
	}

	channels := normalizeList(*channelFlag, strings.ToLower)
	if len(channels) == 0 {
		fatal("at least one channel is required")
	}

	rawSymbols := normalizeList(*symbolFlag, func(s string) string {
		return strings.TrimSpace(s)
	})
	var symbols []string
	for _, sym := range rawSymbols {
		normalized := normalizeSymbol(sym)
		if normalized == "" {
			fmt.Fprintf(os.Stderr, "warning: skipping invalid symbol %q\n", sym)
			continue
		}
		symbols = append(symbols, normalized)
	}
	if len(symbols) == 0 {
		fatal("at least one valid symbol is required")
	}

	duration := *durationFlag
	if duration <= 0 {
		fatal("duration must be positive")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	for _, exchange := range exchanges {
		for _, symbol := range symbols {
			for _, channel := range channels {
				if ctx.Err() != nil {
					return
				}

				fmt.Printf("\n=== exchange=%s symbol=%s channel=%s ===\n", exchange, symbol, channel)
				if err := streamCombination(ctx, exchange, symbol, channel, duration); err != nil {
					if errors.Is(err, context.Canceled) {
						return
					}
					fmt.Fprintf(os.Stderr, "error: %v\n", err)
				}
			}
		}
	}
}

func streamCombination(parent context.Context, exchange, symbol, channel string, duration time.Duration) error {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	provider, err := core.New(exchange, nil)
	if err != nil {
		return fmt.Errorf("create provider %s: %w", exchange, err)
	}
	defer provider.Close()

	if !provider.Capabilities().Has(core.CapabilityWebsocketPublic) {
		return fmt.Errorf("provider %s does not support public websocket market data", provider.Name())
	}

	topic, err := resolveTopic(provider.Name(), channel, symbol)
	if err != nil {
		return err
	}

	sub, err := provider.WS().SubscribePublic(ctx, topic)
	if err != nil {
		return fmt.Errorf("subscribe: %w", err)
	}
	defer sub.Close()

	fmt.Printf("Streaming %s from %s (%s)\n", topic, provider.Name(), strings.ToUpper(exchange))

	timer := time.NewTimer(duration)
	defer timer.Stop()

	for {
		select {
		case <-parent.Done():
			return parent.Err()
		case <-timer.C:
			fmt.Printf("Completed after %s\n", duration)
			return nil
		case err := <-sub.Err():
			if err != nil {
				return fmt.Errorf("stream error: %w", err)
			}
			return nil
		case msg, ok := <-sub.C():
			if !ok {
				return nil
			}
			if out, ok := formatEventMessage(msg); ok {
				fmt.Println(out)
				continue
			}
			if err := handleMetaMessage(msg); err != nil {
				return err
			}
		}
	}
}

func normalizeList(raw string, norm func(string) string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		normalized := norm(trimmed)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	return result
}

func resolveTopic(providerName, channel, symbol string) (string, error) {
	if strings.Contains(channel, ":") {
		return channel, nil
	}
	ch := strings.ToLower(channel)
	if ch == "" {
		return "", errors.New("channel is required")
	}
	symbol = normalizeSymbol(symbol)
	if symbol == "" {
		return "", errors.New("symbol must be in BASE-QUOTE form")
	}

	switch strings.ToLower(providerName) {
	case "binance":
		switch ch {
		case "trade":
			return corews.TopicTrade + ":" + symbol, nil
		case "ticker":
			return corews.TopicTicker + ":" + symbol, nil
		case "depth":
			return corews.TopicBook + ":" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "okx":
		switch ch {
		case "trade":
			return corews.TopicTrade + ":" + symbol, nil
		case "ticker":
			return corews.TopicTicker + ":" + symbol, nil
		case "depth":
			return corews.TopicBook + ":" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "coinbase":
		switch ch {
		case "trade":
			return corews.TopicTrade + ":" + symbol, nil
		case "ticker":
			return corews.TopicTicker + ":" + symbol, nil
		case "depth":
			return corews.TopicBook + ":" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "kraken":
		switch ch {
		case "trade":
			return corews.TopicTrade + ":" + symbol, nil
		case "ticker":
			return corews.TopicTicker + ":" + symbol, nil
		case "depth":
			return corews.TopicBook + ":" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	default:
		return ch + ":" + symbol, nil
	}
}

func normalizeSymbol(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "_", "-")
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "-") {
		return ""
	}
	return s
}

func formatEventMessage(msg core.Message) (string, bool) {
	ts := msg.At.UTC().Format(time.RFC3339)
	if msg.Event == "" || msg.Parsed == nil {
		return "", false
	}
	switch evt := msg.Parsed.(type) {
	case *corews.TradeEvent:
		return fmt.Sprintf("[%s] %s trade price=%s qty=%s", ts, evt.Symbol, formatRat(evt.Price), formatRat(evt.Quantity)), true
	case *corews.TickerEvent:
		return fmt.Sprintf("[%s] %s ticker bid=%s ask=%s", ts, evt.Symbol, formatRat(evt.Bid), formatRat(evt.Ask)), true
	case *corews.DepthEvent:
		bid := "—"
		ask := "—"
		if len(evt.Bids) > 0 {
			bid = fmt.Sprintf("%s@%s", formatRat(evt.Bids[0].Price), formatRat(evt.Bids[0].Qty))
		}
		if len(evt.Asks) > 0 {
			ask = fmt.Sprintf("%s@%s", formatRat(evt.Asks[0].Price), formatRat(evt.Asks[0].Qty))
		}
		return fmt.Sprintf("[%s] %s depth bid=%s ask=%s", ts, evt.Symbol, bid, ask), true
	case *corews.OrderEvent:
		return fmt.Sprintf("[%s] %s order id=%s status=%s filled=%s avg=%s", ts, evt.Symbol, evt.OrderID, evt.Status, formatRat(evt.FilledQty), formatRat(evt.AvgPrice)), true
	case *corews.BalanceEvent:
		return fmt.Sprintf("[%s] balance update assets=%d", ts, len(evt.Balances)), true
	default:
		return "", false
	}
}

func handleMetaMessage(msg core.Message) error {
	topic := strings.ToLower(msg.Topic)
	switch topic {
	case "", "heartbeat":
		return nil
	case "systemstatus":
		if len(msg.Raw) == 0 {
			return nil
		}
		var env struct {
			Status       string `json:"status"`
			Version      string `json:"version"`
			ConnectionID any    `json:"connectionID"`
		}
		if err := json.Unmarshal(msg.Raw, &env); err != nil {
			return nil
		}
		fmt.Fprintf(os.Stderr, "[%s] system status=%s version=%s connection=%v\n", msg.At.UTC().Format(time.RFC3339), strings.ToLower(env.Status), env.Version, env.ConnectionID)
		return nil
	case "subscriptionstatus":
		if len(msg.Raw) == 0 {
			return nil
		}
		var env struct {
			Status       string `json:"status"`
			ErrorMessage string `json:"errorMessage"`
			Pair         string `json:"pair"`
		}
		if err := json.Unmarshal(msg.Raw, &env); err != nil {
			return nil
		}
		if strings.EqualFold(env.Status, "error") {
			return fmt.Errorf("subscription error for %s: %s", env.Pair, env.ErrorMessage)
		}
		fmt.Fprintf(os.Stderr, "[%s] subscription %s status=%s\n", msg.At.UTC().Format(time.RFC3339), env.Pair, strings.ToLower(env.Status))
		return nil
	default:
		if len(msg.Raw) == 0 {
			return nil
		}
		raw := string(msg.Raw)
		if len(raw) > 256 {
			raw = raw[:256] + "…"
		}
		fmt.Fprintf(os.Stderr, "[%s] ignored meta topic=%s payload=%s\n", msg.At.UTC().Format(time.RFC3339), topic, raw)
		return nil
	}
}

func formatRat(r *big.Rat) string {
	if r == nil {
		return ""
	}
	return r.FloatString(8)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
