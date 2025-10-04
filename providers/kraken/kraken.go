package kraken

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/providers/kraken/infra/rest"
	"github.com/coachpo/meltica/providers/kraken/infra/wsinfra"
	"github.com/coachpo/meltica/providers/kraken/routing"
)

// capability bitset for Kraken features.
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

// Provider implements core.Provider for Kraken.
type Provider struct {
	name            string
	restClient      *rest.Client
	restRouter      *routing.RESTRouter
	wsInfra         *wsinfra.Client
	wsRouter        *routing.WSRouter
	apiKey          string
	secret          string
	instCache       map[string]core.Instrument
	canonToKraken   map[string]string
	canonToKrakenWS map[string]string
	nativeToCanon   map[string]string
	tokenMu         sync.Mutex
	wsToken         string
	wsTokenExpiry   time.Time
}

// TestOnlyNewProvider constructs a provider instance for tests.
func TestOnlyNewProvider() *Provider {
	restClient := rest.NewClient(rest.Config{HTTPClient: &http.Client{Timeout: 15 * time.Second}})
	p := &Provider{
		name:            "kraken",
		restClient:      restClient,
		restRouter:      routing.NewRESTRouter(restClient),
		wsInfra:         wsinfra.NewClient(),
		instCache:       map[string]core.Instrument{},
		canonToKraken:   map[string]string{},
		canonToKrakenWS: map[string]string{},
		nativeToCanon:   map[string]string{},
	}
	p.wsRouter = routing.NewWSRouter(p.wsInfra, p)
	return p
}

// TestOnlyNormalizeBalances exposes balance normalization for fixtures.
func TestOnlyNormalizeBalances(raw map[string]string) []core.Balance {
	return normalizeBalances(raw)
}

// New constructs a Kraken provider with optional API credentials.
func New(apiKey, secret string) (*Provider, error) {
	restClient := rest.NewClient(rest.Config{APIKey: apiKey, Secret: secret, HTTPClient: &http.Client{Timeout: 15 * time.Second}})
	restRouter := routing.NewRESTRouter(restClient)
	wsInfra := wsinfra.NewClient()
	p := &Provider{
		name:            "kraken",
		restClient:      restClient,
		restRouter:      restRouter,
		wsInfra:         wsInfra,
		apiKey:          apiKey,
		secret:          secret,
		instCache:       map[string]core.Instrument{},
		canonToKraken:   map[string]string{},
		canonToKrakenWS: map[string]string{},
		nativeToCanon:   map[string]string{},
	}
	p.wsRouter = routing.NewWSRouter(wsInfra, p)
	return p, nil
}

func (p *Provider) dispatchREST(ctx context.Context, api rest.API, method, path string, query map[string]string, body []byte, signed bool, out any) error {
	if p.restRouter == nil {
		return errors.New("kraken: rest router not initialized")
	}
	msg := routing.RESTMessage{API: api, Method: method, Path: path, Query: query, Body: body, Signed: signed}
	return p.restRouter.Dispatch(ctx, msg, out)
}

// Name reports the adapter identifier.
func (p *Provider) Name() string { return p.name }

// Capabilities reports the supported feature matrix.
func (p *Provider) Capabilities() core.ProviderCapabilities { return capabilities }

func (p *Provider) SupportedProtocolVersion() string { return core.ProtocolVersion }

// Spot exposes Kraken spot REST endpoints.
func (p *Provider) Spot(ctx context.Context) core.SpotAPI { return spotAPI{p} }

// LinearFutures exposes Kraken linear futures REST endpoints.
func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI { return futuresAPI{p, "linear"} }

// InverseFutures exposes Kraken inverse futures REST endpoints.
func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI {
	return futuresAPI{p, "inverse"}
}

// WS returns the websocket handler.
func (p *Provider) WS() core.WS {
	if p.wsRouter == nil {
		p.wsInfra = wsinfra.NewClient()
		p.wsRouter = routing.NewWSRouter(p.wsInfra, p)
	}
	return newWSService(p.wsRouter)
}

// Close cleans up resources.
func (p *Provider) Close() error {
	if p.wsRouter != nil {
		_ = p.wsRouter.Close()
	}
	return nil
}

