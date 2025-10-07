package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/registry"
	corestreams "github.com/coachpo/meltica/core/streams"
	binanceplugin "github.com/coachpo/meltica/exchanges/binance/plugin"
	"github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

const (
	defaultSymbolList     = "BTC-USDT,ETH-USDT,BNB-USDT"
	fallbackPriceScale    = 2
	fallbackQuantityScale = 6
)

func main() {
	symbolsFlag := flag.String("symbols", defaultSymbolList, "Comma separated canonical symbols (e.g. BTC-USDT,ETH-USDT)")
	includeBook := flag.Bool("book", false, "Subscribe to order book deltas in addition to trades and tickers")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	exchange, err := registry.Resolve(binanceplugin.Name)
	if err != nil {
		log.Fatalf("resolve exchange: %v", err)
	}
	defer exchange.Close()

	symbols := selectSymbols(*symbolsFlag)
	if len(symbols) == 0 {
		log.Fatalf("no symbols provided")
	}

	if err := run(ctx, exchange, symbols, *includeBook); err != nil {
		log.Fatalf("market data error: %v", err)
	}
}

func run(ctx context.Context, exchange core.Exchange, symbols []string, withBook bool) error {
	wsParticipant, ok := exchange.(core.WebsocketParticipant)
	if !ok {
		return fmt.Errorf("exchange %s does not expose websocket access", exchange.Name())
	}

	instruments := loadInstrumentCache(ctx, exchange)
	formatter := newPrecisionFormatter(instruments)

	topics := buildTopics(symbols)
	if len(topics) == 0 {
		return fmt.Errorf("no topics to subscribe")
	}

	sub, err := wsParticipant.WS().SubscribePublic(ctx, topics...)
	if err != nil {
		return fmt.Errorf("subscribe public: %w", err)
	}
	defer sub.Close()

	fmt.Printf("Subscribed to %d topics:\n", len(topics))
	for _, topic := range topics {
		fmt.Printf("  %s\n", topic)
	}

	var bookWG sync.WaitGroup
	if withBook {
		if obs, ok := exchange.(orderBookSubscriber); ok {
			startOrderBookFeeds(ctx, &bookWG, obs, formatter, symbols)
		} else {
			log.Printf("exchange %s does not provide applied order book snapshots", exchange.Name())
		}
	}
	defer bookWG.Wait()

	msgCh := sub.C()
	errCh := sub.Err()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errCh:
			if !ok {
				return nil
			}
			if err != nil {
				return fmt.Errorf("subscription error: %w", err)
			}
		case msg, ok := <-msgCh:
			if !ok {
				return nil
			}
			printMessage(msg, formatter)
		}
	}

	sub.Close()
	bookWG.Wait()
	return nil
}

func selectSymbols(raw string) []string {
	symbols := parseSymbolList(raw)
	if len(symbols) > 0 {
		return symbols
	}
	return parseSymbolList(defaultSymbolList)
}

func parseSymbolList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	seen := make(map[string]struct{}, len(parts))
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		symbol := normalizeSymbol(part)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		out = append(out, symbol)
	}
	return out
}

func buildTopics(symbols []string) []string {
	topics := make([]string, 0, len(symbols)*2)
	for _, symbol := range symbols {
		canonical := normalizeSymbol(symbol)
		if canonical == "" {
			continue
		}
		topics = append(topics,
			core.MustCanonicalTopic(core.TopicTrade, canonical),
			core.MustCanonicalTopic(core.TopicTicker, canonical),
		)
	}
	return topics
}

func printMessage(msg core.Message, formatter precisionFormatter) {
	switch evt := msg.Parsed.(type) {
	case *corestreams.TradeEvent:
		fmt.Printf("[%s] TRADE  %s qty=%s price=%s\n",
			formatTime(evt.Time),
			evt.Symbol,
			formatter.quantity(evt.Symbol, evt.Quantity),
			formatter.price(evt.Symbol, evt.Price),
		)
	case *corestreams.TickerEvent:
		fmt.Printf("[%s] TICKER %s bid=%s ask=%s\n",
			formatTime(evt.Time),
			evt.Symbol,
			formatter.price(evt.Symbol, evt.Bid),
			formatter.price(evt.Symbol, evt.Ask),
		)
	case *corestreams.BookEvent:
		fmt.Printf("[%s] BOOK   %s bids=%s asks=%s depth=%d/%d\n",
			formatTime(evt.Time),
			evt.Symbol,
			describeTopLevel(formatter, evt.Symbol, evt.Bids),
			describeTopLevel(formatter, evt.Symbol, evt.Asks),
			len(evt.Bids),
			len(evt.Asks),
		)
	default:
		if len(msg.Raw) > 0 {
			fmt.Printf("[%s] EVENT  topic=%s route=%s raw=%s\n",
				formatTime(msg.At),
				msg.Topic,
				msg.Event,
				string(msg.Raw),
			)
		} else {
			fmt.Printf("[%s] EVENT  topic=%s\n",
				formatTime(msg.At),
				msg.Topic,
			)
		}
	}
}

