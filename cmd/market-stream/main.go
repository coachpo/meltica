package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/coachpo/meltica/core"
	_ "github.com/coachpo/meltica/providers/binance"
	_ "github.com/coachpo/meltica/providers/coinbase"
	_ "github.com/coachpo/meltica/providers/kraken"
	_ "github.com/coachpo/meltica/providers/okx"
)

func main() {
	var (
		exchangeFlag    = flag.String("exchange", "binance", "Exchange to connect to (binance, okx, coinbase, kraken)")
		symbolFlag      = flag.String("symbol", "BTC-USDT", "Trading symbol in canonical BASE-QUOTE form")
		channelFlag     = flag.String("channel", "ticker", "Market data stream to subscribe to (ticker, trades, depth)")
		interactiveFlag = flag.Bool("interactive", true, "Launch interactive guide to confirm exchange, symbol, and channel")
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: market-stream [flags]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Continuously stream public market data via meltica.\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	interactive := *interactiveFlag
	if stat, err := os.Stdin.Stat(); err == nil {
		if stat.Mode()&os.ModeCharDevice == 0 {
			interactive = false
		}
	}

	exchangeInput := strings.TrimSpace(*exchangeFlag)
	symbolInput := strings.TrimSpace(*symbolFlag)
	channelInput := strings.TrimSpace(*channelFlag)

	if interactive {
		var err error
		exchangeInput, symbolInput, channelInput, err = runInteractiveGuide(exchangeInput, symbolInput, channelInput)
		if err != nil {
			fatal("interactive guide: %v", err)
		}
	}

	exchange := strings.ToLower(strings.TrimSpace(exchangeInput))
	if exchange == "" {
		fatal("exchange is required")
	}

	symbol := normalizeSymbol(symbolInput)
	if symbol == "" {
		fatal("symbol must be in BASE-QUOTE form, e.g. BTC-USDT")
	}

	channel := strings.TrimSpace(channelInput)
	if channel == "" {
		fatal("channel is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	provider, err := core.New(exchange, nil)
	if err != nil {
		fatal("create provider: %v", err)
	}
	defer provider.Close()

	if !provider.Capabilities().Has(core.CapabilityWebsocketPublic) {
		fatal("provider %s does not support public websocket market data", provider.Name())
	}

	topic, err := resolveTopic(provider.Name(), channel, symbol)
	if err != nil {
		fatal("%v", err)
	}

	sub, err := provider.WS().SubscribePublic(ctx, topic)
	if err != nil {
		fatal("subscribe: %v", err)
	}
	defer sub.Close()

	fmt.Printf("Streaming %s from %s (%s)\n", topic, provider.Name(), strings.ToUpper(exchange))

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-sub.Err():
			if err != nil {
				fatal("stream error: %v", err)
			}
			return
		case msg, ok := <-sub.C():
			if !ok {
				return
			}
			if out, ok := formatEventMessage(msg); ok {
				fmt.Println(out)
				continue
			}
			if err := handleMetaMessage(msg); err != nil {
				fatal("%v", err)
			}
		}
	}
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
		case "trade", "trades":
			return "trades:" + symbol, nil
		case "ticker", "tickers":
			return "tickers:" + symbol, nil
		case "depth", "book", "books5", "depth5":
			return "depth:" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "okx":
		switch ch {
		case "trade", "trades":
			return "trades:" + symbol, nil
		case "ticker", "tickers":
			return "tickers:" + symbol, nil
		case "depth", "book":
			return "books:" + symbol, nil
		case "books5", "depth5":
			return "books5:" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "coinbase":
		switch ch {
		case "trade", "trades":
			return core.TopicTrade + ":" + symbol, nil
		case "ticker", "tickers":
			return core.TopicTicker + ":" + symbol, nil
		case "depth", "book", "level2":
			return core.TopicDepth + ":" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	case "kraken":
		switch ch {
		case "trade", "trades":
			return "trade:" + symbol, nil
		case "ticker", "tickers":
			return "ticker:" + symbol, nil
		case "depth", "book":
			return "book:" + symbol, nil
		default:
			return ch + ":" + symbol, nil
		}
	default:
		return ch + ":" + symbol, nil
	}
}

func runInteractiveGuide(exchange, symbol, channel string) (string, string, string, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("Welcome to the meltica market-stream interactive guide.")
	fmt.Println("Follow the prompts to choose an exchange, trading symbol, and channel.")
	fmt.Println()

	exchanges := []string{"binance", "okx", "coinbase", "kraken"}
	channels := []string{"ticker", "trades", "depth"}

	exchange = promptSelection(reader, "exchange", exchanges, exchange)
	fmt.Println()

	symbol = promptSymbol(reader, symbol)
	fmt.Println()

	channel = promptSelection(reader, "channel", channels, channel)
	fmt.Println()

	fmt.Printf("Using exchange=%s symbol=%s channel=%s\n", strings.ToLower(strings.TrimSpace(exchange)), normalizeSymbol(symbol), strings.TrimSpace(channel))
	fmt.Println()

	return exchange, symbol, channel, nil
}

func promptSelection(reader *bufio.Reader, name string, options []string, current string) string {
	current = strings.TrimSpace(current)
	if current == "" && len(options) > 0 {
		current = options[0]
	}

	for {
		fmt.Printf("Select %s:\n", name)
		fmt.Println("  Type the number or name. Press Enter to accept the default.")
		for i, opt := range options {
			fmt.Printf("  %d) %s\n", i+1, opt)
		}
		prompt := fmt.Sprintf("Enter %s", name)
		if len(options) > 0 {
			prompt = fmt.Sprintf("Enter %s (1-%d)", name, len(options))
		}
		if current != "" {
			fmt.Printf("%s [%s]: ", prompt, current)
		} else {
			fmt.Printf("%s: ", prompt)
		}

		line, err := readLine(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: unable to read %s: %v\n", name, err)
			return current
		}
		input := strings.TrimSpace(line)

		if input == "" {
			if current != "" {
				return current
			}
			continue
		}

		if idx, err := strconv.Atoi(input); err == nil {
			if idx >= 1 && idx <= len(options) {
				return options[idx-1]
			}
		}

		for _, opt := range options {
			if strings.EqualFold(opt, input) {
				return opt
			}
		}

		return input
	}
}

func promptSymbol(reader *bufio.Reader, current string) string {
	normalized := normalizeSymbol(current)
	if normalized != "" {
		current = normalized
	}
	if current == "" {
		current = "BTC-USDT"
	}

	for {
		fmt.Printf("Enter trading symbol in BASE-QUOTE form (e.g. BTC-USDT) [%s]: ", current)
		line, err := readLine(reader)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: unable to read symbol: %v\n", err)
			return current
		}
		input := strings.TrimSpace(line)
		if input == "" {
			return current
		}

		normalized = normalizeSymbol(input)
		if normalized == "" {
			fmt.Println("Symbol must be in BASE-QUOTE form like BTC-USDT. Please try again.")
			continue
		}
		return normalized
	}
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return line, nil
		}
		return line, err
	}
	return line, nil
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
	case *core.TradeEvent:
		return fmt.Sprintf("[%s] %s trade price=%s qty=%s", ts, evt.Symbol, formatRat(evt.Price), formatRat(evt.Quantity)), true
	case *core.TickerEvent:
		return fmt.Sprintf("[%s] %s ticker bid=%s ask=%s", ts, evt.Symbol, formatRat(evt.Bid), formatRat(evt.Ask)), true
	case *core.DepthEvent:
		bid := "—"
		ask := "—"
		if len(evt.Bids) > 0 {
			bid = fmt.Sprintf("%s@%s", formatRat(evt.Bids[0].Price), formatRat(evt.Bids[0].Qty))
		}
		if len(evt.Asks) > 0 {
			ask = fmt.Sprintf("%s@%s", formatRat(evt.Asks[0].Price), formatRat(evt.Asks[0].Qty))
		}
		return fmt.Sprintf("[%s] %s depth bid=%s ask=%s", ts, evt.Symbol, bid, ask), true
	case *core.OrderEvent:
		return fmt.Sprintf("[%s] %s order id=%s status=%s filled=%s avg=%s", ts, evt.Symbol, evt.OrderID, evt.Status, formatRat(evt.FilledQty), formatRat(evt.AvgPrice)), true
	case *core.BalanceEvent:
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
