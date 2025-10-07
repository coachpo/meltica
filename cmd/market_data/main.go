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
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/registry"
	corestreams "github.com/coachpo/meltica/core/streams"
	binancel4 "github.com/coachpo/meltica/exchanges/binance/filter"
	binanceplugin "github.com/coachpo/meltica/exchanges/binance/plugin"
	"github.com/coachpo/meltica/exchanges/shared/routing"
	"github.com/coachpo/meltica/internal/numeric"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

const (
	defaultSymbolList     = "BTC-USDT,ETH-USDT,BNB-USDT,SOL-USDT,PEPE-USDT,DOGE-USDT,CAKE-USDT,XRP-USDT,ADA-USDT,BROCCOLI714-USDT"
	fallbackPriceScale    = 2
	fallbackQuantityScale = 6
	defaultBookDepth      = 50
)

func main() {
	symbolsFlag := flag.String("symbols", defaultSymbolList, "Comma separated canonical symbols (e.g. BTC-USDT,ETH-USDT)")
	bookDepth := flag.Int("book-depth", defaultBookDepth, "Maximum depth levels per side to display for books (<=0 keeps full depth)")
	minEmit := flag.Duration("throttle", 0, "Minimum interval between successive events per symbol/type (e.g. 250ms)")
	enableVWAP := flag.Bool("enable-vwap", true, "Emit rolling VWAP analytics events for trades")
	enablePrivate := flag.Bool("private", false, "Enable private stream subscriptions (account, orders)")
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

	if err := run(ctx, exchange, symbols, *bookDepth, *minEmit, *enableVWAP, *enablePrivate); err != nil {
		log.Fatalf("market data error: %v", err)
	}
}

func run(ctx context.Context, exchange core.Exchange, symbols []string, bookDepth int, minEmit time.Duration, enableVWAP bool, enablePrivate bool) error {
	instruments := loadInstrumentCache(ctx, exchange)
	formatter := newPrecisionFormatter(instruments)

	var (
		interactionFacade *mdfilter.InteractionFacade
		pipelineStream    mdfilter.PipelineStream
		err               error
	)

	interactionFacade, pipelineStream, err = startMarketPipeline(ctx, exchange, symbols, bookDepth, minEmit, enableVWAP, enablePrivate)
	if err != nil {
		return fmt.Errorf("start filter: %w", err)
	}
	defer pipelineStream.Close()
	defer interactionFacade.Close()

	fmt.Printf("Streaming %d symbols: trades & tickers", len(symbols))
	fmt.Printf(" + books(depth=%d)", bookDepth)
	if minEmit > 0 {
		fmt.Printf(" throttle=%s", minEmit)
	}
	if !enableVWAP {
		fmt.Printf(" vwap=disabled")
	}
	if enablePrivate {
		fmt.Printf(" private=enabled")
	}
	fmt.Println()

	eventCh := pipelineStream.Events
	errCh := pipelineStream.Errors

	for {
		select {
		case <-ctx.Done():
			return nil
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				if eventCh == nil {
					return nil
				}
				continue
			}
			if err != nil {
				log.Printf("market data error: %v", err)
			}
		case evt, ok := <-eventCh:
			if !ok {
				eventCh = nil
				if errCh == nil {
					return nil
				}
				continue
			}
			switch payload := evt.Payload.(type) {
			case mdfilter.TradePayload:
				if payload.Trade != nil {
					printTradeEvent(*payload.Trade, formatter)
				}
			case mdfilter.TickerPayload:
				if payload.Ticker != nil {
					printTickerEvent(*payload.Ticker, formatter)
				}
			case mdfilter.BookPayload:
				if payload.Book != nil {
					printBookSnapshot(*payload.Book, formatter)
				}
			case mdfilter.AnalyticsPayload:
				if payload.Analytics != nil {
					printVWAPEvent(*payload.Analytics, evt.At, formatter)
				}
			case mdfilter.AccountPayload:
				if payload.Account != nil {
					printAccountEvent(*payload.Account, formatter)
				}
			case mdfilter.OrderPayload:
				if payload.Order != nil {
					printOrderEvent(*payload.Order, formatter)
				}
			default:
				// ignore for now
			}
		}
	}
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

func printTradeEvent(evt corestreams.TradeEvent, formatter precisionFormatter) {
	fmt.Printf("[%s] TRADE  %s qty=%s price=%s\n",
		formatTime(evt.Time),
		evt.Symbol,
		formatter.quantity(evt.Symbol, evt.Quantity),
		formatter.price(evt.Symbol, evt.Price),
	)
}

func printTickerEvent(evt corestreams.TickerEvent, formatter precisionFormatter) {
	fmt.Printf("[%s] TICKER %s bid=%s ask=%s\n",
		formatTime(evt.Time),
		evt.Symbol,
		formatter.price(evt.Symbol, evt.Bid),
		formatter.price(evt.Symbol, evt.Ask),
	)
}