func describeTopLevel(formatter precisionFormatter, symbol string, levels []core.BookDepthLevel) string {
	if len(levels) == 0 {
		return "-"
	}
	level := levels[0]
	return fmt.Sprintf("%s@%s",
		formatter.quantity(symbol, level.Qty),
		formatter.price(symbol, level.Price),
	)
}

func formatDecimal(r *big.Rat, scale int) string {
	if r == nil {
		return "0"
	}
	formatted := numeric.Format(r, scale)
	if formatted == "" {
		return "0"
	}
	return formatted
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return time.Now().UTC().Format(time.RFC3339Nano)
	}
	return t.UTC().Format(time.RFC3339Nano)
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

type instrumentPrecision struct {
	priceScale int
	qtyScale   int
}

type precisionFormatter struct {
	scales map[string]instrumentPrecision
}

func newPrecisionFormatter(instruments map[string]core.Instrument) precisionFormatter {
	scales := make(map[string]instrumentPrecision, len(instruments))
	for symbol, inst := range instruments {
		scales[normalizeSymbol(symbol)] = instrumentPrecision{
			priceScale: inst.PriceScale,
			qtyScale:   inst.QtyScale,
		}
	}
	return precisionFormatter{scales: scales}
}

func (pf precisionFormatter) price(symbol string, value *big.Rat) string {
	scale := fallbackPriceScale
	if info, ok := pf.scales[normalizeSymbol(symbol)]; ok && info.priceScale > 0 {
		scale = info.priceScale
	}
	return formatDecimal(value, scale)
}

func (pf precisionFormatter) quantity(symbol string, value *big.Rat) string {
	scale := fallbackQuantityScale
	if info, ok := pf.scales[normalizeSymbol(symbol)]; ok && info.qtyScale > 0 {
		scale = info.qtyScale
	}
	return formatDecimal(value, scale)
}

func loadInstrumentCache(ctx context.Context, exchange core.Exchange) map[string]core.Instrument {
	cache := make(map[string]core.Instrument)

	appendInstruments := func(insts []core.Instrument, err error) {
		if err != nil {
			log.Printf("load instruments: %v", err)
			return
		}
		for _, inst := range insts {
			cache[normalizeSymbol(inst.Symbol)] = inst
		}
	}

	if spot, ok := exchange.(core.SpotParticipant); ok {
		if api := spot.Spot(ctx); api != nil {
			appendInstruments(api.Instruments(ctx))
		}
	}
	if linear, ok := exchange.(core.LinearFuturesParticipant); ok {
		if api := linear.LinearFutures(ctx); api != nil {
			appendInstruments(api.Instruments(ctx))
		}
	}
	if inverse, ok := exchange.(core.InverseFuturesParticipant); ok {
		if api := inverse.InverseFutures(ctx); api != nil {
			appendInstruments(api.Instruments(ctx))
		}
	}
	return cache
}

type orderBookSubscriber interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

func startOrderBookFeeds(
	ctx context.Context,
	wg *sync.WaitGroup,
	subscriber orderBookSubscriber,
	formatter precisionFormatter,
	symbols []string,
) {
	for _, symbol := range symbols {
		symbol := normalizeSymbol(symbol)
		if symbol == "" {
			continue
		}

		wg.Add(1)
		go func(sym string) {
			defer wg.Done()

			feedCtx, cancel := context.WithCancel(ctx)
			defer cancel()

			events, errs, err := subscriber.OrderBookSnapshots(feedCtx, sym)
			if err != nil {
				log.Printf("order book feed %s: %v", sym, err)
				return
			}

			eventCh := events
			errCh := errs
			for {
				select {
				case <-feedCtx.Done():
					return
				case evt, ok := <-eventCh:
					if !ok {
						eventCh = nil
						if errCh == nil {
							return
						}
						continue
					}
					printBookSnapshot(evt, formatter)
				case err, ok := <-errCh:
					if !ok {
						errCh = nil
						if eventCh == nil {
							return
						}
						continue
					}
					if err != nil {
						log.Printf("order book feed %s error: %v", sym, err)
					}
				}
			}
		}(symbol)
	}
}

func printBookSnapshot(evt corestreams.BookEvent, formatter precisionFormatter) {
	fmt.Printf("[%s] BOOK   %s bids=%s asks=%s depth=%d/%d snapshot\n",
		formatTime(evt.Time),
		evt.Symbol,
		describeTopLevel(formatter, evt.Symbol, evt.Bids),
		describeTopLevel(formatter, evt.Symbol, evt.Asks),
		len(evt.Bids),
		len(evt.Asks),
	)
}
