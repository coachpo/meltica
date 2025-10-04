package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	coreprovider "github.com/coachpo/meltica/core/provider"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance"
	"github.com/coachpo/meltica/providers/coinbase"
	"github.com/coachpo/meltica/providers/kraken"
	"github.com/coachpo/meltica/providers/okx"
)

const (
	displayLevels   = 5
	refreshInterval = 200 * time.Millisecond
)

type providerInstance interface {
	Close() error
	WS() core.WS
	Spot(ctx context.Context) core.SpotAPI
}

type orderBookSnapshotProvider interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan coreprovider.BookEvent, <-chan error, error)
}

type providerConfig struct {
	name            string
	nativeSymbol    string
	canonicalSymbol string
	instantiate     func() (providerInstance, error)
}

var renderMu sync.Mutex

func main() {
	// Define command-line flags
	var (
		enableBinance  = flag.Bool("binance", false, "Enable Binance provider")
		enableCoinbase = flag.Bool("coinbase", false, "Enable Coinbase provider")
		enableKraken   = flag.Bool("kraken", false, "Enable Kraken provider")
		enableOKX      = flag.Bool("okx", false, "Enable OKX provider")
		enableAll      = flag.Bool("all", false, "Enable all providers")
		providersList  = flag.String("providers", "", "Comma-separated list of providers to enable (binance,coinbase,kraken,okx)")
	)

	flag.Parse()

	// Parse providers from command line
	enabledProviders := parseEnabledProviders(*enableBinance, *enableCoinbase, *enableKraken, *enableOKX, *enableAll, *providersList)

	if len(enabledProviders) == 0 {
		log.Fatal("No providers enabled. Use -all to enable all providers or specify individual providers with -binance, -coinbase, -kraken, -okx, or -providers=binance,coinbase")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	allProviders := []providerConfig{
		{
			name:            "Binance",
			nativeSymbol:    "BTCUSDT",
			canonicalSymbol: "BTC-USDT",
			instantiate: func() (providerInstance, error) {
				return binance.New("", "")
			},
		},
		{
			name:            "Coinbase",
			nativeSymbol:    "BTC-USD",
			canonicalSymbol: "BTC-USD",
			instantiate: func() (providerInstance, error) {
				return coinbase.New("", "", "")
			},
		},
		{
			name:            "Kraken",
			nativeSymbol:    "XBT/USDT",
			canonicalSymbol: "XBT-USDT",
			instantiate: func() (providerInstance, error) {
				return kraken.New("", "")
			},
		},
		{
			name:            "OKX",
			nativeSymbol:    "BTC-USDT",
			canonicalSymbol: "BTC-USDT",
			instantiate: func() (providerInstance, error) {
				return okx.New("", "", "")
			},
		},
	}

	// Filter providers based on enabled flags
	providers := filterProviders(allProviders, enabledProviders)

	fmt.Printf("Starting Multi-Provider Order Book Management Validation...\n")
	fmt.Printf("Enabled providers: %s\n", strings.Join(getProviderNames(providers), ", "))

	var wg sync.WaitGroup

	for _, cfg := range providers {
		fmt.Printf("Starting %s Order Book Management Validation...\n", cfg.name)
		fmt.Printf("Monitoring %s depth via provider pipelines...\n", cfg.nativeSymbol)

		prov, err := cfg.instantiate()
		if err != nil {
			log.Fatalf("failed to create %s provider: %v", cfg.name, err)
		}

		wg.Add(1)
		go func(cfg providerConfig, prov providerInstance) {
			defer wg.Done()
			defer prov.Close()

			validateRESTLayer(ctx, cfg, prov)
			validateWSRouting(ctx, cfg, prov)

			events, errCh, err := obtainBookStream(ctx, cfg, prov)
			if err != nil {
				log.Printf("[%s] stream start failed: %v", cfg.name, err)
				return
			}

			renderMu.Lock()
			fmt.Printf("[%s] awaiting order book snapshots...\n", cfg.name)
			renderMu.Unlock()

			var lastRender time.Time
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-errCh:
					if err != nil {
						log.Printf("[%s] stream error: %v", cfg.name, err)
					}
					return
				case evt, ok := <-events:
					if !ok {
						return
					}
					if !lastRender.IsZero() && time.Since(lastRender) < refreshInterval {
						continue
					}
					lastRender = time.Now()
					renderMu.Lock()
					renderSnapshot(cfg.name, cfg.nativeSymbol, evt)
					renderMu.Unlock()
				}
			}
		}(cfg, prov)
	}

	<-ctx.Done()
	fmt.Println("Stopping order book monitor...")

	wg.Wait()
}

