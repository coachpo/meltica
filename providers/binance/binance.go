package binance

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	corews "github.com/coachpo/meltica/core/ws"
	"github.com/coachpo/meltica/providers/binance/ws"
	"github.com/coachpo/meltica/transport"
)

type Provider struct {
	name       string
	sapi       *transport.Client // Spot API (SAPI): https://api.binance.com - Spot + account/user operations
	fapi       *transport.Client // Futures API (FAPI): https://fapi.binance.com - USD-stablecoins-M Futures
	dapi       *transport.Client // Delivery API (DAPI): https://dapi.binance.com - Coin-margined Futures
	apiKey     string
	secret     string
	instCache  map[string]core.Instrument
	binToCanon map[string]string
}

var capabilities = core.Capabilities(
	core.CapabilitySpotPublicREST,
	core.CapabilitySpotTradingREST,
	core.CapabilityLinearPublicREST,
	core.CapabilityLinearTradingREST,
	core.CapabilityInversePublicREST,
	core.CapabilityInverseTradingREST,
	core.CapabilityWebsocketPublic,
	core.CapabilityWebsocketPrivate,
)

func New(apiKey, secret string) (*Provider, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	// SAPI: Spot API - https://api.binance.com (Spot + account/user operations)
	sapi := &transport.Client{HTTP: httpClient, BaseURL: "https://api.binance.com"}
	// FAPI: Futures API - https://fapi.binance.com (USD-stablecoins-M Futures)
	fapi := &transport.Client{HTTP: httpClient, BaseURL: "https://fapi.binance.com"}
	// DAPI: Delivery API - https://dapi.binance.com (Coin-margined Futures)
	dapi := &transport.Client{HTTP: httpClient, BaseURL: "https://dapi.binance.com"}
	p := &Provider{name: "binance", sapi: sapi, fapi: fapi, dapi: dapi, apiKey: apiKey, secret: secret}
	signer := func(method, path string, q map[string]string, body []byte, ts int64) (http.Header, error) {
		hdr, err := signHMAC(p.secret, method, path, q, body, ts)
		if hdr == nil {
			hdr = http.Header{}
		}
		if p.apiKey != "" {
			hdr.Set("X-MBX-APIKEY", p.apiKey)
		}
		return hdr, err
	}
	sapi.Signer = signer
	fapi.Signer = signer
	dapi.Signer = signer
	// Attach error mapper for REST
	sapi.OnHTTPError = mapHTTPError
	fapi.OnHTTPError = mapHTTPError
	dapi.OnHTTPError = mapHTTPError
	return p, nil
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Capabilities() core.ProviderCapabilities { return capabilities }

func (p *Provider) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (p *Provider) Spot(ctx context.Context) core.SpotAPI              { return spotAPI{p} }
func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI  { return fapi{p} }
func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI { return dapi{p} }
func (p *Provider) WS() core.WS                                        { return ws.New(p) }
func (p *Provider) Close() error                                       { return nil }

// WebSocket support methods
func (p *Provider) CanonicalSymbol(binanceSymbol string) string {
	s := strings.ToUpper(strings.TrimSpace(binanceSymbol))
	if s == "" {
		fmt.Fprintf(os.Stderr, "ERROR: binance: empty symbol in WSCanonicalSymbol\n")
		panic("binance: empty symbol in WSCanonicalSymbol")
	}

	// Check if we have a mapping for this symbol
	if p.binToCanon != nil {
		if canonical, exists := p.binToCanon[s]; exists {
			return canonical
		}
	}

	// Fallback for common symbols
	if s == "BTCUSDT" {
		return "BTC-USDT"
	}

	// For unknown symbols, try to parse them
	// Binance symbols are typically BASEQUOTE (e.g., ETHUSDT, ADAUSDC)
	// Try to split into base and quote
	for i := len(s) - 4; i >= 3; i-- {
		if s[i:] == "USDT" || s[i:] == "BUSD" || s[i:] == "USDC" {
			base := s[:i]
			quote := s[i:]
			return fmt.Sprintf("%s-%s", base, quote)
		}
	}

	// If we can't parse it, print the abnormal symbol and panic
	fmt.Fprintf(os.Stderr, "ERROR: binance: unsupported symbol '%s'\n", s)
	panic(fmt.Errorf("binance: unsupported symbol %s", s))
}

func (p *Provider) CreateListenKey(ctx context.Context) (string, error) {
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	if err := p.sapi.Do(ctx, http.MethodPost, "/api/v3/userDataStream", nil, nil, false, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (p *Provider) KeepAliveListenKey(ctx context.Context, key string) error {
	q := map[string]string{"listenKey": key}
	return p.sapi.Do(ctx, http.MethodPut, "/api/v3/userDataStream", q, nil, false, nil)
}

func (p *Provider) CloseListenKey(ctx context.Context, key string) error {
	q := map[string]string{"listenKey": key}
	return p.sapi.Do(ctx, http.MethodDelete, "/api/v3/userDataStream", q, nil, false, nil)
}

type spotAPI struct{ p *Provider }

type fapi struct{ p *Provider }

type dapi struct{ p *Provider }

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var resp struct {
		ServerTime int64 `json:"serverTime"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/time", nil, nil, false, &resp); err != nil {
		return time.Time{}, err
	}
	return time.UnixMilli(resp.ServerTime), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Base    string `json:"baseAsset"`
			Quote   string `json:"quoteAsset"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/exchangeInfo", nil, nil, false, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		var priceScale, qtyScale int
		for _, f := range sdef.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				priceScale = scaleFromStep(f.TickSize)
			case "LOT_SIZE":
				qtyScale = scaleFromStep(f.StepSize)
			}
		}
		sym := core.CanonicalSymbol(sdef.Base, sdef.Quote)
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketSpot, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		if s.p.instCache == nil {
			s.p.instCache = map[string]core.Instrument{}
		}
		s.p.instCache[sym] = inst
		if s.p.binToCanon == nil {
			s.p.binToCanon = map[string]string{}
		}
		s.p.binToCanon[sdef.Symbol] = sym
	}
	return out, nil
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp struct {
		Bid string `json:"bidPrice"`
		Ask string `json:"askPrice"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/ticker/bookTicker", map[string]string{"symbol": core.CanonicalToBinance(symbol)}, nil, false, &resp); err != nil {
		return core.Ticker{}, err
	}
	bid, _ := parseDecimalToRat(resp.Bid)
	ask, _ := parseDecimalToRat(resp.Ask)
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp []struct {
		Asset  string `json:"asset"`
		Free   string `json:"free"`
		Locked string `json:"locked"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/account", nil, nil, true, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Balance, 0, len(resp))
	for _, b := range resp {
		avail, _ := parseDecimalToRat(b.Free)
		out = append(out, core.Balance{Asset: b.Asset, Available: avail, Time: time.Now()})
	}
	return out, nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	params := map[string]string{"symbol": core.CanonicalToBinance(symbol)}
	if since > 0 {
		params["startTime"] = fmt.Sprintf("%d", since)
	}
	var resp []struct {
		ID      int64  `json:"id"`
		Price   string `json:"price"`
		Qty     string `json:"qty"`
		IsBuyer bool   `json:"isBuyer"`
		Time    int64  `json:"time"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/myTrades", params, nil, true, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Trade, 0, len(resp))
	for _, tr := range resp {
		price, _ := parseDecimalToRat(tr.Price)
		qty, _ := parseDecimalToRat(tr.Qty)
		side := core.SideSell
		if tr.IsBuyer {
			side = core.SideBuy
		}
		out = append(out, core.Trade{Symbol: symbol, ID: fmt.Sprintf("%d", tr.ID), Price: price, Quantity: qty, Side: side, Time: time.UnixMilli(tr.Time)})
	}
	return out, nil
}
func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	// Basic signed POST /api/v3/order (MVP: minimal fields)
	// Canonical symbol -> Binance symbol
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if req.ClientID != "" {
		q["newClientOrderId"] = req.ClientID
	}
	// decimal formatting per instrument scales
	if req.Quantity != nil || req.Price != nil {
		// ensure cache
		if s.p.instCache == nil {
			if insts, err := s.Instruments(ctx); err == nil {
				_ = insts
			}
		}
		inst := s.p.instCache[req.Symbol]
		if req.Quantity != nil {
			q["quantity"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
		}
		if req.Price != nil {
			q["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
		}
	}
	var resp struct {
		OrderId int64  `json:"orderId"`
		Symbol  string `json:"symbol"`
		Status  string `json:"status"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodPost, "/api/v3/order", q, nil, true, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderId), Symbol: req.Symbol, Status: mapBStatus(resp.Status)}, nil
}
func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	q := map[string]string{"symbol": symbol}
	if id != "" {
		q["orderId"] = id
	}
	if clientID != "" {
		q["origClientOrderId"] = clientID
	}
	var resp struct {
		OrderId int64  `json:"orderId"`
		Symbol  string `json:"symbol"`
		Status  string `json:"status"`
	}
	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/order", q, nil, true, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderId), Symbol: symbol, Status: mapBStatus(resp.Status)}, nil
}
func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	q := map[string]string{"symbol": symbol}
	if id != "" {
		q["orderId"] = id
	}
	if clientID != "" {
		q["origClientOrderId"] = clientID
	}
	return s.p.sapi.Do(ctx, http.MethodDelete, "/api/v3/order", q, nil, true, nil)
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (s spotAPI) SpotNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance spotAPI: unsupported canonical symbol %s", canonical))
}

