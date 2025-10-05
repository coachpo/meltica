package main

import (
	"context"
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
	registry "github.com/coachpo/meltica/core/registry"
	registrybinance "github.com/coachpo/meltica/core/registry/binance"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretopics "github.com/coachpo/meltica/core/topics"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
)

const (
	defaultBase              = "BTC"
	defaultQuote             = "USDT"
	fallbackPriceScale       = 2
	fallbackQuantityScale    = 6
	defaultBookLogInterval   = 200 * time.Millisecond
	maxTopicsPerSubscription = 200
)

// SubscriptionConfig defines symbol-level tuning for market data streams.
type SubscriptionConfig struct {
	DepthLevels    int
	UpdateInterval time.Duration
}

// SymbolSubscriptionRequest describes desired subscription behaviour for a symbol.
type SymbolSubscriptionRequest struct {
	Symbol string
	Trades bool
	Ticker bool
	Book   bool
	Config SubscriptionConfig
}

type symbolSubscription struct {
	Symbol string
	Trades bool
	Ticker bool
	Book   bool
	Config SubscriptionConfig
	Paused bool
}

type bookFeed struct {
	cancel context.CancelFunc
	cfg    SubscriptionConfig
}

type marketEventKind string

const (
	marketEventTrade  marketEventKind = "trade"
	marketEventTicker marketEventKind = "ticker"
	marketEventBook   marketEventKind = "book"
	marketEventError  marketEventKind = "error"
	marketEventSystem marketEventKind = "system"
)

type marketEvent struct {
	Kind    marketEventKind
	Topic   string
	Payload interface{}
	Err     error
	Time    time.Time
	Message string
}

type marketCommand interface{}

type marketSubscribeCommand struct {
	Topics     []string
	BookSymbol string
}

type marketUnsubscribeCommand struct{}

// MarketManager manages concurrent market data streams
type MarketManager struct {
	exchange   core.Exchange
	orderBooks orderBookProvider
	ws         core.WebsocketParticipant
	events     chan marketEvent
	wg         sync.WaitGroup
	mu         sync.RWMutex
	cancel     context.CancelFunc
	ctx        context.Context

	// Active subscriptions
	subs      []core.Subscription
	bookFeeds map[string]*bookFeed
	symbols   map[string]*symbolSubscription
	refreshCh chan struct{}
}

type orderBookProvider interface {
	OrderBookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error)
}

func NewMarketManager(exchange core.Exchange) (*MarketManager, error) {
	wsParticipant, ok := exchange.(core.WebsocketParticipant)
	if !ok {
		return nil, fmt.Errorf("exchange %s does not expose websocket access", exchange.Name())
	}
	orderBook, _ := exchange.(orderBookProvider)
	return &MarketManager{
		exchange:   exchange,
		ws:         wsParticipant,
		orderBooks: orderBook,
		events:     make(chan marketEvent, 256),
		bookFeeds:  make(map[string]*bookFeed),
		symbols:    make(map[string]*symbolSubscription),
	}, nil
}

func (m *MarketManager) Start(ctx context.Context) {
	m.ctx, m.cancel = context.WithCancel(ctx)
	m.refreshCh = make(chan struct{}, 1)

	// Start the event router
	m.wg.Add(1)
	go m.eventRouter(m.ctx)

	// Coordinate subscription rebuilds asynchronously
	m.wg.Add(1)
	go m.subscriptionCoordinator(m.ctx)
}

func (m *MarketManager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.cleanupSubscriptions()
	if m.refreshCh != nil {
		close(m.refreshCh)
		m.refreshCh = nil
	}
	m.mu.Unlock()
	m.wg.Wait()
	close(m.events)
}

func (m *MarketManager) Events() <-chan marketEvent {
	return m.events
}

