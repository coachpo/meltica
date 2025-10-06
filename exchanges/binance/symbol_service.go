package binance

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type symbolService struct {
	router   routingrest.RESTDispatcher
	registry *symbolRegistry
	cache    map[core.Market]map[string]core.Instrument
	mu       sync.RWMutex
}

type marketSpec struct {
	api    rest.API
	path   string
	market core.Market
}

type symbolPayload struct {
	instrument core.Instrument
	native     string
}

const symbolLoadTimeout = 5 * time.Second

func newSymbolService(router routingrest.RESTDispatcher) *symbolService {
	return &symbolService{
		router:   router,
		registry: newSymbolRegistry(),
		cache:    make(map[core.Market]map[string]core.Instrument),
	}
}

func (s *symbolService) swapRouter(router routingrest.RESTDispatcher) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.router = router
	s.registry = newSymbolRegistry()
	s.cache = make(map[core.Market]map[string]core.Instrument)
}

func (s *symbolService) ensureMarket(ctx context.Context, market core.Market) error {
	if s.registry.isLoaded(market) {
		return nil
	}
	spec, ok := marketSpecFor(market)
	if !ok {
		return internal.Exchange("symbol loader: unsupported market %s", market)
	}

	s.mu.RLock()
	router := s.router
	s.mu.RUnlock()
	if router == nil {
		return internal.Exchange("symbol loader: rest router unavailable")
	}

	loaderCtx, cancel := normalizedContext(ctx, symbolLoadTimeout)
	defer cancel()

	payload, err := s.fetchMarketSymbols(loaderCtx, spec, router)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.registry.isLoaded(market) {
		return nil
	}
	s.recordMarketSymbolsLocked(market, payload)
	return nil
}

func (s *symbolService) instruments(ctx context.Context, market core.Market) ([]core.Instrument, error) {
	if err := s.ensureMarket(ctx, market); err != nil {
		return nil, err
	}
	s.mu.RLock()
	cache := s.cache[market]
	s.mu.RUnlock()
	out := make([]core.Instrument, 0, len(cache))
	for _, inst := range cache {
		out = append(out, inst)
	}
	sortInstruments(out)
	return out, nil
}

func (s *symbolService) instrument(ctx context.Context, market core.Market, symbol string) (core.Instrument, bool, error) {
	if err := s.ensureMarket(ctx, market); err != nil {
		return core.Instrument{}, false, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if cache, ok := s.cache[market]; ok {
		inst, ok := cache[symbol]
		return inst, ok, nil
	}
	return core.Instrument{}, false, nil
}

func (s *symbolService) nativeForMarkets(ctx context.Context, canonical string, markets ...core.Market) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(canonical))
	if trimmed == "" {
		return "", internal.Invalid("unsupported symbol %s", canonical)
	}
	targetMarkets := marketsOrAll(markets...)
	for _, market := range targetMarkets {
		if err := s.ensureMarket(ctx, market); err != nil {
			return "", err
		}
		if native, ok := s.registry.native(market, trimmed); ok {
			return native, nil
		}
	}
	if len(markets) == 0 {
		if native, ok := s.registry.nativeAny(trimmed); ok {
			return native, nil
		}
	}
	return "", internal.Invalid("unsupported symbol %s", trimmed)
}

func (s *symbolService) canonicalForMarkets(ctx context.Context, binanceSymbol string, markets ...core.Market) (string, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if trimmed == "" {
		return "", internal.Invalid("unsupported symbol %s", binanceSymbol)
	}
	targetMarkets := marketsOrAll(markets...)
	belongsToTarget := func(canonical string) bool {
		if len(markets) == 0 {
			return true
		}
		for _, market := range targetMarkets {
			if _, ok := s.registry.native(market, canonical); ok {
				return true
			}
		}
		return false
	}
	if canonical, ok := s.registry.canonical(trimmed); ok && belongsToTarget(canonical) {
		return canonical, nil
	}
	for _, market := range targetMarkets {
		if err := s.ensureMarket(ctx, market); err != nil {
			return "", err
		}
		if canonical, ok := s.registry.canonical(trimmed); ok && belongsToTarget(canonical) {
			return canonical, nil
		}
	}
	return "", internal.Exchange("unsupported symbol %s", trimmed)
}

func (s *symbolService) snapshot(ctx context.Context) (registrySnapshot, error) {
	for _, market := range marketsOrAll() {
		if err := s.ensureMarket(ctx, market); err != nil {
			return registrySnapshot{}, err
		}
	}
	return s.registry.snapshot(), nil
}

