package mock

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	"github.com/coachpo/meltica/pipeline"
)

const (
	defaultBufferSize = 16
	restFeedName      = "rest"
)

type config struct {
	protocol     string
	exchangeCaps core.ExchangeCapabilities
	pipelineCaps pipeline.Capabilities
	privateFeeds []string
	buffer       int
}

// Option customizes the sandbox exchange and adapter produced by NewExchange.
type Option func(*config)

// WithCapabilities overrides the exchange capability bitset.
func WithCapabilities(caps ...core.Capability) Option {
	return func(cfg *config) {
		if len(caps) > 0 {
			cfg.exchangeCaps = core.Capabilities(caps...)
		}
	}
}

// WithProtocolVersion overrides the protocol version reported by the exchange.
func WithProtocolVersion(version string) Option {
	return func(cfg *config) {
		trimmed := strings.TrimSpace(version)
		if trimmed != "" {
			cfg.protocol = trimmed
		}
	}
}

// WithPipelineCapabilities configures the Level-4 adapter capability flags.
func WithPipelineCapabilities(caps pipeline.Capabilities) Option {
	return func(cfg *config) {
		cfg.pipelineCaps = caps
	}
}

// WithPrivateFeeds defines the private stream buckets exposed by the adapter.
func WithPrivateFeeds(feeds ...string) Option {
	return func(cfg *config) {
		unique := make([]string, 0, len(feeds))
		seen := make(map[string]struct{}, len(feeds))
		for _, feed := range feeds {
			trimmed := strings.TrimSpace(feed)
			if trimmed == "" {
				continue
			}
			key := strings.ToLower(trimmed)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			unique = append(unique, trimmed)
		}
		if len(unique) > 0 {
			cfg.privateFeeds = unique
		}
	}
}

// WithChannelBuffer sets the channel buffer size used by the adapter feeds.
func WithChannelBuffer(size int) Option {
	return func(cfg *config) {
		if size > 0 {
			cfg.buffer = size
		}
	}
}

// Exchange is a sandbox exchange implementation paired with an in-memory adapter.
type Exchange struct {
	name     core.ExchangeName
	protocol string
	caps     core.ExchangeCapabilities
	adapter  *Adapter
}

// Adapter is an in-memory implementation of pipeline.Adapter for regression tests.
type Adapter struct {
	name    core.ExchangeName
	caps    pipeline.Capabilities
	buffer  int
	mu      sync.Mutex
	books   map[string]*bookFeed
	trades  map[string]*tradeFeed
	tickers map[string]*tickerFeed
	privs   map[string]*privateFeed
	order   []string
	rest    map[string]pipeline.Event
	closed  bool
}

type bookFeed struct {
	events chan corestreams.BookEvent
	errors chan error
}

type tradeFeed struct {
	events chan corestreams.TradeEvent
	errors chan error
}

type tickerFeed struct {
	events chan corestreams.TickerEvent
	errors chan error
}

type privateFeed struct {
	name   string
	events chan pipeline.Event
	errors chan error
}

// NewExchange constructs a sandbox exchange and companion adapter for tests.
func NewExchange(name core.ExchangeName, opts ...Option) (*Exchange, *Adapter) {
	cfg := config{
		protocol: core.ProtocolVersion,
		exchangeCaps: core.Capabilities(
			core.CapabilitySpotPublicREST,
			core.CapabilitySpotTradingREST,
			core.CapabilityMarketTrades,
			core.CapabilityMarketTicker,
			core.CapabilityMarketOrderBook,
			core.CapabilityWebsocketPublic,
			core.CapabilityWebsocketPrivate,
		),
		pipelineCaps: pipeline.Capabilities{
			Books:          true,
			Trades:         true,
			Tickers:        true,
			PrivateStreams: true,
			RESTEndpoints:  true,
		},
		privateFeeds: []string{"account", "orders"},
		buffer:       defaultBufferSize,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}

	adapter := newAdapter(name, cfg)
	exchange := &Exchange{
		name:     name,
		protocol: cfg.protocol,
		caps:     cfg.exchangeCaps,
		adapter:  adapter,
	}
	return exchange, adapter
}