func (m *MarketManager) Subscribe(cmd marketSubscribeCommand) {
	reqs := make(map[string]*SymbolSubscriptionRequest)
	for _, topic := range cmd.Topics {
		channel, symbol := coretopics.Parse(topic)
		symbol = normalizeSymbol(symbol)
		if symbol == "" {
			continue
		}
		req, ok := reqs[symbol]
		if !ok {
			req = &SymbolSubscriptionRequest{Symbol: symbol}
			reqs[symbol] = req
		}
		switch channel {
		case coretopics.TopicTrade:
			req.Trades = true
		case coretopics.TopicTicker:
			req.Ticker = true
		case coretopics.TopicBook:
			req.Book = true
		}
	}
	if book := normalizeSymbol(cmd.BookSymbol); book != "" {
		req, ok := reqs[book]
		if !ok {
			req = &SymbolSubscriptionRequest{Symbol: book}
			reqs[book] = req
		}
		req.Book = true
	}
	if len(reqs) == 0 {
		return
	}
	reqSlice := make([]SymbolSubscriptionRequest, 0, len(reqs))
	for _, req := range reqs {
		reqSlice = append(reqSlice, *req)
	}
	m.SubscribeSymbols(reqSlice...)
	m.emit(marketEvent{
		Kind:    marketEventSystem,
		Message: fmt.Sprintf("subscribed topics=%v book=%s", cmd.Topics, cmd.BookSymbol),
	})
}

func (m *MarketManager) Unsubscribe() {
	active := m.ActiveSubscriptions()
	if len(active) == 0 {
		return
	}
	symbols := make([]string, 0, len(active))
	for _, sub := range active {
		symbols = append(symbols, sub.Symbol)
	}
	m.UnsubscribeSymbols(symbols...)
	m.emit(marketEvent{Kind: marketEventSystem, Message: "unsubscribed"})
}

func (m *MarketManager) cleanupSubscriptions() {
	for _, sub := range m.subs {
		if sub != nil {
			_ = sub.Close()
		}
	}
	m.subs = nil
	for symbol, feed := range m.bookFeeds {
		if feed != nil && feed.cancel != nil {
			feed.cancel()
		}
		delete(m.bookFeeds, symbol)
	}
}

func (m *MarketManager) subscriptionCoordinator(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-m.refreshCh:
			if !ok {
				return
			}
			m.rebuildSubscriptions()
		}
	}
}

func (m *MarketManager) rebuildSubscriptions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.ctx == nil {
		return
	}

	topics := m.collectActiveTopicsLocked()
	m.resetWebSocketSubscriptionsLocked()
	if len(topics) > 0 {
		for _, batch := range chunkTopics(topics, maxTopicsPerSubscription) {
			m.startWebSocketSubscriptionLocked(batch)
		}
	}
	m.syncBookFeedsLocked()
}

func (m *MarketManager) collectActiveTopicsLocked() []string {
	if len(m.symbols) == 0 {
		return nil
	}
	topics := make([]string, 0, len(m.symbols)*3)
	for symbol, sub := range m.symbols {
		if sub.Paused {
			continue
		}
		if sub.Trades {
			topics = append(topics, coretopics.Trade(symbol))
		}
		if sub.Ticker {
			topics = append(topics, coretopics.Ticker(symbol))
		}
	}
	return topics
}

func (m *MarketManager) resetWebSocketSubscriptionsLocked() {
	for _, sub := range m.subs {
		if sub != nil {
			_ = sub.Close()
		}
	}
	m.subs = nil
}

func (m *MarketManager) startWebSocketSubscriptionLocked(topics []string) {
	if len(topics) == 0 {
		return
	}
	ctx := m.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	sub, err := m.ws.WS().SubscribePublic(ctx, topics...)
	if err != nil {
		m.emit(marketEvent{Kind: marketEventError, Message: "subscribe public", Err: err})
		log.Printf("failed to subscribe topics %v: %v", topics, err)
		return
	}
	m.subs = append(m.subs, sub)
	m.wg.Add(1)
	go m.handleWebSocketMessages(ctx, sub.C(), sub.Err())
	log.Printf("subscribed %d topics", len(topics))
}

func (m *MarketManager) syncBookFeedsLocked() {
	needed := make(map[string]SubscriptionConfig)
	for symbol, sub := range m.symbols {
		if sub.Paused || !sub.Book {
			continue
		}
		needed[symbol] = sub.Config
	}
	for symbol, feed := range m.bookFeeds {
		cfg, ok := needed[symbol]
		if !ok || feed.cfg != cfg {
			if feed.cancel != nil {
				feed.cancel()
			}
			delete(m.bookFeeds, symbol)
		}
	}
	for symbol, cfg := range needed {
		if _, ok := m.bookFeeds[symbol]; ok {
			continue
		}
		m.startOrderBookFeedLocked(symbol, cfg)
	}
}

