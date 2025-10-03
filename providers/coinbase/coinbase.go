package coinbase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	cbws "github.com/coachpo/meltica/providers/coinbase/ws"
	"github.com/coachpo/meltica/transport"
)

var capabilities = core.Capabilities(
	core.CapabilitySpotPublicREST,
	core.CapabilitySpotTradingREST,
	core.CapabilityWebsocketPublic,
	core.CapabilityWebsocketPrivate,
)

type Provider struct {
	name       string
	rest       *transport.Client
	apiKey     string
	secret     string
	passphrase string

	mu            sync.RWMutex
	instCache     map[string]core.Instrument
	canonToNative map[string]string
	nativeToCanon map[string]string
}

func TestOnlyNewProvider() *Provider {
	return &Provider{
		name:          "coinbase",
		instCache:     map[string]core.Instrument{},
		canonToNative: map[string]string{},
		nativeToCanon: map[string]string{},
	}
}

func TestOnlyNormalizeBalances(raw []accountBalance) []core.Balance {
	return normalizeBalances(raw)
}

func New(apiKey, secret, passphrase string) (*Provider, error) {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	rest := &transport.Client{
		HTTP:    httpClient,
		BaseURL: "https://api.exchange.coinbase.com",
		Retry: transport.RetryPolicy{
			MaxRetries: 3,
			BaseDelay:  250 * time.Millisecond,
			MaxDelay:   1500 * time.Millisecond,
		},
		DefaultHeaders: map[string]string{
			"Accept":       "application/json",
			"Content-Type": "application/json",
			"User-Agent":   "meltica-coinbase/0.1",
		},
		OnHTTPError: mapHTTPError,
	}
	p := &Provider{
		name:       "coinbase",
		rest:       rest,
		apiKey:     apiKey,
		secret:     secret,
		passphrase: passphrase,
	}
	if apiKey != "" && secret != "" && passphrase != "" {
		signer, err := newSigner(apiKey, secret, passphrase)
		if err != nil {
			return nil, err
		}
		rest.Signer = signer
	}
	return p, nil
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Capabilities() core.ProviderCapabilities { return capabilities }

func (p *Provider) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (p *Provider) Spot(ctx context.Context) core.SpotAPI { return spotAPI{p} }

func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI { return unsupportedFutures{} }

func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI { return unsupportedFutures{} }

func (p *Provider) WS() core.WS { return cbws.New(p) }

func (p *Provider) Close() error { return nil }

// WebSocket support methods
func (p *Provider) Native(symbol string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.canonToNative[symbol]
}

func (p *Provider) CanonicalFromNative(native string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nativeToCanon[strings.ToUpper(native)]
}

func (p *Provider) EnsureInstruments(ctx context.Context) error {
	p.mu.RLock()
	ready := len(p.instCache) > 0
	p.mu.RUnlock()
	if ready {
		return nil
	}
	s := spotAPI{p: p}
	_, err := s.Instruments(ctx)
	return err
}

func (p *Provider) APIKey() string {
	return p.apiKey
}

func (p *Provider) Secret() string {
	return p.secret
}

func (p *Provider) Passphrase() string {
	return p.passphrase
}

type spotAPI struct{ p *Provider }

type unsupportedFutures struct{}

func (unsupportedFutures) Instruments(ctx context.Context) ([]core.Instrument, error) {
	return nil, core.ErrNotSupported
}

func (unsupportedFutures) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	return core.Ticker{}, core.ErrNotSupported
}

func (unsupportedFutures) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	return core.Order{}, core.ErrNotSupported
}

func (unsupportedFutures) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	return nil, core.ErrNotSupported
}

// Symbol conversion stubs to satisfy core.FuturesAPI; panic per instruction
func (unsupportedFutures) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("coinbase futures unsupported: %s", canonical))
}