// WebSocket support methods
func (p *Provider) NativeSymbolForWS(canon string) string {
	if p.canonToKrakenWS != nil {
		if native := p.canonToKrakenWS[canon]; native != "" {
			return native
		}
	}
	if p.canonToKraken != nil {
		if native := p.canonToKraken[canon]; native != "" {
			return native
		}
	}
	return canon
}

func (p *Provider) CanonicalSymbol(exch string, requested []string) string {
	if exch == "" {
		return routing.CanonicalFromRequested(exch, requested)
	}
	upper := strings.ToUpper(strings.TrimSpace(exch))
	if p.nativeToCanon != nil {
		if canon := p.nativeToCanon[upper]; canon != "" {
			return canon
		}
		if canon := p.nativeToCanon[strings.ReplaceAll(upper, "/", "")]; canon != "" {
			return canon
		}
	}
	for c, native := range p.canonToKraken {
		if strings.EqualFold(native, upper) {
			return c
		}
	}
	for c, native := range p.canonToKrakenWS {
		if strings.EqualFold(native, upper) {
			return c
		}
	}
	return routing.CanonicalFromRequested(upper, requested)
}

func (p *Provider) MapNativeToCanon(native string) string {
	if native == "" {
		return native
	}
	native = strings.ToUpper(native)
	if canon := p.nativeToCanon[native]; canon != "" {
		return canon
	}
	return native
}

func (p *Provider) EnsureInstruments(ctx context.Context) error {
	if len(p.instCache) > 0 {
		return nil
	}
	_, err := spotAPI{p}.Instruments(ctx)
	return err
}

func (p *Provider) APIKey() string {
	return p.apiKey
}

func (p *Provider) Secret() string {
	return p.secret
}

func (p *Provider) GetWSToken(ctx context.Context) (string, error) {
	return p.getWSToken(ctx)
}

func (p *Provider) getWSToken(ctx context.Context) (string, error) {
	p.tokenMu.Lock()
	if p.wsToken != "" && time.Now().Before(p.wsTokenExpiry.Add(-1*time.Minute)) {
		tok := p.wsToken
		p.tokenMu.Unlock()
		return tok, nil
	}
	p.tokenMu.Unlock()

	form := url.Values{}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	var resp struct {
		Error  []string `json:"error"`
		Result []struct {
			Token   string `json:"token"`
			Expires int64  `json:"expires"`
		} `json:"result"`
	}
	if err := p.dispatchREST(ctx, rest.SpotAPI, http.MethodPost, "/0/private/GetWebSocketsToken", nil, []byte(form.Encode()), true, &resp); err != nil {
		return "", err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return "", err
	}
	if len(resp.Result) == 0 || resp.Result[0].Token == "" {
		return "", errors.New("kraken: empty ws token response")
	}

	tok := resp.Result[0].Token
	dur := time.Duration(resp.Result[0].Expires) * time.Second
	if dur == 0 {
		dur = time.Hour
	}

	p.tokenMu.Lock()
	p.wsToken = tok
	p.wsTokenExpiry = time.Now().Add(dur)
	p.tokenMu.Unlock()

	return tok, nil
}

func (p *Provider) doPrivate(ctx context.Context, path string, form url.Values, out any) error {
	if form == nil {
		form = url.Values{}
	}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	body := []byte(form.Encode())
	return p.dispatchREST(ctx, rest.SpotAPI, http.MethodPost, path, nil, body, true, out)
}

func (p *Provider) mapNativeToCanon(native string) string {
	if native == "" {
		return native
	}
	native = strings.ToUpper(native)
	if canon := p.nativeToCanon[native]; canon != "" {
		return canon
	}
	return native
}