// parseEnabledProviders determines which providers are enabled based on command-line flags
func parseEnabledProviders(enableBinance, enableCoinbase, enableKraken, enableOKX, enableAll bool, providersList string) map[string]bool {
	enabled := make(map[string]bool)

	// If -all is specified, enable all providers
	if enableAll {
		enabled["binance"] = true
		enabled["coinbase"] = true
		enabled["kraken"] = true
		enabled["okx"] = true
		return enabled
	}

	// Parse individual provider flags
	if enableBinance {
		enabled["binance"] = true
	}
	if enableCoinbase {
		enabled["coinbase"] = true
	}
	if enableKraken {
		enabled["kraken"] = true
	}
	if enableOKX {
		enabled["okx"] = true
	}

	// Parse comma-separated providers list
	if providersList != "" {
		providers := strings.Split(strings.ToLower(providersList), ",")
		for _, p := range providers {
			p = strings.TrimSpace(p)
			switch p {
			case "binance", "coinbase", "kraken", "okx":
				enabled[p] = true
			}
		}
	}

	return enabled
}

// filterProviders returns only the providers that are enabled
func filterProviders(allProviders []providerConfig, enabled map[string]bool) []providerConfig {
	var filtered []providerConfig
	for _, p := range allProviders {
		if enabled[strings.ToLower(p.name)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// getProviderNames returns a list of provider names for display
func getProviderNames(providers []providerConfig) []string {
	names := make([]string, len(providers))
	for i, p := range providers {
		names[i] = p.name
	}
	return names
}

func validateRESTLayer(ctx context.Context, cfg providerConfig, prov providerInstance) {
	switch p := prov.(type) {
	case *binance.Provider:
		const snapshotDepth = 100
		snapshot, updateID, err := p.DepthSnapshot(ctx, cfg.canonicalSymbol, snapshotDepth)
		if err != nil {
			log.Printf("[%s] REST validation failed: %v", cfg.name, err)
			return
		}
		fmt.Printf("[%s] REST layer OK: snapshot %d levels fetched (LastUpdateID=%d)\n", cfg.name, len(snapshot.Bids)+len(snapshot.Asks), updateID)
		topBid := bestLevel(snapshot.Bids, true)
		topAsk := bestLevel(snapshot.Asks, false)
		fmt.Printf("  Top of book via REST -> Bid %s / %s, Ask %s / %s\n",
			formatRat(topBid.qty, 6), formatRat(topBid.price, 2),
			formatRat(topAsk.price, 2), formatRat(topAsk.qty, 6))
	case *coinbase.Provider:
		spot := p.Spot(ctx)
		if ticker, err := spot.Ticker(ctx, cfg.canonicalSymbol); err != nil {
			log.Printf("[%s] REST ticker failed: %v", cfg.name, err)
		} else {
			fmt.Printf("[%s] REST layer OK: ticker bid %s ask %s\n", cfg.name, formatRat(ticker.Bid, 2), formatRat(ticker.Ask, 2))
		}
	case *kraken.Provider:
		if _, err := p.Spot(ctx).Instruments(ctx); err != nil {
			log.Printf("[%s] REST instruments failed: %v", cfg.name, err)
		}
		if ticker, err := p.Spot(ctx).Ticker(ctx, cfg.canonicalSymbol); err != nil {
			log.Printf("[%s] REST ticker failed: %v", cfg.name, err)
		} else {
			fmt.Printf("[%s] REST layer OK: ticker bid %s ask %s\n", cfg.name, formatRat(ticker.Bid, 2), formatRat(ticker.Ask, 2))
		}
	case *okx.Provider:
		if ticker, err := p.Spot(ctx).Ticker(ctx, cfg.canonicalSymbol); err != nil {
			log.Printf("[%s] REST ticker failed: %v", cfg.name, err)
		} else {
			fmt.Printf("[%s] REST layer OK: ticker bid %s ask %s\n", cfg.name, formatRat(ticker.Bid, 2), formatRat(ticker.Ask, 2))
		}
	default:
		if ticker, err := prov.Spot(ctx).Ticker(ctx, cfg.canonicalSymbol); err == nil {
			fmt.Printf("[%s] REST layer OK: ticker bid %s ask %s\n", cfg.name, formatRat(ticker.Bid, 2), formatRat(ticker.Ask, 2))
		} else {
			log.Printf("[%s] REST ticker failed: %v", cfg.name, err)
		}
	}
}

func validateWSRouting(ctx context.Context, cfg providerConfig, prov providerInstance) {
	ws := prov.WS()
	sub, err := ws.SubscribePublic(ctx,
		corews.BookTopic(cfg.canonicalSymbol),
		corews.TickerTopic(cfg.canonicalSymbol),
		corews.TradeTopic(cfg.canonicalSymbol))
	if err != nil {
		log.Printf("[%s] WS routing validation failed: %v", cfg.name, err)
		return
	}

	go func() {
		defer sub.Close()
		validated := map[string]bool{
			coreprovider.RouteBookSnapshot: false,
			coreprovider.RouteTickerUpdate: false,
			coreprovider.RouteTradeUpdate:  false,
		}
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				if err != nil {
					log.Printf("[%s] WS routing error: %v", cfg.name, err)
				}
				return
			case msg, ok := <-sub.C():
				if !ok {
					return
				}
				switch evt := msg.Parsed.(type) {
				case *coreprovider.BookEvent:
					if !validated[coreprovider.RouteBookSnapshot] {
						fmt.Printf("[%s] WS routing OK: depth update %d/%d levels\n", cfg.name, len(evt.Bids), len(evt.Asks))
						validated[coreprovider.RouteBookSnapshot] = true
					}
				case *coreprovider.TickerEvent:
					if !validated[coreprovider.RouteTickerUpdate] {
						fmt.Printf("[%s] WS routing OK: ticker update bid %s ask %s\n", cfg.name, formatRat(evt.Bid, 2), formatRat(evt.Ask, 2))
						validated[coreprovider.RouteTickerUpdate] = true
					}
				case *coreprovider.TradeEvent:
					if !validated[coreprovider.RouteTradeUpdate] {
						fmt.Printf("[%s] WS routing OK: trade %s @ %s\n", cfg.name, formatRat(evt.Quantity, 4), formatRat(evt.Price, 2))
						validated[coreprovider.RouteTradeUpdate] = true
					}
				}
				done := true
				for _, v := range validated {
					if !v {
						done = false
						break
					}
				}
				if done {
					return
				}
			}
		}
	}()
}

func obtainBookStream(ctx context.Context, cfg providerConfig, prov providerInstance) (<-chan coreprovider.BookEvent, <-chan error, error) {
	if streamer, ok := prov.(orderBookSnapshotProvider); ok {
		return streamer.OrderBookSnapshots(ctx, cfg.canonicalSymbol)
	}
	return subscribeBookStream(ctx, prov.WS(), cfg.canonicalSymbol)
}

func subscribeBookStream(ctx context.Context, ws core.WS, canonicalSymbol string) (<-chan coreprovider.BookEvent, <-chan error, error) {
	sub, err := ws.SubscribePublic(ctx, corews.BookTopic(canonicalSymbol))
	if err != nil {
		return nil, nil, err
	}
	events := make(chan coreprovider.BookEvent, 32)
	errs := make(chan error, 1)

	go func() {
		defer close(events)
		defer close(errs)
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-sub.Err():
				if err != nil {
					select {
					case errs <- err:
					default:
					}
				}
				return
			case msg, ok := <-sub.C():
				if !ok {
					return
				}
				if msg.Event != coreprovider.RouteBookSnapshot {
					continue
				}
				book, ok := msg.Parsed.(*coreprovider.BookEvent)
				if !ok {
					continue
				}
				events <- cloneBookEvent(*book)
			}
		}
	}()

	return events, errs, nil
}