// Adapter returns the sandbox adapter instance.
func (e *Exchange) Adapter() *Adapter {
	if e == nil {
		return nil
	}
	return e.adapter
}

func (e *Exchange) Name() string { return string(e.name) }

func (e *Exchange) Capabilities() core.ExchangeCapabilities { return e.caps }

func (e *Exchange) SupportedProtocolVersion() string { return e.protocol }

func (e *Exchange) Close() error {
	if e.adapter != nil {
		e.adapter.Close()
	}
	return nil
}

func newAdapter(name core.ExchangeName, cfg config) *Adapter {
	a := &Adapter{
		name:    name,
		caps:    cfg.pipelineCaps,
		buffer:  cfg.buffer,
		books:   make(map[string]*bookFeed),
		trades:  make(map[string]*tradeFeed),
		tickers: make(map[string]*tickerFeed),
		privs:   make(map[string]*privateFeed),
		rest:    make(map[string]pipeline.Event),
	}
	for _, feed := range cfg.privateFeeds {
		a.ensurePrivateFeed(feed)
	}
	return a
}

// Capabilities reports the adapter feature flags.
func (a *Adapter) Capabilities() pipeline.Capabilities { return a.caps }

// ExchangeName exposes the venue identifier backing the adapter.
func (a *Adapter) ExchangeName() core.ExchangeName { return a.name }

// BookSources exposes channels scoped to the requested symbols.
func (a *Adapter) BookSources(_ context.Context, symbols []string) ([]pipeline.BookSource, error) {
	sources := make([]pipeline.BookSource, 0, len(symbols))
	for _, symbol := range symbols {
		feed := a.ensureBookFeed(symbol)
		sources = append(sources, pipeline.BookSource{
			Symbol: normalizeSymbol(symbol),
			Events: feed.events,
			Errors: feed.errors,
		})
	}
	return sources, nil
}

// TradeSources exposes trade channels for the requested symbols.
func (a *Adapter) TradeSources(_ context.Context, symbols []string) ([]pipeline.TradeSource, error) {
	sources := make([]pipeline.TradeSource, 0, len(symbols))
	for _, symbol := range symbols {
		feed := a.ensureTradeFeed(symbol)
		sources = append(sources, pipeline.TradeSource{
			Symbol: normalizeSymbol(symbol),
			Events: feed.events,
			Errors: feed.errors,
		})
	}
	return sources, nil
}

// TickerSources exposes ticker channels for the requested symbols.
func (a *Adapter) TickerSources(_ context.Context, symbols []string) ([]pipeline.TickerSource, error) {
	sources := make([]pipeline.TickerSource, 0, len(symbols))
	for _, symbol := range symbols {
		feed := a.ensureTickerFeed(symbol)
		sources = append(sources, pipeline.TickerSource{
			Symbol: normalizeSymbol(symbol),
			Events: feed.events,
			Errors: feed.errors,
		})
	}
	return sources, nil
}

// PrivateSources exposes the configured private stream feeds.
func (a *Adapter) PrivateSources(context.Context, *pipeline.AuthContext) ([]pipeline.PrivateSource, error) {
	a.mu.Lock()
	order := append([]string(nil), a.order...)
	feeds := make([]*privateFeed, 0, len(order))
	for _, key := range order {
		if feed, ok := a.privs[key]; ok {
			feeds = append(feeds, feed)
		}
	}
	a.mu.Unlock()

	sources := make([]pipeline.PrivateSource, 0, len(feeds))
	for _, feed := range feeds {
		sources = append(sources, pipeline.PrivateSource{
			Events: feed.events,
			Errors: feed.errors,
		})
	}
	return sources, nil
}