func (m *MarketManager) startOrderBookFeedLocked(symbol string, cfg SubscriptionConfig) {
	if m.orderBooks == nil {
		m.events <- marketEvent{Kind: marketEventError, Message: "order book snapshots not supported"}
		return
	}
	baseCtx := m.ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	snapshots, errs, err := m.orderBooks.OrderBookSnapshots(ctx, symbol)
	if err != nil {
		cancel()
		m.events <- marketEvent{Kind: marketEventError, Message: "order book initialization", Err: err}
		return
	}
	m.bookFeeds[symbol] = &bookFeed{cancel: cancel, cfg: cfg}
	m.wg.Add(1)
	go m.handleOrderBookSnapshots(ctx, symbol, cfg, snapshots, errs)
	log.Printf("started order book feed for %s", symbol)
}

func (m *MarketManager) queueRefreshLocked() {
	if m.refreshCh == nil {
		return
	}
	select {
	case m.refreshCh <- struct{}{}:
	default:
	}
}

func chunkTopics(topics []string, size int) [][]string {
	if len(topics) == 0 {
		return nil
	}
	if size <= 0 || len(topics) <= size {
		return [][]string{topics}
	}
	chunks := make([][]string, 0, (len(topics)+size-1)/size)
	for start := 0; start < len(topics); start += size {
		end := start + size
		if end > len(topics) {
			end = len(topics)
		}
		chunk := make([]string, end-start)
		copy(chunk, topics[start:end])
		chunks = append(chunks, chunk)
	}
	return chunks
}

func (m *MarketManager) SubscribeSymbols(reqs ...SymbolSubscriptionRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, raw := range reqs {
		req := normalizeRequest(raw)
		if req.Symbol == "" {
			continue
		}
		existing, ok := m.symbols[req.Symbol]
		if !ok {
			existing = &symbolSubscription{Symbol: req.Symbol}
			m.symbols[req.Symbol] = existing
			log.Printf("adding subscription %s trades=%t ticker=%t book=%t", req.Symbol, req.Trades, req.Ticker, req.Book)
		} else {
			log.Printf("updating subscription %s trades=%t ticker=%t book=%t", req.Symbol, req.Trades, req.Ticker, req.Book)
		}
		existing.Trades = req.Trades
		existing.Ticker = req.Ticker
		existing.Book = req.Book
		existing.Config = mergeConfig(existing.Config, req.Config)
		existing.Paused = false
	}
	m.queueRefreshLocked()
}

func (m *MarketManager) UpdateSymbols(reqs ...SymbolSubscriptionRequest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, raw := range reqs {
		req := normalizeRequest(raw)
		if req.Symbol == "" {
			continue
		}
		if existing, ok := m.symbols[req.Symbol]; ok {
			existing.Trades = req.Trades || existing.Trades
			existing.Ticker = req.Ticker || existing.Ticker
			existing.Book = req.Book || existing.Book
			existing.Config = mergeConfig(existing.Config, req.Config)
			log.Printf("updated config for %s: %+v", req.Symbol, existing.Config)
		}
	}
	m.queueRefreshLocked()
}

func (m *MarketManager) UnsubscribeSymbols(symbols ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sym := range symbols {
		key := normalizeSymbol(sym)
		if key == "" {
			continue
		}
		if _, ok := m.symbols[key]; ok {
			delete(m.symbols, key)
			log.Printf("removed subscription %s", key)
		}
	}
	m.queueRefreshLocked()
}

func (m *MarketManager) PauseSymbols(symbols ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sym := range symbols {
		key := normalizeSymbol(sym)
		if sub, ok := m.symbols[key]; ok && !sub.Paused {
			sub.Paused = true
			log.Printf("paused subscription %s", key)
		}
	}
	m.queueRefreshLocked()
}

func (m *MarketManager) ResumeSymbols(symbols ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, sym := range symbols {
		key := normalizeSymbol(sym)
		if sub, ok := m.symbols[key]; ok && sub.Paused {
			sub.Paused = false
			log.Printf("resumed subscription %s", key)
		}
	}
	m.queueRefreshLocked()
}

