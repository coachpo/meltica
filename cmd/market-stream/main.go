package main

import (
	"context"
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
	_ "github.com/coachpo/meltica/providers/binance"
	_ "github.com/coachpo/meltica/providers/coinbase"
	_ "github.com/coachpo/meltica/providers/kraken"
	_ "github.com/coachpo/meltica/providers/okx"
)

func main() {
	var (
		exchangeFlag = flag.String("exchange", "binance", "Exchange to connect to (binance, okx, coinbase, kraken)")
		symbolFlag   = flag.String("symbol", "BTC-USDT", "Trading symbol in canonical BASE-QUOTE form")
		channelFlag  = flag.String("channel", "ticker", "Market data stream to subscribe to (ticker, trades, depth)")
	)

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: market-stream [flags]\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Continuously stream public market data via meltica.\n\n")
		flag.PrintDefaults()
	}

	flag.Parse()

	exchange := strings.ToLower(strings.TrimSpace(*exchangeFlag))
	if exchange == "" {
		fatal("exchange is required")
	}

	symbol := normalizeSymbol(*symbolFlag)
	if symbol == "" {
		fatal("symbol must be in BASE-QUOTE form, e.g. BTC-USDT")
	}

	channel := strings.TrimSpace(*channelFlag)
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
			fmt.Println(formatMessage(msg))
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

func formatMessage(msg core.Message) string {
	ts := msg.At.UTC().Format(time.RFC3339)
	if msg.Event != "" {
		switch evt := msg.Parsed.(type) {
		case *core.TradeEvent:
			return fmt.Sprintf("[%s] %s trade price=%s qty=%s", ts, evt.Symbol, formatRat(evt.Price), formatRat(evt.Quantity))
		case *core.TickerEvent:
			return fmt.Sprintf("[%s] %s ticker bid=%s ask=%s", ts, evt.Symbol, formatRat(evt.Bid), formatRat(evt.Ask))
		case *core.DepthEvent:
			bid := "—"
			ask := "—"
			if len(evt.Bids) > 0 {
				bid = fmt.Sprintf("%s@%s", formatRat(evt.Bids[0].Price), formatRat(evt.Bids[0].Qty))
			}
			if len(evt.Asks) > 0 {
				ask = fmt.Sprintf("%s@%s", formatRat(evt.Asks[0].Price), formatRat(evt.Asks[0].Qty))
			}
			return fmt.Sprintf("[%s] %s depth bid=%s ask=%s", ts, evt.Symbol, bid, ask)
		case *core.OrderEvent:
			return fmt.Sprintf("[%s] %s order id=%s status=%s filled=%s avg=%s", ts, evt.Symbol, evt.OrderID, evt.Status, formatRat(evt.FilledQty), formatRat(evt.AvgPrice))
		case *core.BalanceEvent:
			return fmt.Sprintf("[%s] balance update assets=%d", ts, len(evt.Balances))
		}
	}
	if len(msg.Raw) > 0 {
		raw := string(msg.Raw)
		if len(raw) > 256 {
			raw = raw[:256] + "…"
		}
		if msg.Topic != "" {
			return fmt.Sprintf("[%s] %s raw=%s", ts, msg.Topic, raw)
		}
		return fmt.Sprintf("[%s] raw=%s", ts, raw)
	}
	return fmt.Sprintf("[%s] event=%s", ts, msg.Event)
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