// ExecuteREST emits the configured REST response or a default payload.
func (a *Adapter) ExecuteREST(_ context.Context, req pipeline.InteractionRequest) (<-chan pipeline.Event, <-chan error, error) {
	events := make(chan pipeline.Event, 1)
	errs := make(chan error, 1)

	key := requestKey(req)

	a.mu.Lock()
	resp, ok := a.rest[key]
	if ok {
		delete(a.rest, key)
	}
	if !ok && len(a.rest) > 0 {
		for alias, event := range a.rest {
			resp = event
			delete(a.rest, alias)
			ok = true
			break
		}
	}
	a.mu.Unlock()

	if !ok {
		resp = pipeline.Event{
			Transport:     pipeline.TransportREST,
			Symbol:        normalizeSymbol(req.Symbol),
			At:            time.Now(),
			CorrelationID: req.CorrelationID,
			Payload: pipeline.RestResponsePayload{
				Response: &pipeline.RestResponse{
					RequestID:  req.CorrelationID,
					Method:     req.Method,
					Path:       req.Path,
					StatusCode: 200,
				},
			},
			Metadata: map[string]any{
				"source.feed":   restFeedName,
				"source.symbol": normalizeSymbol(req.Symbol),
			},
		}
	}

	events <- resp
	close(events)
	close(errs)
	return events, errs, nil
}

// InitPrivateSession is a no-op for the sandbox adapter.
func (a *Adapter) InitPrivateSession(context.Context, *pipeline.AuthContext) error { return nil }

// Close releases all feed channels.
func (a *Adapter) Close() {
	a.mu.Lock()
	if a.closed {
		a.mu.Unlock()
		return
	}
	a.closed = true

	bookFeeds := make([]*bookFeed, 0, len(a.books))
	for _, feed := range a.books {
		bookFeeds = append(bookFeeds, feed)
	}
	tradeFeeds := make([]*tradeFeed, 0, len(a.trades))
	for _, feed := range a.trades {
		tradeFeeds = append(tradeFeeds, feed)
	}
	tickerFeeds := make([]*tickerFeed, 0, len(a.tickers))
	for _, feed := range a.tickers {
		tickerFeeds = append(tickerFeeds, feed)
	}
	privateFeeds := make([]*privateFeed, 0, len(a.privs))
	for _, feed := range a.privs {
		privateFeeds = append(privateFeeds, feed)
	}

	a.books = nil
	a.trades = nil
	a.tickers = nil
	a.privs = nil
	a.rest = nil
	a.order = nil
	a.mu.Unlock()

	for _, feed := range bookFeeds {
		close(feed.events)
		close(feed.errors)
	}
	for _, feed := range tradeFeeds {
		close(feed.events)
		close(feed.errors)
	}
	for _, feed := range tickerFeeds {
		close(feed.events)
		close(feed.errors)
	}
	for _, feed := range privateFeeds {
		close(feed.events)
		close(feed.errors)
	}
}

// PublishBook pushes a book event onto the mock feed.
func (a *Adapter) PublishBook(symbol string, evt corestreams.BookEvent) {
	feed := a.ensureBookFeed(symbol)
	feed.events <- evt
}

// PublishTrade pushes a trade event onto the mock feed.
func (a *Adapter) PublishTrade(symbol string, evt corestreams.TradeEvent) {
	feed := a.ensureTradeFeed(symbol)
	feed.events <- evt
}

// PublishTicker pushes a ticker event onto the mock feed.
func (a *Adapter) PublishTicker(symbol string, evt corestreams.TickerEvent) {
	feed := a.ensureTickerFeed(symbol)
	feed.events <- evt
}

// PublishPrivate pushes a private event onto the mock feed, defaulting metadata where absent.
func (a *Adapter) PublishPrivate(feedName string, evt pipeline.Event) {
	feed := a.ensurePrivateFeed(feedName)
	if evt.Transport == pipeline.TransportUnknown {
		evt.Transport = pipeline.TransportPrivateWS
	}
	if evt.At.IsZero() {
		evt.At = time.Now()
	}
	if evt.Metadata == nil {
		evt.Metadata = make(map[string]any)
	} else {
		evt.Metadata = cloneMetadata(evt.Metadata)
	}
	if _, ok := evt.Metadata["source.feed"]; !ok {
		evt.Metadata["source.feed"] = feed.name
	}
	if evt.Symbol != "" {
		if _, ok := evt.Metadata["source.symbol"]; !ok {
			evt.Metadata["source.symbol"] = evt.Symbol
		}
	}
	feed.events <- evt
}