func (m *MarketManager) ActiveSubscriptions() []symbolSubscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]symbolSubscription, 0, len(m.symbols))
	for _, sub := range m.symbols {
		out = append(out, *sub)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func (m *MarketManager) SubscriptionConfig(symbol string) (SubscriptionConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if sub, ok := m.symbols[normalizeSymbol(symbol)]; ok {
		return sub.Config, true
	}
	return SubscriptionConfig{}, false
}

func normalizeRequest(req SymbolSubscriptionRequest) SymbolSubscriptionRequest {
	req.Symbol = normalizeSymbol(req.Symbol)
	if !req.Trades && !req.Ticker && !req.Book {
		req.Trades = true
		req.Ticker = true
		req.Book = true
	}
	if req.Config.DepthLevels < 0 {
		req.Config.DepthLevels = 0
	}
	return req
}

func mergeConfig(existing, incoming SubscriptionConfig) SubscriptionConfig {
	result := existing
	if incoming.DepthLevels != 0 {
		result.DepthLevels = incoming.DepthLevels
	}
	if incoming.UpdateInterval != 0 {
		result.UpdateInterval = incoming.UpdateInterval
	}
	return result
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

func (m *MarketManager) handleWebSocketMessages(ctx context.Context, msgCh <-chan core.Message, errCh <-chan error) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgCh:
			if !ok {
				m.handleWSFailure(nil)
				return
			}
			m.handleWSMessage(msg)
		case err, ok := <-errCh:
			if !ok {
				m.handleWSFailure(nil)
				return
			}
			if err != nil {
				m.handleWSFailure(err)
			}
		}
	}
}

func (m *MarketManager) handleWSFailure(err error) {
	if err != nil {
		log.Printf("websocket subscription error: %v", err)
		m.emit(marketEvent{Kind: marketEventError, Message: "websocket subscription error", Err: err})
	} else {
		log.Printf("websocket subscription closed; scheduling resubscription")
	}
	m.mu.Lock()
	m.queueRefreshLocked()
	m.mu.Unlock()
}

func (m *MarketManager) handleOrderBookSnapshots(ctx context.Context, symbol string, cfg SubscriptionConfig, snapshots <-chan corestreams.BookEvent, errs <-chan error) {
	defer m.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case book, ok := <-snapshots:
			if !ok {
				return
			}
			bookCopy := book
			if bookCopy.Symbol == "" {
				bookCopy.Symbol = symbol
			}
			m.emit(marketEvent{
				Kind:    marketEventBook,
				Topic:   coretopics.Book(bookCopy.Symbol),
				Payload: &bookCopy,
				Time:    bookCopy.Time,
				Message: fmt.Sprintf("depth=%d interval=%s", cfg.DepthLevels, cfg.UpdateInterval),
			})
		case err, ok := <-errs:
			if !ok {
				return
			}
			if err != nil {
				m.emit(marketEvent{
					Kind:    marketEventError,
					Message: "order book snapshot error",
					Err:     err,
				})
			}
		}
	}
}

