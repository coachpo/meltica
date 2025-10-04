package exchange

import (
	"context"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	numeric "github.com/coachpo/meltica/exchanges/shared/infra/numeric"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

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

func (x *Exchange) ensureMarketSymbols(ctx context.Context, market core.Market) error {
	if x.symbols.isLoaded(market) {
		return nil
	}
	spec, ok := marketSpecFor(market)
	if !ok {
		return internal.Exchange("symbol loader: unsupported market %s", market)
	}

	x.symbolsMu.Lock()
	defer x.symbolsMu.Unlock()
	if x.symbols.isLoaded(market) {
		return nil
	}

	loaderCtx, cancel := normalizedContext(ctx, symbolLoadTimeout)
	defer cancel()

	payload, err := x.fetchMarketSymbols(loaderCtx, spec)
	if err != nil {
		return err
	}
	x.recordMarketSymbols(spec.market, payload)
	return nil
}

func (x *Exchange) ensureAllSymbols(ctx context.Context) error {
	for _, market := range []core.Market{core.MarketSpot, core.MarketLinearFutures, core.MarketInverseFutures} {
		if err := x.ensureMarketSymbols(ctx, market); err != nil {
			return err
		}
	}
	return nil
}

func (x *Exchange) fetchMarketSymbols(ctx context.Context, spec marketSpec) ([]symbolPayload, error) {
	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Base    string `json:"baseAsset"`
			Quote   string `json:"quoteAsset"`
			Status  string `json:"status"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	msg := routingrest.RESTMessage{API: string(spec.api), Method: http.MethodGet, Path: spec.path}
	if err := x.restRouter.Dispatch(ctx, msg, &resp); err != nil {
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
		priceScale, qtyScale := 0, 0
		for _, ft := range sdef.Filters {
			switch ft.FilterType {
			case "PRICE_FILTER":
				priceScale = numeric.ScaleFromStep(ft.TickSize)
			case "LOT_SIZE":
				qtyScale = numeric.ScaleFromStep(ft.StepSize)
			}
		}
		inst := core.Instrument{Symbol: core.CanonicalSymbol(base, quote), Base: base, Quote: quote, Market: spec.market, PriceScale: priceScale, QtyScale: qtyScale}
		payload = append(payload, symbolPayload{instrument: inst, native: native})
	}
	return payload, nil
}

func (x *Exchange) recordMarketSymbols(market core.Market, payload []symbolPayload) {
	entries := make([]symbolEntry, 0, len(payload))
	cache := make(map[string]core.Instrument, len(payload))
	for _, item := range payload {
		entries = append(entries, symbolEntry{canonical: item.instrument.Symbol, native: item.native})
		cache[item.instrument.Symbol] = item.instrument
	}
	x.symbols.replace(market, entries)
	if x.instCache == nil {
		x.instCache = make(map[core.Market]map[string]core.Instrument)
	}
	x.instCache[market] = cache
}

func (x *Exchange) instrumentsForMarket(market core.Market) []core.Instrument {
	x.symbolsMu.RLock()
	defer x.symbolsMu.RUnlock()
	cache := x.instCache[market]
	out := make([]core.Instrument, 0, len(cache))
	for _, inst := range cache {
		out = append(out, inst)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

func (x *Exchange) instrument(market core.Market, symbol string) (core.Instrument, bool) {
	x.symbolsMu.RLock()
	defer x.symbolsMu.RUnlock()
	if marketCache, ok := x.instCache[market]; ok {
		inst, ok := marketCache[symbol]
		return inst, ok
	}
	return core.Instrument{}, false
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