// SetRESTResponse configures the next REST response for the provided correlation ID.
func (a *Adapter) SetRESTResponse(correlationID string, evt pipeline.Event) {
	key := strings.TrimSpace(correlationID)
	if key == "" {
		key = strings.TrimSpace(evt.CorrelationID)
	}
	if key == "" {
		return
	}
	if evt.Transport == pipeline.TransportUnknown {
		evt.Transport = pipeline.TransportREST
	}
	if evt.At.IsZero() {
		evt.At = time.Now()
	}
	if evt.Metadata == nil {
		evt.Metadata = map[string]any{}
	} else {
		evt.Metadata = cloneMetadata(evt.Metadata)
	}
	if _, ok := evt.Metadata["source.feed"]; !ok {
		evt.Metadata["source.feed"] = restFeedName
	}
	if evt.CorrelationID == "" {
		evt.CorrelationID = correlationID
	}

	a.mu.Lock()
	if a.rest == nil {
		a.rest = make(map[string]pipeline.Event)
	}
	a.rest[key] = evt
	a.mu.Unlock()
}

func (a *Adapter) ensureBookFeed(symbol string) *bookFeed {
	key := normalizeSymbol(symbol)
	a.mu.Lock()
	defer a.mu.Unlock()
	if feed, ok := a.books[key]; ok {
		return feed
	}
	feed := &bookFeed{
		events: make(chan corestreams.BookEvent, a.buffer),
		errors: make(chan error, a.buffer),
	}
	a.books[key] = feed
	return feed
}

func (a *Adapter) ensureTradeFeed(symbol string) *tradeFeed {
	key := normalizeSymbol(symbol)
	a.mu.Lock()
	defer a.mu.Unlock()
	if feed, ok := a.trades[key]; ok {
		return feed
	}
	feed := &tradeFeed{
		events: make(chan corestreams.TradeEvent, a.buffer),
		errors: make(chan error, a.buffer),
	}
	a.trades[key] = feed
	return feed
}

func (a *Adapter) ensureTickerFeed(symbol string) *tickerFeed {
	key := normalizeSymbol(symbol)
	a.mu.Lock()
	defer a.mu.Unlock()
	if feed, ok := a.tickers[key]; ok {
		return feed
	}
	feed := &tickerFeed{
		events: make(chan corestreams.TickerEvent, a.buffer),
		errors: make(chan error, a.buffer),
	}
	a.tickers[key] = feed
	return feed
}

func (a *Adapter) ensurePrivateFeed(name string) *privateFeed {
	label := name
	if strings.TrimSpace(label) == "" {
		label = "private"
	}
	key := strings.ToLower(strings.TrimSpace(label))
	a.mu.Lock()
	defer a.mu.Unlock()
	if feed, ok := a.privs[key]; ok {
		return feed
	}
	feed := &privateFeed{
		name:   label,
		events: make(chan pipeline.Event, a.buffer),
		errors: make(chan error, a.buffer),
	}
	a.privs[key] = feed
	a.order = append(a.order, key)
	return feed
}

func requestKey(req pipeline.InteractionRequest) string {
	if trimmed := strings.TrimSpace(req.CorrelationID); trimmed != "" {
		return trimmed
	}
	method := strings.ToUpper(strings.TrimSpace(req.Method))
	path := strings.TrimSpace(req.Path)
	if method == "" && path == "" {
		return restFeedName
	}
	return method + " " + path
}

func normalizeSymbol(symbol string) string {
	trimmed := strings.TrimSpace(symbol)
	if trimmed == "" {
		return ""
	}
	return strings.ToUpper(trimmed)
}

func cloneMetadata(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