func (unsupportedFutures) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("coinbase futures unsupported: %s", native))
}

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var resp struct {
		Epoch float64 `json:"epoch"`
	}
	if err := s.p.rest.Do(ctx, http.MethodGet, "/time", nil, nil, false, &resp); err != nil {
		return time.Time{}, err
	}
	if resp.Epoch == 0 {
		return time.Time{}, nil
	}
	sec, frac := math.Modf(resp.Epoch)
	return time.Unix(int64(sec), int64(frac*1e9)).UTC(), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var products []product
	if err := s.p.rest.Do(ctx, http.MethodGet, "/products", nil, nil, false, &products); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(products))
	s.p.mu.Lock()
	defer s.p.mu.Unlock()
	if s.p.instCache == nil {
		s.p.instCache = map[string]core.Instrument{}
		s.p.canonToNative = map[string]string{}
		s.p.nativeToCanon = map[string]string{}
	}
	for _, prod := range products {
		if !strings.EqualFold(prod.Status, "online") {
			continue
		}
		base := strings.ToUpper(prod.BaseCurrency)
		quote := strings.ToUpper(prod.QuoteCurrency)
		symbol := core.CanonicalSymbol(base, quote)
		priceScale := scaleFromIncrement(prod.QuoteIncrement)
		qtyScale := scaleFromIncrement(prod.BaseIncrement)
		inst := core.Instrument{
			Symbol:     symbol,
			Base:       base,
			Quote:      quote,
			Market:     core.MarketSpot,
			PriceScale: priceScale,
			QtyScale:   qtyScale,
		}
		s.p.instCache[symbol] = inst
		native := strings.ToUpper(prod.ID)
		s.p.canonToNative[symbol] = native
		s.p.nativeToCanon[native] = symbol
		out = append(out, inst)
	}
	return out, nil
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	product, err := s.p.ensureProduct(ctx, symbol)
	if err != nil {
		return core.Ticker{}, err
	}
	var resp struct {
		Bid  string    `json:"bid"`
		Ask  string    `json:"ask"`
		Time time.Time `json:"time"`
	}
	path := fmt.Sprintf("/products/%s/ticker", url.PathEscape(product))
	if err := s.p.rest.Do(ctx, http.MethodGet, path, nil, nil, false, &resp); err != nil {
		return core.Ticker{}, err
	}
	bid := parseDecimal(resp.Bid)
	ask := parseDecimal(resp.Ask)
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: resp.Time}, nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp []accountBalance
	if err := s.p.rest.Do(ctx, http.MethodGet, "/accounts", nil, nil, true, &resp); err != nil {
		return nil, err
	}
	return normalizeBalances(resp), nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	product, err := s.p.ensureProduct(ctx, symbol)
	if err != nil {
		return nil, err
	}
	trades := make([]core.Trade, 0, 100)
	cursor := ""
	prevCursor := ""
	const maxPages = 5
	for page := 0; page < maxPages; page++ {
		params := map[string]string{
			"product_id": product,
			"limit":      "100",
		}
		switch {
		case cursor != "":
			params["after"] = cursor
		case since > 0:
			params["after"] = fmt.Sprintf("%d", since)
		}
		var resp []fill
		hdr, err := s.p.rest.DoWithHeaders(ctx, http.MethodGet, "/fills", params, nil, true, &resp)
		if err != nil {
			return nil, err
		}
		for _, f := range resp {
			price := parseDecimal(f.Price)
			qty := parseDecimal(f.Size)
			side := mapSide(f.Side)
			symbolCanon := s.p.canonFromNative(strings.ToUpper(f.ProductID))
			if symbolCanon == "" {
				symbolCanon = symbol
			}
			trades = append(trades, core.Trade{
				Symbol:   symbolCanon,
				ID:       fmt.Sprintf("%d", f.TradeID),
				Price:    price,
				Quantity: qty,
				Side:     side,
				Time:     parseTime(f.CreatedAt),
			})
		}
		cursor = ""
		if hdr != nil {
			cursor = hdr.Get("Cb-After")
		}
		if len(resp) == 0 || cursor == "" || cursor == prevCursor {
			break
		}
		prevCursor = cursor
	}
	return trades, nil
}