// Refresh reloads symbols for specified markets (or all markets if none specified).
// It takes a write lock, clears the selected markets, and repopulates them.
// If reload fails, it restores from a saved snapshot and returns the error.
func (s *symbolService) Refresh(ctx context.Context, markets ...core.Market) error {
	targetMarkets := marketsOrAll(markets...)

	// Take a snapshot of current state for rollback
	s.mu.RLock()
	registrySnap := s.registry.snapshot()
	cacheSnap := make(map[core.Market]map[string]core.Instrument, len(s.cache))
	for market, instruments := range s.cache {
		instCopy := make(map[string]core.Instrument, len(instruments))
		for k, v := range instruments {
			instCopy[k] = v
		}
		cacheSnap[market] = instCopy
	}
	loadedSnap := make(map[core.Market]bool)
	s.registry.mu.RLock()
	for market, status := range s.registry.loaded {
		loadedSnap[market] = status
	}
	s.registry.mu.RUnlock()
	s.mu.RUnlock()

	// Clear the target markets
	s.mu.Lock()
	s.registry.mu.Lock()
	for _, market := range targetMarkets {
		// Clear cache
		delete(s.cache, market)
		// Clear registry entries
		if marketMap, ok := s.registry.marketCanonicalMap[market]; ok {
			for _, native := range marketMap {
				delete(s.registry.nativeToCanonical, native)
			}
			delete(s.registry.marketCanonicalMap, market)
		}
		// Mark as not loaded
		s.registry.loaded[market] = false
	}
	s.registry.mu.Unlock()
	s.mu.Unlock()

	// Attempt to reload each market
	for _, market := range targetMarkets {
		if err := s.ensureMarket(ctx, market); err != nil {
			// Rollback on failure
			s.mu.Lock()
			s.registry.mu.Lock()
			s.registry.nativeToCanonical = registrySnap.nativeToCanonical
			s.registry.marketCanonicalMap = registrySnap.marketCanonicalMap
			s.registry.loaded = loadedSnap
			s.registry.mu.Unlock()
			s.cache = cacheSnap
			s.mu.Unlock()
			return err
		}
	}

	return nil
}

func (s *symbolService) fetchMarketSymbols(ctx context.Context, spec marketSpec, router routingrest.RESTDispatcher) ([]symbolPayload, error) {
	var resp struct {
		Symbols []struct {
			Symbol              string `json:"symbol"`
			Base                string `json:"baseAsset"`
			Quote               string `json:"quoteAsset"`
			Status              string `json:"status"`
			BasePrecision       int    `json:"baseAssetPrecision"`
			QuoteAssetPrecision int    `json:"quoteAssetPrecision"`
			QuotePrecision      int    `json:"quotePrecision"`
			Filters             []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	msg := routingrest.RESTMessage{API: string(spec.api), Method: http.MethodGet, Path: spec.path, Query: map[string]string{"symbolStatus": "TRADING"}}
	if err := router.Dispatch(ctx, msg, &resp); err != nil {
		return nil, err
	}

	payload := make([]symbolPayload, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		status := strings.ToUpper(strings.TrimSpace(sdef.Status))
		if status != "" && status != "TRADING" {
			continue
		}
		base := strings.ToUpper(strings.TrimSpace(sdef.Base))
		quote := strings.ToUpper(strings.TrimSpace(sdef.Quote))
		native := strings.ToUpper(strings.TrimSpace(sdef.Symbol))
		if base == "" || quote == "" || native == "" {
			continue
		}
		priceScale := maxInt(sdef.QuoteAssetPrecision, sdef.QuotePrecision)
		qtyScale := sdef.BasePrecision
		for _, ft := range sdef.Filters {
			switch ft.FilterType {
			case "PRICE_FILTER":
				scale := numeric.ScaleFromStep(ft.TickSize)
				priceScale = maxInt(priceScale, scale)
			case "LOT_SIZE":
				scale := numeric.ScaleFromStep(ft.StepSize)
				qtyScale = maxInt(qtyScale, scale)
			}
		}
		inst := core.Instrument{Symbol: core.CanonicalSymbol(base, quote), Base: base, Quote: quote, Market: spec.market, PriceScale: priceScale, QtyScale: qtyScale}
		payload = append(payload, symbolPayload{instrument: inst, native: native})
	}
	return payload, nil
}

func (s *symbolService) recordMarketSymbolsLocked(market core.Market, payload []symbolPayload) {
	entries := make([]symbolEntry, 0, len(payload))
	cache := make(map[string]core.Instrument, len(payload))
	for _, item := range payload {
		entries = append(entries, symbolEntry{canonical: item.instrument.Symbol, native: item.native})
		cache[item.instrument.Symbol] = item.instrument
	}
	s.registry.replace(market, entries)
	if s.cache == nil {
		s.cache = make(map[core.Market]map[string]core.Instrument)
	}
	s.cache[market] = cache
}

func sortInstruments(instruments []core.Instrument) {
	sort.Slice(instruments, func(i, j int) bool { return instruments[i].Symbol < instruments[j].Symbol })
}

func marketSpecFor(market core.Market) (marketSpec, bool) {
	switch market {
	case core.MarketSpot:
		return marketSpec{api: rest.SpotAPI, path: "/api/v3/exchangeInfo", market: core.MarketSpot}, true
	case core.MarketLinearFutures:
		return marketSpec{api: rest.LinearAPI, path: "/fapi/v1/exchangeInfo", market: core.MarketLinearFutures}, true
	case core.MarketInverseFutures:
		return marketSpec{api: rest.InverseAPI, path: "/dapi/v1/exchangeInfo", market: core.MarketInverseFutures}, true
	default:
		return marketSpec{}, false
	}
}

func normalizedContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