type spotAPI struct{ p *Provider }
type futuresAPI struct {
	p    *Provider
	mode string // "linear" or "inverse"
}

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			UnixTime int64 `json:"unixtime"`
		} `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodGet, "/0/public/Time", nil, nil, false, &resp); err != nil {
		return time.Time{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return time.Time{}, err
	}
	if resp.Result.UnixTime == 0 {
		return time.Time{}, nil
	}
	return time.Unix(resp.Result.UnixTime, 0).UTC(), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Error  []string             `json:"error"`
		Result map[string]assetPair `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodGet, "/0/public/AssetPairs", nil, nil, false, &resp); err != nil {
		return nil, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Result))
	if s.p.instCache == nil {
		s.p.instCache = map[string]core.Instrument{}
	}
	if s.p.canonToKraken == nil {
		s.p.canonToKraken = map[string]string{}
	}
	if s.p.canonToKrakenWS == nil {
		s.p.canonToKrakenWS = map[string]string{}
	}
	if s.p.nativeToCanon == nil {
		s.p.nativeToCanon = map[string]string{}
	}
	for _, pair := range resp.Result {
		base := normalizeAsset(pair.Base)
		quote := normalizeAsset(pair.Quote)
		if base == "" || quote == "" {
			continue
		}
		symbol := core.CanonicalSymbol(base, quote)
		inst := core.Instrument{
			Symbol:     symbol,
			Base:       base,
			Quote:      quote,
			Market:     core.MarketSpot,
			PriceScale: pair.PairDecimals,
			QtyScale:   pair.LotDecimals,
		}
		s.p.instCache[symbol] = inst
		s.p.canonToKraken[symbol] = pair.AltName
		wsName := pair.WSName
		if wsName == "" {
			wsName = pair.AltName
		}
		s.p.canonToKrakenWS[symbol] = wsName
		s.p.nativeToCanon[strings.ToUpper(pair.AltName)] = symbol
		s.p.nativeToCanon[strings.ToUpper(wsName)] = symbol
		out = append(out, inst)
	}
	return out, nil
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	if err := s.p.ensureInstruments(ctx); err != nil {
		return core.Ticker{}, err
	}
	pair := s.p.canonToKraken[symbol]
	if pair == "" {
		return core.Ticker{}, core.ErrNotSupported
	}
	query := map[string]string{"pair": pair}
	var resp struct {
		Error  []string                        `json:"error"`
		Result map[string]krakenTickerEnvelope `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodGet, "/0/public/Ticker", query, nil, false, &resp); err != nil {
		return core.Ticker{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return core.Ticker{}, err
	}
	for _, data := range resp.Result {
		bid, _ := parseDecimal(data.Bid())
		ask, _ := parseDecimal(data.Ask())
		return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
	}
	return core.Ticker{}, nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp struct {
		Error  []string          `json:"error"`
		Result map[string]string `json:"result"`
	}
	if err := s.p.doPrivate(ctx, "/0/private/Balance", nil, &resp); err != nil {
		return nil, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return nil, err
	}
	return normalizeBalances(resp.Result), nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	if err := s.p.ensureInstruments(ctx); err != nil {
		return nil, err
	}
	pair := s.p.canonToKraken[symbol]
	if pair == "" {
		return nil, core.ErrNotSupported
	}
	out := make([]core.Trade, 0, 64)
	offset := 0
	const pageSize = 50
	for {
		form := url.Values{}
		form.Set("pair", pair)
		if since > 0 {
			form.Set("start", strconv.FormatInt(since, 10))
		}
		if offset > 0 {
			form.Set("ofs", strconv.Itoa(offset))
		}
		var resp struct {
			Error  []string `json:"error"`
			Result struct {
				Trades map[string]krakenTradeRecord `json:"trades"`
				Count  int                          `json:"count"`
			} `json:"result"`
		}
		if err := s.p.doPrivate(ctx, "/0/private/TradesHistory", form, &resp); err != nil {
			return nil, err
		}
		if err := rest.ResultError(resp.Error); err != nil {
			return nil, err
		}
		batch := make([]core.Trade, 0, len(resp.Result.Trades))
		for _, tr := range resp.Result.Trades {
			canon := s.p.mapNativeToCanon(strings.ToUpper(tr.Pair))
			price := parseDecimalStr(tr.Price)
			qty := parseDecimalStr(tr.Volume)
			var side core.OrderSide
			if strings.EqualFold(tr.Type, "buy") {
				side = core.SideBuy
			} else {
				side = core.SideSell
			}
			batch = append(batch, core.Trade{
				Symbol:   canon,
				ID:       tr.OrderTxID,
				Price:    price,
				Quantity: qty,
				Side:     side,
				Time:     time.Unix(tr.Timestamp(), 0).UTC(),
			})
		}
		if len(batch) == 0 {
			break
		}
		sort.Slice(batch, func(i, j int) bool {
			return batch[i].Time.Before(batch[j].Time)
		})
		out = append(out, batch...)
		offset += len(batch)
		if resp.Result.Count == 0 {
			if len(batch) < pageSize {
				break
			}
			continue
		}
		if offset >= resp.Result.Count {
			break
		}
	}
	return out, nil
}

func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	if err := s.p.ensureInstruments(ctx); err != nil {
		return core.Order{}, err
	}
	pair := s.p.canonToKraken[req.Symbol]
	if pair == "" {
		return core.Order{}, core.ErrNotSupported
	}
	inst := s.p.instCache[req.Symbol]
	form := url.Values{}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	form.Set("pair", pair)
	form.Set("type", strings.ToLower(string(req.Side)))
	form.Set("ordertype", mapOrderType(req.Type))
	if req.Quantity != nil {
		form.Set("volume", core.FormatDecimal(req.Quantity, inst.QtyScale))
	}
	if req.Price != nil {
		form.Set("price", core.FormatDecimal(req.Price, inst.PriceScale))
	}
	if req.ClientID != "" {
		form.Set("userref", req.ClientID)
	}
	if tif, err := mapTimeInForce(req.TimeInForce); err != nil {
		return core.Order{}, err
	} else if tif != "" {
		form.Set("timeinforce", tif)
	}
	if req.ReduceOnly {
		form.Set("reduce_only", "true")
	}
	body := []byte(form.Encode())
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			TxID []string `json:"txid"`
		} `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodPost, "/0/private/AddOrder", nil, body, true, &resp); err != nil {
		return core.Order{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return core.Order{}, err
	}
	orderID := ""
	if len(resp.Result.TxID) > 0 {
		orderID = resp.Result.TxID[0]
	}
	return core.Order{ID: orderID, Symbol: req.Symbol, Status: core.OrderNew}, nil
}