func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	product, err := s.p.ensureProduct(ctx, req.Symbol)
	if err != nil {
		return core.Order{}, err
	}
	inst := s.p.inst(req.Symbol)
	payload := map[string]any{
		"product_id": product,
		"side":       strings.ToLower(string(req.Side)),
		"type":       mapOrderType(req.Type),
	}
	if req.Quantity != nil {
		payload["size"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		payload["price"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	if req.ClientID != "" {
		payload["client_oid"] = req.ClientID
	}
	if tif, err := mapTimeInForce(req.TimeInForce); err != nil {
		return core.Order{}, err
	} else if tif != "" {
		payload["time_in_force"] = tif
	}
	if req.ReduceOnly {
		payload["reduce_only"] = true
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return core.Order{}, err
	}
	var resp orderResponse
	if err := s.p.rest.Do(ctx, http.MethodPost, "/orders", nil, body, true, &resp); err != nil {
		return core.Order{}, err
	}
	symbolCanon := req.Symbol
	if native := strings.ToUpper(resp.ProductID); native != "" {
		symbolCanon = s.p.canonFromNative(native)
		if symbolCanon == "" {
			symbolCanon = req.Symbol
		}
	}
	return core.Order{ID: resp.ID, Symbol: symbolCanon, Status: mapStatus(resp.Status, resp.DoneReason)}, nil
}

func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	path := ""
	switch {
	case id != "":
		path = fmt.Sprintf("/orders/%s", url.PathEscape(id))
	case clientID != "":
		path = fmt.Sprintf("/orders/client:%s", url.PathEscape(clientID))
	default:
		return core.Order{}, errors.New("coinbase: order id or client id required")
	}
	var resp orderResponse
	if err := s.p.rest.Do(ctx, http.MethodGet, path, nil, nil, true, &resp); err != nil {
		return core.Order{}, err
	}
	filled := parseDecimal(resp.FilledSize)
	value := parseDecimal(resp.ExecutedValue)
	avg := averagePrice(value, filled)
	symbolCanon := symbol
	if native := strings.ToUpper(resp.ProductID); native != "" {
		symbolCanon = s.p.canonFromNative(native)
		if symbolCanon == "" {
			symbolCanon = symbol
		}
	}
	createdAt := parseTime(resp.CreatedAt)
	doneAt := parseTime(resp.DoneAt)
	return core.Order{
		ID:        resp.ID,
		Symbol:    symbolCanon,
		Status:    mapStatus(resp.Status, resp.DoneReason),
		FilledQty: filled,
		AvgPrice:  avg,
		CreatedAt: createdAt,
		UpdatedAt: doneAt,
	}, nil
}

func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	path := ""
	switch {
	case id != "":
		path = fmt.Sprintf("/orders/%s", url.PathEscape(id))
	case clientID != "":
		path = fmt.Sprintf("/orders/client:%s", url.PathEscape(clientID))
	default:
		return errors.New("coinbase: cancel requires order id or client id")
	}
	return s.p.rest.Do(ctx, http.MethodDelete, path, nil, nil, true, nil)
}

// Symbol conversion (static demo): only BTC-USD style to BTC-USDT is not 1:1 on Coinbase,
// but per instruction, support BTCUSDT <-> BTC-USDT only, panic otherwise.
func (s spotAPI) SpotNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("coinbase spotAPI: unsupported canonical symbol %s", canonical))
}

func (s spotAPI) SpotCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("coinbase spotAPI: unsupported native symbol %s", native))
}

func (p *Provider) ensureProduct(ctx context.Context, symbol string) (string, error) {
	if native := p.native(symbol); native != "" {
		return native, nil
	}
	s := spotAPI{p: p}
	if _, err := s.Instruments(ctx); err != nil {
		return "", err
	}
	if native := p.native(symbol); native != "" {
		return native, nil
	}
	return "", core.ErrNotSupported
}