func cloneBookEvent(evt coreprovider.BookEvent) coreprovider.BookEvent {
	clone := coreprovider.BookEvent{
		Symbol: evt.Symbol,
		Time:   evt.Time,
		Bids:   cloneLevels(evt.Bids),
		Asks:   cloneLevels(evt.Asks),
	}
	return clone
}

func cloneLevels(levels []core.BookDepthLevel) []core.BookDepthLevel {
	out := make([]core.BookDepthLevel, len(levels))
	for i, lvl := range levels {
		out[i].Price = cloneRat(lvl.Price)
		out[i].Qty = cloneRat(lvl.Qty)
	}
	return out
}

func cloneRat(r *big.Rat) *big.Rat {
	if r == nil {
		return nil
	}
	var out big.Rat
	out.Set(r)
	return &out
}

func renderSnapshot(providerName, nativeSymbol string, event coreprovider.BookEvent) {
	bids := topLevels(event.Bids, displayLevels, true)
	asks := topLevels(event.Asks, displayLevels, false)

	fmt.Printf("%s %s Depth | Update at %s | Bids %d | Asks %d\n",
		providerName, nativeSymbol, event.Time.Format(time.RFC3339Nano), len(event.Bids), len(event.Asks))
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

func bestLevel(levels []core.BookDepthLevel, desc bool) bookLevel {
	filtered := topLevels(levels, 1, desc)
	if len(filtered) == 0 {
		return bookLevel{}
	}
	return filtered[0]
}

func formatRat(r *big.Rat, precision int) string {
	if r == nil {
		return "0"
	}
	return r.FloatString(precision)
}