func (s spotAPI) SpotCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance spotAPI: unsupported native symbol %s", native))
}

// DepthSnapshot fetches the current order book snapshot for a symbol
func (s spotAPI) DepthSnapshot(ctx context.Context, symbol string, limit int) (corews.BookEvent, int64, error) {
	params := map[string]string{
		"symbol": core.CanonicalToBinance(symbol),
		"limit":  fmt.Sprintf("%d", limit),
	}

	var resp struct {
		LastUpdateID int64           `json:"lastUpdateId"`
		Bids         [][]interface{} `json:"bids"`
		Asks         [][]interface{} `json:"asks"`
	}

	if err := s.p.sapi.Do(ctx, http.MethodGet, "/api/v3/depth", params, nil, false, &resp); err != nil {
		return corews.BookEvent{}, 0, err
	}

	// Parse depth levels
	bids := depthLevelsFromPairs(resp.Bids)
	asks := depthLevelsFromPairs(resp.Asks)

	bookEvent := corews.BookEvent{
		Symbol: symbol,
		Bids:   bids,
		Asks:   asks,
		Time:   time.Now(),
	}

	return bookEvent, resp.LastUpdateID, nil
}

func (f fapi) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Base    string `json:"baseAsset"`
			Quote   string `json:"quoteAsset"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := f.p.fapi.Do(ctx, http.MethodGet, "/fapi/v1/exchangeInfo", nil, nil, false, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		var priceScale, qtyScale int
		for _, ft := range sdef.Filters {
			switch ft.FilterType {
			case "PRICE_FILTER":
				priceScale = scaleFromStep(ft.TickSize)
			case "LOT_SIZE":
				qtyScale = scaleFromStep(ft.StepSize)
			}
		}
		sym := core.CanonicalSymbol(sdef.Base, sdef.Quote)
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketLinearFutures, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		if f.p.instCache == nil {
			f.p.instCache = map[string]core.Instrument{}
		}
		f.p.instCache[sym] = inst
	}
	return out, nil
}
func (f fapi) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	if err := f.p.fapi.Do(ctx, http.MethodGet, "/fapi/v1/ticker/bookTicker", map[string]string{"symbol": symbol}, nil, false, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}
func (f fapi) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := mapBTIF(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	// ensure instrument scales
	if f.p.instCache == nil || (req.Quantity != nil || req.Price != nil) && f.p.instCache[req.Symbol].Symbol == "" {
		_, _ = f.Instruments(ctx)
	}
	inst := f.p.instCache[req.Symbol]
	if req.Quantity != nil {
		q["quantity"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		q["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	var resp struct {
		OrderId int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	if err := f.p.fapi.Do(ctx, http.MethodPost, "/fapi/v1/order", q, nil, true, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderId), Symbol: req.Symbol, Status: mapBStatus(resp.Status)}, nil
}
func (f fapi) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		q["symbol"] = symbols[0]
	}
	var raw []map[string]any
	if err := f.p.fapi.Do(ctx, http.MethodGet, "/fapi/v2/positionRisk", q, nil, true, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, d := range raw {
		sym, _ := d["symbol"].(string)
		qStr, _ := d["positionAmt"].(string)
		epStr, _ := d["entryPrice"].(string)
		upStr, _ := d["unRealizedProfit"].(string)
		var qty, ep, up *big.Rat
		if v, ok := parseDecimalToRat(qStr); ok {
			qty = v
		}
		if v, ok := parseDecimalToRat(epStr); ok {
			ep = v
		}
		if v, ok := parseDecimalToRat(upStr); ok {
			up = v
		}
		side := core.SideBuy
		if qty != nil && qty.Sign() < 0 {
			side = core.SideSell
		}
		if qty != nil && qty.Sign() < 0 {
			qty = new(big.Rat).Abs(qty)
		}
		out = append(out, core.Position{Symbol: sym, Side: side, Quantity: qty, EntryPrice: ep, Unrealized: up})
	}
	return out, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (f fapi) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance futuresAPI: unsupported canonical symbol %s", canonical))
}

func (f fapi) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance futuresAPI: unsupported native symbol %s", native))
}

func (d dapi) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Symbols []struct {
			Symbol  string `json:"symbol"`
			Base    string `json:"baseAsset"`
			Quote   string `json:"quoteAsset"`
			Filters []struct {
				FilterType string `json:"filterType"`
				TickSize   string `json:"tickSize"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := d.p.dapi.Do(ctx, http.MethodGet, "/dapi/v1/exchangeInfo", nil, nil, false, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Symbols))
	for _, sdef := range resp.Symbols {
		var priceScale, qtyScale int
		for _, ft := range sdef.Filters {
			switch ft.FilterType {
			case "PRICE_FILTER":
				priceScale = scaleFromStep(ft.TickSize)
			case "LOT_SIZE":
				qtyScale = scaleFromStep(ft.StepSize)
			}
		}
		sym := core.CanonicalSymbol(sdef.Base, sdef.Quote)
		inst := core.Instrument{Symbol: sym, Base: sdef.Base, Quote: sdef.Quote, Market: core.MarketInverseFutures, PriceScale: priceScale, QtyScale: qtyScale}
		out = append(out, inst)
		if d.p.instCache == nil {
			d.p.instCache = map[string]core.Instrument{}
		}
		d.p.instCache[sym] = inst
	}
	return out, nil
}
func (d dapi) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	if err := d.p.dapi.Do(ctx, http.MethodGet, "/dapi/v1/ticker/bookTicker", map[string]string{"symbol": symbol}, nil, false, &struct{}{}); err != nil {
		return core.Ticker{}, err
	}
	return core.Ticker{Symbol: symbol, Time: time.Now()}, nil
}
func (d dapi) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	q := map[string]string{
		"symbol": core.CanonicalToBinance(req.Symbol),
		"side":   string(req.Side),
		"type":   string(req.Type),
	}
	if tif := mapBTIF(req.TimeInForce); tif != "" {
		q["timeInForce"] = tif
	}
	if d.p.instCache == nil || (req.Quantity != nil || req.Price != nil) && d.p.instCache[req.Symbol].Symbol == "" {
		_, _ = d.Instruments(ctx)
	}
	inst := d.p.instCache[req.Symbol]
	if req.Quantity != nil {
		q["quantity"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		q["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	var resp struct {
		OrderId int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	if err := d.p.dapi.Do(ctx, http.MethodPost, "/dapi/v1/order", q, nil, true, &resp); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: fmt.Sprintf("%d", resp.OrderId), Symbol: req.Symbol, Status: mapBStatus(resp.Status)}, nil
}
func (d dapi) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{}
	if len(symbols) == 1 {
		q["symbol"] = symbols[0]
	}
	var raw []map[string]any
	if err := d.p.dapi.Do(ctx, http.MethodGet, "/dapi/v1/positionRisk", q, nil, true, &raw); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(raw))
	for _, d := range raw {
		sym, _ := d["symbol"].(string)
		qStr, _ := d["positionAmt"].(string)
		epStr, _ := d["entryPrice"].(string)
		upStr, _ := d["unRealizedProfit"].(string)
		var qty, ep, up *big.Rat
		if v, ok := parseDecimalToRat(qStr); ok {
			qty = v
		}
		if v, ok := parseDecimalToRat(epStr); ok {
			ep = v
		}
		if v, ok := parseDecimalToRat(upStr); ok {
			up = v
		}
		side := core.SideBuy
		if qty != nil && qty.Sign() < 0 {
			side = core.SideSell
		}
		if qty != nil && qty.Sign() < 0 {
			qty = new(big.Rat).Abs(qty)
		}
		out = append(out, core.Position{Symbol: sym, Side: side, Quantity: qty, EntryPrice: ep, Unrealized: up})
	}
	return out, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (d dapi) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTCUSDT"
	}
	panic(fmt.Errorf("binance inverse futuresAPI: unsupported canonical symbol %s", canonical))
}

func (d dapi) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTCUSDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("binance inverse futuresAPI: unsupported native symbol %s", native))
}

// WebSocket methods are implemented in ws.go

// depthLevelsFromPairs converts [price, quantity] string pairs into structured depth levels
func depthLevelsFromPairs(pairs [][]interface{}) []corews.DepthLevel {
	levels := make([]corews.DepthLevel, 0, len(pairs))
	for _, pair := range pairs {
		if len(pair) < 2 {
			continue
		}
		var pStr, qStr string
		switch v := pair[0].(type) {
		case string:
			pStr = v
		default:
			pStr = fmt.Sprint(v)
		}
		switch v := pair[1].(type) {
		case string:
			qStr = v
		default:
			qStr = fmt.Sprint(v)
		}
		price, _ := parseDecimalToRat(pStr)
		qty, _ := parseDecimalToRat(qStr)
		levels = append(levels, corews.DepthLevel{Price: price, Qty: qty})
	}
	return levels
}