func (m *MarketManager) handleWSMessage(msg core.Message) {
	switch evt := msg.Parsed.(type) {
	case *corestreams.TradeEvent:
		m.emit(marketEvent{
			Kind:    marketEventTrade,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	case *corestreams.TickerEvent:
		m.emit(marketEvent{
			Kind:    marketEventTicker,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	case *corestreams.BookEvent:
		m.emit(marketEvent{
			Kind:    marketEventBook,
			Topic:   msg.Topic,
			Payload: evt,
			Time:    evt.Time,
		})
	default:
		m.emit(marketEvent{
			Kind:    marketEventSystem,
			Topic:   msg.Topic,
			Message: fmt.Sprintf("unhandled message route=%s", msg.Event),
			Time:    msg.At,
		})
	}
}

func (m *MarketManager) eventRouter(ctx context.Context) {
	defer m.wg.Done()

	// This goroutine just ensures we properly close the events channel
	// when the context is cancelled
	<-ctx.Done()
}

func (m *MarketManager) emit(evt marketEvent) {
	if m.ctx != nil {
		select {
		case <-m.ctx.Done():
			return
		default:
		}
	}
	select {
	case m.events <- evt:
	default:
		if m.ctx != nil {
			select {
			case <-m.ctx.Done():
				return
			default:
			}
		}
		// Drop event if channel is full to prevent blocking
		log.Printf("event channel full, dropping event: %v", evt.Kind)
	}
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	fmt.Println("Starting Binance Order Book Management Validation...")

	exchange, err := registry.Resolve(registrybinance.Name)
	if err != nil {
		log.Fatalf("failed to create Binance exchange: %v", err)
	}
	defer exchange.Close()

	canonicalSymbol := core.CanonicalSymbol(defaultBase, defaultQuote)
	nativeSymbol, err := core.NativeSymbol(registrybinance.Name, canonicalSymbol)
	if err != nil {
		log.Printf("failed to resolve native symbol for %s: %v", canonicalSymbol, err)
		nativeSymbol = canonicalSymbol
	}

	fmt.Printf("Monitoring %s depth via exchange pipelines...\n", nativeSymbol)

	manager, err := NewMarketManager(exchange)
	if err != nil {
		log.Fatalf("exchange does not satisfy runtime requirements: %v", err)
	}
	manager.Start(ctx)
	managerStarted := true
	defer func() {
		if managerStarted {
			manager.Stop()
		}
	}()

	instruments := loadInstrumentCache(ctx, exchange)
	formatter := newPrecisionFormatter(instruments)
	processor := newEventProcessor(manager, formatter)

	events := manager.Events()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for evt := range events {
			processor.handle(evt)
		}
		log.Println("market event stream closed")
	}()

	// Subscribe to market data after event processing goroutine is ready
	manager.SubscribeSymbols(buildDefaultSubscriptions(canonicalSymbol, instruments)...)

	<-ctx.Done()
	fmt.Println("Stopping order book monitor...")
	manager.Stop()
	managerStarted = false
	wg.Wait()
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

func resolveEventSymbol(explicit, topic string) string {
	if explicit != "" {
		return explicit
	}
	if topic == "" {
		return ""
	}
	_, symbol, ok := strings.Cut(topic, ":")
	if !ok {
		return ""
	}
	return symbol
}

func loadInstrumentCache(ctx context.Context, exchange core.Exchange) map[string]core.Instrument {
	cache := make(map[string]core.Instrument)
	appendInstruments := func(label string, insts []core.Instrument, err error) {
		if err != nil {
			log.Printf("load %s instruments: %v", label, err)
			return
		}
		for _, inst := range insts {
			cache[inst.Symbol] = inst
		}
	}
	if spot, ok := exchange.(core.SpotParticipant); ok {
		if api := spot.Spot(ctx); api != nil {
			insts, err := api.Instruments(ctx)
			appendInstruments("spot", insts, err)
		}
	}
	if linear, ok := exchange.(core.LinearFuturesParticipant); ok {
		if api := linear.LinearFutures(ctx); api != nil {
			insts, err := api.Instruments(ctx)
			appendInstruments("linear futures", insts, err)
		}
	}
	if inverse, ok := exchange.(core.InverseFuturesParticipant); ok {
		if api := inverse.InverseFutures(ctx); api != nil {
			insts, err := api.Instruments(ctx)
			appendInstruments("inverse futures", insts, err)
		}
	}
	return cache
}

func buildDefaultSubscriptions(primary string, instruments map[string]core.Instrument) []SymbolSubscriptionRequest {
	candidates := []string{
		primary,
		"ETH-USDT",
		"BNB-USDT",
		"XRP-USDT",
		"SOL-USDT",
		"DOGE-USDT",
	}
	seen := make(map[string]struct{})
	requests := make([]SymbolSubscriptionRequest, 0, len(candidates))
	for idx, candidate := range candidates {
		symbol := normalizeSymbol(candidate)
		if symbol == "" {
			continue
		}
		if _, ok := seen[symbol]; ok {
			continue
		}
		seen[symbol] = struct{}{}
		if len(instruments) > 0 {
			if _, ok := instruments[symbol]; !ok {
				continue
			}
		}
		req := SymbolSubscriptionRequest{
			Symbol: symbol,
			Trades: true,
			Ticker: true,
			Config: SubscriptionConfig{UpdateInterval: 500 * time.Millisecond},
		}
		if symbol == primary || idx == 0 {
			req.Book = true
			req.Config.DepthLevels = 2
			req.Config.UpdateInterval = defaultBookLogInterval
		}
		requests = append(requests, req)
	}
	if len(requests) == 0 && primary != "" {
		requests = append(requests, SymbolSubscriptionRequest{Symbol: primary, Trades: true, Ticker: true, Book: true, Config: SubscriptionConfig{DepthLevels: 1, UpdateInterval: defaultBookLogInterval}})
	}
	return requests
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
		scales[symbol] = instrumentPrecision{priceScale: inst.PriceScale, qtyScale: inst.QtyScale}
	}
	return precisionFormatter{scales: scales}
}

func (pf precisionFormatter) price(symbol string, value *big.Rat) string {
	scale := fallbackPriceScale
	if info, ok := pf.scales[symbol]; ok && info.priceScale > 0 {
		scale = info.priceScale
	}
	return formatDecimal(value, scale)
}

func (pf precisionFormatter) quantity(symbol string, value *big.Rat) string {
	scale := fallbackQuantityScale
	if info, ok := pf.scales[symbol]; ok && info.qtyScale > 0 {
		scale = info.qtyScale
	}
	return formatDecimal(value, scale)
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

type eventProcessor struct {
	manager      *MarketManager
	formatter    precisionFormatter
	lastBookLogs map[string]time.Time
}

func newEventProcessor(manager *MarketManager, formatter precisionFormatter) *eventProcessor {
	return &eventProcessor{manager: manager, formatter: formatter, lastBookLogs: make(map[string]time.Time)}
}

func (p *eventProcessor) handle(evt marketEvent) {
	switch evt.Kind {
	case marketEventTrade:
		trade, ok := evt.Payload.(*corestreams.TradeEvent)
		if !ok || trade == nil {
			log.Printf("trade event payload mismatch: %#v", evt.Payload)
			return
		}
		symbol := resolveEventSymbol(trade.Symbol, evt.Topic)
		log.Printf("[WS] trade topic=%s price=%s qty=%s at=%s",
			evt.Topic,
			p.formatter.price(symbol, trade.Price),
			p.formatter.quantity(symbol, trade.Quantity),
			trade.Time.Format(time.RFC3339Nano),
		)
	case marketEventTicker:
		ticker, ok := evt.Payload.(*corestreams.TickerEvent)
		if !ok || ticker == nil {
			log.Printf("ticker event payload mismatch: %#v", evt.Payload)
			return
		}
		symbol := resolveEventSymbol(ticker.Symbol, evt.Topic)
		log.Printf("[WS] ticker topic=%s bid=%s ask=%s at=%s",
			evt.Topic,
			p.formatter.price(symbol, ticker.Bid),
			p.formatter.price(symbol, ticker.Ask),
			ticker.Time.Format(time.RFC3339Nano),
		)
	case marketEventBook:
		book, ok := evt.Payload.(*corestreams.BookEvent)
		if !ok || book == nil {
			log.Printf("book event payload mismatch: %#v", evt.Payload)
			return
		}
		symbol := resolveEventSymbol(book.Symbol, evt.Topic)
		interval := defaultBookLogInterval
		depth := 1
		if cfg, ok := p.manager.SubscriptionConfig(symbol); ok {
			if cfg.UpdateInterval > 0 {
				interval = cfg.UpdateInterval
			}
			if cfg.DepthLevels > 0 {
				depth = cfg.DepthLevels
			}
		}
		last := p.lastBookLogs[symbol]
		if !last.IsZero() && time.Since(last) < interval {
			return
		}
		p.lastBookLogs[symbol] = time.Now()
		bids := topLevels(book.Bids, depth, true)
		asks := topLevels(book.Asks, depth, false)
		log.Printf("[WS] book topic=%s depth=%d bids=%d asks=%d bidLevels=%s askLevels=%s at=%s",
			evt.Topic,
			depth,
			len(book.Bids),
			len(book.Asks),
			formatBookLevels(symbol, bids, p.formatter),
			formatBookLevels(symbol, asks, p.formatter),
			book.Time.Format(time.RFC3339Nano),
		)
	case marketEventError:
		if evt.Err != nil {
			log.Printf("market stream error: %v", evt.Err)
		} else {
			log.Printf("market stream error: %s", evt.Message)
		}
	case marketEventSystem:
		if evt.Message != "" {
			log.Printf("market stream: %s", evt.Message)
		}
	default:
		log.Printf("unhandled market event kind=%s topic=%s", evt.Kind, evt.Topic)
	}
}

func formatBookLevels(symbol string, levels []bookLevel, formatter precisionFormatter) string {
	if len(levels) == 0 {
		return "-"
	}
	parts := make([]string, len(levels))
	for i, level := range levels {
		parts[i] = fmt.Sprintf("%s@%s", formatter.quantity(symbol, level.qty), formatter.price(symbol, level.price))
	}
	return strings.Join(parts, " | ")
}