func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	form := url.Values{}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	if id != "" {
		form.Set("txid", id)
	}
	if clientID != "" {
		form.Set("userref", clientID)
	}
	body := []byte(form.Encode())
	var resp struct {
		Error  []string               `json:"error"`
		Result map[string]krakenOrder `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodPost, "/0/private/QueryOrders", nil, body, true, &resp); err != nil {
		return core.Order{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return core.Order{}, err
	}
	for _, ord := range resp.Result {
		filled, _ := parseDecimal(ord.VolumeExecuted)
		avg, _ := parseDecimal(ord.AvgPrice)
		return core.Order{ID: ord.TxID, Symbol: symbol, Status: mapStatus(ord.Status), FilledQty: filled, AvgPrice: avg, CreatedAt: time.Unix(ord.OpenTime, 0)}, nil
	}
	return core.Order{}, nil
}

func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	if id == "" && clientID == "" {
		return errors.New("kraken cancel requires order id")
	}
	form := url.Values{}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	if id != "" {
		form.Set("txid", id)
	}
	if clientID != "" {
		form.Set("userref", clientID)
	}
	body := []byte(form.Encode())
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}
	if err := s.p.dispatchREST(ctx, rest.SpotAPI, http.MethodPost, "/0/private/CancelOrder", nil, body, true, &resp); err != nil {
		return err
	}
	return rest.ResultError(resp.Error)
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (s spotAPI) SpotNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "XBTUSDT"
	}
	panic(fmt.Errorf("kraken spotAPI: unsupported canonical symbol %s", canonical))
}

func (s spotAPI) SpotCanonicalSymbol(native string) string {
	n := strings.ToUpper(strings.ReplaceAll(native, "/", ""))
	if n == "XBTUSDT" || n == "BTCUSDT" {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("kraken spotAPI: unsupported native symbol %s", native))
}

// futuresAPI implementations
func (f futuresAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var resp struct {
		Error  []string            `json:"error"`
		Result []krakenFuturesInst `json:"result"`
	}
	if err := f.p.dispatchREST(ctx, rest.FuturesAPI, http.MethodGet, "/api/v3/instruments", nil, nil, false, &resp); err != nil {
		return nil, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(resp.Result))
	for _, inst := range resp.Result {
		// Skip if not matching our mode (linear/inverse)
		if f.mode == "linear" && inst.MarginCurrency != "USD" {
			continue
		}
		if f.mode == "inverse" && inst.MarginCurrency == "USD" {
			continue
		}
		symbol := core.CanonicalSymbol(inst.Underlying, inst.MarginCurrency)
		market := core.MarketLinearFutures
		if f.mode == "inverse" {
			market = core.MarketInverseFutures
		}
		out = append(out, core.Instrument{
			Symbol:     symbol,
			Base:       inst.Underlying,
			Quote:      inst.MarginCurrency,
			Market:     market,
			PriceScale: 2, // Default scale
			QtyScale:   6, // Default scale
		})
	}
	return out, nil
}

func (f futuresAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var resp struct {
		Error  []string                       `json:"error"`
		Result map[string]krakenFuturesTicker `json:"result"`
	}
	if err := f.p.dispatchREST(ctx, rest.FuturesAPI, http.MethodGet, "/api/v3/tickers", nil, nil, false, &resp); err != nil {
		return core.Ticker{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return core.Ticker{}, err
	}
	for _, tick := range resp.Result {
		bid := parseDecimalStr(tick.Bid)
		ask := parseDecimalStr(tick.Ask)
		return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
	}
	return core.Ticker{}, nil
}

func (f futuresAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	form := url.Values{}
	form.Set("nonce", strconv.FormatInt(time.Now().UnixMilli(), 10))
	form.Set("symbol", req.Symbol)
	form.Set("side", strings.ToLower(string(req.Side)))
	form.Set("orderType", mapOrderType(req.Type))
	if req.Quantity != nil {
		form.Set("size", core.FormatDecimal(req.Quantity, 6))
	}
	if req.Price != nil {
		form.Set("price", core.FormatDecimal(req.Price, 2))
	}
	if req.ClientID != "" {
		form.Set("cliOrdId", req.ClientID)
	}
	body := []byte(form.Encode())
	var resp struct {
		Error  []string `json:"error"`
		Result struct {
			OrderID string `json:"order_id"`
		} `json:"result"`
	}
	if err := f.p.dispatchREST(ctx, rest.FuturesAPI, http.MethodPost, "/api/v3/sendorder", nil, body, true, &resp); err != nil {
		return core.Order{}, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return core.Order{}, err
	}
	return core.Order{ID: resp.Result.OrderID, Symbol: req.Symbol, Status: core.OrderNew}, nil
}

func (f futuresAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	var resp struct {
		Error  []string                `json:"error"`
		Result []krakenFuturesPosition `json:"result"`
	}
	if err := f.p.dispatchREST(ctx, rest.FuturesAPI, http.MethodGet, "/api/v3/openpositions", nil, nil, true, &resp); err != nil {
		return nil, err
	}
	if err := rest.ResultError(resp.Error); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(resp.Result))
	for _, pos := range resp.Result {
		if len(symbols) > 0 && symbols[0] != "" && pos.Symbol != symbols[0] {
			continue
		}
		qty := parseDecimalStr(pos.Size)
		ep := parseDecimalStr(pos.EntryPrice)
		up := parseDecimalStr(pos.UnrealizedPnL)
		side := core.SideBuy
		if strings.ToLower(pos.Side) == "short" {
			side = core.SideSell
		}
		out = append(out, core.Position{
			Symbol:     pos.Symbol,
			Side:       side,
			Quantity:   qty,
			EntryPrice: ep,
			Unrealized: up,
		})
	}
	return out, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (f futuresAPI) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "XBTUSDT"
	}
	panic(fmt.Errorf("kraken futuresAPI: unsupported canonical symbol %s", canonical))
}

func (f futuresAPI) FutureCanonicalSymbol(native string) string {
	n := strings.ToUpper(strings.ReplaceAll(native, "/", ""))
	if n == "XBTUSDT" || n == "BTCUSDT" {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("kraken futuresAPI: unsupported native symbol %s", native))
}

func (p *Provider) ensureInstruments(ctx context.Context) error {
	if len(p.instCache) > 0 {
		return nil
	}
	_, err := spotAPI{p}.Instruments(ctx)
	return err
}

type assetPair struct {
	AltName      string `json:"altname"`
	WSName       string `json:"wsname"`
	Base         string `json:"base"`
	Quote        string `json:"quote"`
	PairDecimals int    `json:"pair_decimals"`
	LotDecimals  int    `json:"lot_decimals"`
}

type krakenTickerEnvelope struct {
	AskLevels [][]string `json:"a"`
	BidLevels [][]string `json:"b"`
}

func (k krakenTickerEnvelope) Ask() string {
	if len(k.AskLevels) > 0 && len(k.AskLevels[0]) > 0 {
		return k.AskLevels[0][0]
	}
	return ""
}

func (k krakenTickerEnvelope) Bid() string {
	if len(k.BidLevels) > 0 && len(k.BidLevels[0]) > 0 {
		return k.BidLevels[0][0]
	}
	return ""
}

type krakenOrder struct {
	TxID           string `json:"refid"`
	Status         string `json:"status"`
	VolumeExecuted string `json:"vol_exec"`
	AvgPrice       string `json:"price"`
	OpenTime       int64  `json:"opentm"`
}

type krakenTradeRecord struct {
	OrderTxID string `json:"ordertxid"`
	Pair      string `json:"pair"`
	Time      string `json:"time"`
	Type      string `json:"type"`
	OrderType string `json:"ordertype"`
	Price     string `json:"price"`
	Cost      string `json:"cost"`
	Fee       string `json:"fee"`
	Volume    string `json:"vol"`
	Margin    string `json:"margin"`
}

func (t krakenTradeRecord) Timestamp() int64 {
	s := strings.ReplaceAll(t.Time, ".", "")
	if s == "" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i / 1000
}

func normalizeAsset(asset string) string {
	asset = strings.ToUpper(strings.TrimSpace(asset))
	return asset
}

func parseDecimal(s string) (*big.Rat, bool) {
	var out big.Rat
	if s == "" {
		return nil, false
	}
	if _, ok := out.SetString(s); !ok {
		return nil, false
	}
	return &out, true
}

func parseDecimalStr(s string) *big.Rat {
	r, _ := parseDecimal(s)
	return r
}

func mapOrderType(t core.OrderType) string {
	switch t {
	case core.TypeMarket:
		return "market"
	case core.TypeLimit:
		return "limit"
	default:
		return "limit"
	}
}

func mapTimeInForce(t core.TimeInForce) (string, error) {
	switch t {
	case "":
		return "", nil
	case core.ICO:
		return "IOC", nil
	case core.FOK:
		return "FOK", nil
	default:
		return "", &errs.E{
			Provider: "kraken",
			Code:     errs.CodeInvalid,
			RawCode:  "unsupported_time_in_force",
			RawMsg:   fmt.Sprintf("unsupported time in force %q", t),
		}
	}
}

// normalizeBalances converts raw asset entries to canonical balances.
func normalizeBalances(raw map[string]string) []core.Balance {
	balances := make([]core.Balance, 0, len(raw))
	for asset, amt := range raw {
		r := parseDecimalStr(amt)
		balances = append(balances, core.Balance{Asset: normalizeAsset(asset), Available: r, Time: time.Now()})
	}
	return balances
}

// Kraken futures types
type krakenFuturesInst struct {
	Symbol         string `json:"symbol"`
	Underlying     string `json:"underlying"`
	MarginCurrency string `json:"marginCurrency"`
	ContractType   string `json:"contractType"`
}

type krakenFuturesTicker struct {
	Bid string `json:"bid"`
	Ask string `json:"ask"`
}

type krakenFuturesPosition struct {
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	Size          string `json:"size"`
	EntryPrice    string `json:"entryPrice"`
	UnrealizedPnL string `json:"unrealizedPnL"`
}