func printVWAPEvent(evt mdfilter.AnalyticsEvent, ts time.Time, formatter precisionFormatter) {
	vwap := "-"
	if evt.VWAP != nil {
		vwap = formatter.price(evt.Symbol, evt.VWAP)
	}
	fmt.Printf("[%s] VWAP   %s vwap=%s trades=%d\n",
		formatTime(ts),
		evt.Symbol,
		vwap,
		evt.TradeCount,
	)
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

func startMarketPipeline(
	ctx context.Context,
	exchange core.Exchange,
	symbols []string,
	bookDepth int,
	minEmit time.Duration,
	enableVWAP bool,
	enablePrivate bool,
) (*mdfilter.InteractionFacade, mdfilter.PipelineStream, error) {
	adapter, auth, err := resolveFilterAdapter(exchange)
	if err != nil {
		return nil, mdfilter.PipelineStream{}, err
	}

	facade := mdfilter.NewInteractionFacade(adapter, auth)

	// Build options for subscription
	var options []mdfilter.PublicOption
	options = append(options, mdfilter.WithTrades(), mdfilter.WithTickers(), mdfilter.WithBooks())
	if bookDepth > 0 {
		options = append(options, mdfilter.WithBookDepth(bookDepth))
	}
	if minEmit > 0 {
		options = append(options, mdfilter.WithMinEmitInterval(minEmit))
	}
	//TODO: explain what is snapshots, and WithSnapshots
	options = append(options, mdfilter.WithSnapshots())
	if enableVWAP {
		options = append(options, mdfilter.WithVWAP())
	}

	var stream mdfilter.PipelineStream

	// Use multichannel subscription if private is enabled
	if enablePrivate && auth != nil {
		stream, err = facade.MultiChannelStream(ctx, symbols, enablePrivate, options...)
	} else {
		stream, err = facade.SubscribePublic(ctx, symbols, options...)
	}

	if err != nil {
		facade.Close()
		return nil, mdfilter.PipelineStream{}, err
	}
	return facade, stream, nil
}

// TODO study this
func resolveFilterAdapter(exchange core.Exchange) (mdfilter.Adapter, *mdfilter.AuthContext, error) {
	switch exchange.Name() {
	case string(binanceplugin.Name):
		return buildBinanceFilterAdapter(exchange)
	default:
		return nil, nil, fmt.Errorf("exchange %s does not expose a filter adapter", exchange.Name())
	}
}

// TODO study this
func buildBinanceFilterAdapter(exchange core.Exchange) (mdfilter.Adapter, *mdfilter.AuthContext, error) {
	var books interface {
		BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
	}
	if bs, ok := exchange.(interface {
		BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
	}); ok {
		books = bs
	}

	var ws core.WS
	if participant, ok := exchange.(core.WebsocketParticipant); ok {
		ws = participant.WS()
	}

	// Check for private capabilities
	var privateWS interface {
		SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error)
	}
	var restRouter routing.RESTDispatcher

	// Try to extract private capabilities from the exchange
	if privateParticipant, ok := exchange.(interface {
		PrivateWS() core.WS
	}); ok {
		privateWS = privateParticipant.PrivateWS()
	}

	if restParticipant, ok := exchange.(interface {
		RESTRouter() interface{}
	}); ok {
		if router := restParticipant.RESTRouter(); router != nil {
			if r, ok := router.(routing.RESTDispatcher); ok {
				restRouter = r
			}
		}
	}

	// Build auth context if credentials are available
	var auth *mdfilter.AuthContext
	if creds, ok := exchange.(interface {
		Credentials() (string, string)
	}); ok {
		apiKey, secret := creds.Credentials()
		if apiKey != "" && secret != "" {
			auth = &mdfilter.AuthContext{
				APIKey: apiKey,
				Secret: secret,
			}
		}
	}

	if books == nil && ws == nil && privateWS == nil && restRouter == nil {
		return nil, nil, fmt.Errorf("exchange %s does not expose required feeds", exchange.Name())
	}

	var adapter mdfilter.Adapter
	var err error

	// Use enhanced adapter with REST capabilities if available
	if restRouter != nil {
		adapter, err = binancel4.NewAdapterWithREST(books, ws, privateWS, restRouter)
	} else {
		// Fall back to basic adapter without REST capabilities
		adapter, err = binancel4.NewAdapter(books, ws)
	}

	if err != nil {
		return nil, nil, err
	}
	return adapter, auth, nil
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

func printAccountEvent(evt mdfilter.AccountEvent, formatter precisionFormatter) {
	fmt.Printf("[%s] ACCOUNT %s balance=%s available=%s locked=%s\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		evt.Symbol,
		formatter.quantity(evt.Symbol, evt.Balance),
		formatter.quantity(evt.Symbol, evt.Available),
		formatter.quantity(evt.Symbol, evt.Locked),
	)
}

func printOrderEvent(evt mdfilter.OrderEvent, formatter precisionFormatter) {
	price := "-"
	if evt.Price != nil {
		price = formatter.price(evt.Symbol, evt.Price)
	}
	fmt.Printf("[%s] ORDER  %s id=%s side=%s price=%s qty=%s status=%s\n",
		time.Now().UTC().Format(time.RFC3339Nano),
		evt.Symbol,
		evt.OrderID,
		evt.Side,
		price,
		formatter.quantity(evt.Symbol, evt.Quantity),
		evt.Status,
	)
}