func (p *Provider) native(symbol string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.canonToNative[symbol]
}

func (p *Provider) inst(symbol string) core.Instrument {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.instCache[symbol]
}

func (p *Provider) canonFromNative(native string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nativeToCanon[strings.ToUpper(native)]
}

func (p *Provider) ensureInstruments(ctx context.Context) error {
	p.mu.RLock()
	ready := len(p.instCache) > 0
	p.mu.RUnlock()
	if ready {
		return nil
	}
	s := spotAPI{p: p}
	_, err := s.Instruments(ctx)
	return err
}

func averagePrice(value, qty *big.Rat) *big.Rat {
	if value == nil || qty == nil || qty.Sign() == 0 {
		return nil
	}
	return new(big.Rat).Quo(value, qty)
}

func parseTime(val string) time.Time {
	if val == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, val); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func normalizeBalances(raw []accountBalance) []core.Balance {
	out := make([]core.Balance, 0, len(raw))
	now := time.Now().UTC()
	for _, b := range raw {
		amt := parseDecimal(b.Available)
		asset := strings.ToUpper(b.Currency)
		out = append(out, core.Balance{Asset: asset, Available: amt, Time: now})
	}
	return out
}

func parseDecimal(v string) *big.Rat {
	if v == "" {
		return nil
	}
	var r big.Rat
	if _, ok := r.SetString(v); !ok {
		return nil
	}
	return &r
}

func scaleFromIncrement(step string) int {
	if step == "" {
		return 0
	}
	if strings.Contains(step, ".") {
		return len(strings.TrimRight(strings.Split(step, ".")[1], "0"))
	}
	return 0
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
	case core.GTC:
		return "GTC", nil
	case core.ICO:
		return "IOC", nil
	case core.FOK:
		return "FOK", nil
	default:
		return "", &errs.E{
			Provider: "coinbase",
			Code:     errs.CodeInvalid,
			RawCode:  "unsupported_time_in_force",
			RawMsg:   fmt.Sprintf("unsupported time in force %q", t),
		}
	}
}

func mapSide(side string) core.OrderSide {
	switch strings.ToLower(side) {
	case "buy":
		return core.SideBuy
	case "sell":
		return core.SideSell
	default:
		return core.SideBuy
	}
}

type product struct {
	ID             string `json:"id"`
	BaseCurrency   string `json:"base_currency"`
	QuoteCurrency  string `json:"quote_currency"`
	BaseIncrement  string `json:"base_increment"`
	QuoteIncrement string `json:"quote_increment"`
	Status         string `json:"status"`
}

type accountBalance struct {
	Currency  string `json:"currency"`
	Balance   string `json:"balance"`
	Available string `json:"available"`
	Hold      string `json:"hold"`
}

type fill struct {
	TradeID   int64  `json:"trade_id"`
	ProductID string `json:"product_id"`
	Price     string `json:"price"`
	Size      string `json:"size"`
	Side      string `json:"side"`
	CreatedAt string `json:"created_at"`
}

type orderResponse struct {
	ID            string `json:"id"`
	ProductID     string `json:"product_id"`
	Status        string `json:"status"`
	DoneReason    string `json:"done_reason"`
	FilledSize    string `json:"filled_size"`
	ExecutedValue string `json:"executed_value"`
	CreatedAt     string `json:"created_at"`
	DoneAt        string `json:"done_at"`
}

func mapStatus(status, reason string) core.OrderStatus {
	switch strings.ToLower(status) {
	case "received", "open", "pending", "active":
		return core.OrderNew
	case "done", "settled":
		switch strings.ToLower(reason) {
		case "canceled":
			return core.OrderCanceled
		case "rejected":
			return core.OrderRejected
		default:
			return core.OrderFilled
		}
	default:
		return core.OrderNew
	}
}

func mapHTTPError(status int, body []byte) error {
	return fmt.Errorf("coinbase http %d: %s", status, string(body))
}
