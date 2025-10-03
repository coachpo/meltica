package okx

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coachpo/meltica/core"
	okxws "github.com/coachpo/meltica/providers/okx/ws"
	"github.com/coachpo/meltica/transport"
)

type Provider struct {
	name       string
	rest       *transport.Client
	apiKey     string
	secret     string
	passphrase string
	instCache  map[string]core.Instrument
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

func New(publicKey, secret, passphrase string) (*Provider, error) {
	httpClient := &http.Client{Timeout: 10 * time.Second}
	rest := &transport.Client{HTTP: httpClient, BaseURL: "https://www.okx.com"}
	rest.Signer = okxSigner(publicKey, secret, passphrase)
	return &Provider{name: "okx", rest: rest, apiKey: publicKey, secret: secret, passphrase: passphrase}, nil
}

// do wraps transport.Do and handles OKX envelope {code,msg,data}
func (p *Provider) do(ctx context.Context, method, path string, query map[string]string, body []byte, signed bool, out any) error {
	var env struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := p.rest.Do(ctx, method, path, query, body, signed, &env); err != nil {
		return err
	}
	if env.Code != "0" && env.Code != "" {
		return mapOKXCode(200, env.Code, env.Msg)
	}
	if out == nil {
		return nil
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return nil
	}
	return json.Unmarshal(env.Data, out)
}

func (p *Provider) Name() string { return p.name }

func (p *Provider) Capabilities() core.ProviderCapabilities { return capabilities }

func (p *Provider) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (p *Provider) Spot(ctx context.Context) core.SpotAPI              { return spotAPI{p} }
func (p *Provider) LinearFutures(ctx context.Context) core.FuturesAPI  { return futAPI{p} }
func (p *Provider) InverseFutures(ctx context.Context) core.FuturesAPI { return futAPI{p} }
func (p *Provider) WS() core.WS                                        { return okxws.New(p) }
func (p *Provider) Close() error                                       { return nil }

// WebSocket support methods
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
type futAPI struct{ p *Provider }

func (s spotAPI) ServerTime(ctx context.Context) (time.Time, error) {
	var arr []struct {
		Ts string `json:"ts"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/public/time", nil, nil, false, &arr); err != nil {
		return time.Time{}, err
	}
	ts := "0"
	if len(arr) > 0 {
		ts = arr[0].Ts
	}
	msec, _ := time.ParseDuration(ts + "ms")
	return time.Unix(0, msec.Nanoseconds()), nil
}

func (s spotAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var list []struct {
		InstId string `json:"instId"`
		Base   string `json:"baseCcy"`
		Quote  string `json:"quoteCcy"`
		TickSz string `json:"tickSz"`
		LotSz  string `json:"lotSz"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/public/instruments", map[string]string{"instType": "SPOT"}, nil, false, &list); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(list))
	for _, d := range list {
		sym := core.CanonicalSymbol(d.Base, d.Quote)
		inst := core.Instrument{Symbol: sym, Base: d.Base, Quote: d.Quote, Market: core.MarketSpot,
			PriceScale: scaleFromStep(d.TickSz), QtyScale: scaleFromStep(d.LotSz)}
		out = append(out, inst)
		if s.p.instCache == nil {
			s.p.instCache = map[string]core.Instrument{}
		}
		s.p.instCache[sym] = inst
	}
	return out, nil
}

func (s spotAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var arr []struct {
		BidPx string `json:"bidPx"`
		AskPx string `json:"askPx"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/market/ticker", map[string]string{"instId": core.CanonicalToOKX(symbol)}, nil, false, &arr); err != nil {
		return core.Ticker{}, err
	}
	var bid, ask *big.Rat
	if len(arr) > 0 {
		bid, _ = parseDecimalToRat(arr[0].BidPx)
		ask, _ = parseDecimalToRat(arr[0].AskPx)
	}
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
}

func (s spotAPI) Balances(ctx context.Context) ([]core.Balance, error) {
	var resp []struct {
		Ccy   string `json:"ccy"`
		Avail string `json:"availBal"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/account/balance", nil, nil, true, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Balance, 0, len(resp))
	for _, b := range resp {
		val, _ := parseDecimalToRat(b.Avail)
		out = append(out, core.Balance{Asset: b.Ccy, Available: val, Time: time.Now()})
	}
	return out, nil
}

func (s spotAPI) Trades(ctx context.Context, symbol string, since int64) ([]core.Trade, error) {
	params := map[string]string{"instId": core.CanonicalToOKX(symbol)}
	if since > 0 {
		params["after"] = fmt.Sprintf("%d", since)
	}
	var resp []struct {
		TradeID string `json:"tradeId"`
		Px      string `json:"px"`
		Sz      string `json:"sz"`
		Side    string `json:"side"`
		Ts      string `json:"ts"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/trade/fills", params, nil, true, &resp); err != nil {
		return nil, err
	}
	out := make([]core.Trade, 0, len(resp))
	for _, tr := range resp {
		price, _ := parseDecimalToRat(tr.Px)
		qty, _ := parseDecimalToRat(tr.Sz)
		side := core.SideBuy
		if strings.EqualFold(tr.Side, "sell") {
			side = core.SideSell
		}
		ms, _ := strconv.ParseInt(tr.Ts, 10, 64)
		out = append(out, core.Trade{Symbol: symbol, ID: tr.TradeID, Price: price, Quantity: qty, Side: side, Time: time.UnixMilli(ms)})
	}
	return out, nil
}
func (s spotAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	body := map[string]string{
		"instId":  req.Symbol,
		"tdMode":  "cash",
		"side":    string(req.Side),
		"ordType": string(req.Type),
	}
	if req.ClientID != "" {
		body["clOrdId"] = req.ClientID
	}
	if s.p.instCache == nil {
		_, _ = s.Instruments(ctx)
	}
	inst := s.p.instCache[req.Symbol]
	if req.Quantity != nil {
		body["sz"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		body["px"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	b, _ := json.Marshal(body)
	var data []struct {
		OrdId  string `json:"ordId"`
		InstId string `json:"instId"`
		State  string `json:"state"`
	}
	if err := s.p.do(ctx, http.MethodPost, "/api/v5/trade/order", nil, b, true, &data); err != nil {
		return core.Order{}, err
	}
	if len(data) == 0 {
		return core.Order{}, nil
	}
	d := data[0]
	return core.Order{ID: d.OrdId, Symbol: req.Symbol, Status: mapOKXStatus(d.State)}, nil
}
func (s spotAPI) GetOrder(ctx context.Context, symbol, id, clientID string) (core.Order, error) {
	q := map[string]string{"instId": symbol}
	if id != "" {
		q["ordId"] = id
	}
	if clientID != "" {
		q["clOrdId"] = clientID
	}
	var data []struct {
		OrdId  string `json:"ordId"`
		InstId string `json:"instId"`
		State  string `json:"state"`
	}
	if err := s.p.do(ctx, http.MethodGet, "/api/v5/trade/order", q, nil, true, &data); err != nil {
		return core.Order{}, err
	}
	if len(data) == 0 {
		return core.Order{}, nil
	}
	d := data[0]
	return core.Order{ID: d.OrdId, Symbol: symbol, Status: mapOKXStatus(d.State)}, nil
}
func (s spotAPI) CancelOrder(ctx context.Context, symbol, id, clientID string) error {
	body := map[string]string{"instId": symbol}
	if id != "" {
		body["ordId"] = id
	}
	if clientID != "" {
		body["clOrdId"] = clientID
	}
	b, _ := json.Marshal(body)
	return s.p.do(ctx, http.MethodPost, "/api/v5/trade/cancel-order", nil, b, true, nil)
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (s spotAPI) SpotNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx spotAPI: unsupported canonical symbol %s", canonical))
}

func (s spotAPI) SpotCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx spotAPI: unsupported native symbol %s", native))
}

func (f futAPI) Instruments(ctx context.Context) ([]core.Instrument, error) {
	var list []struct {
		InstId string `json:"instId"`
		Base   string `json:"baseCcy"`
		Quote  string `json:"quoteCcy"`
		TickSz string `json:"tickSz"`
		LotSz  string `json:"lotSz"`
	}
	if err := f.p.do(ctx, http.MethodGet, "/api/v5/public/instruments", map[string]string{"instType": "SWAP"}, nil, false, &list); err != nil {
		return nil, err
	}
	out := make([]core.Instrument, 0, len(list))
	for _, d := range list {
		sym := core.CanonicalSymbol(d.Base, d.Quote)
		inst := core.Instrument{Symbol: sym, Base: d.Base, Quote: d.Quote, Market: core.MarketLinearFutures,
			PriceScale: scaleFromStep(d.TickSz), QtyScale: scaleFromStep(d.LotSz)}
		out = append(out, inst)
		if f.p.instCache == nil {
			f.p.instCache = map[string]core.Instrument{}
		}
		f.p.instCache[sym] = inst
	}
	return out, nil
}
func (f futAPI) PlaceOrder(ctx context.Context, req core.OrderRequest) (core.Order, error) {
	body := map[string]string{
		"instId":  core.CanonicalToOKX(req.Symbol) + "-SWAP",
		"tdMode":  "cross",
		"side":    string(req.Side),
		"ordType": string(req.Type),
	}
	if req.ClientID != "" {
		body["clOrdId"] = req.ClientID
	}
	if f.p.instCache == nil || f.p.instCache[req.Symbol].Symbol == "" {
		_, _ = f.Instruments(ctx)
	}
	inst := f.p.instCache[req.Symbol]
	if req.Quantity != nil {
		body["sz"] = core.FormatDecimal(req.Quantity, inst.QtyScale)
	}
	if req.Price != nil {
		body["px"] = core.FormatDecimal(req.Price, inst.PriceScale)
	}
	b, _ := json.Marshal(body)
	var data []struct {
		OrdId string `json:"ordId"`
		State string `json:"state"`
	}
	if err := f.p.do(ctx, http.MethodPost, "/api/v5/trade/order", nil, b, true, &data); err != nil {
		return core.Order{}, err
	}
	if len(data) == 0 {
		return core.Order{}, nil
	}
	d := data[0]
	return core.Order{ID: d.OrdId, Symbol: req.Symbol, Status: mapOKXStatus(d.State)}, nil
}
func (f futAPI) Positions(ctx context.Context, symbols ...string) ([]core.Position, error) {
	q := map[string]string{"instType": "SWAP"}
	if len(symbols) == 1 && symbols[0] != "" {
		q["instId"] = symbols[0]
	}
	var data []map[string]any
	if err := f.p.do(ctx, http.MethodGet, "/api/v5/account/positions", q, nil, true, &data); err != nil {
		return nil, err
	}
	out := make([]core.Position, 0, len(data))
	for _, d := range data {
		sym, _ := d["instId"].(string)
		sideStr, _ := d["posSide"].(string)
		qStr, _ := d["pos"].(string)
		epStr, _ := d["avgPx"].(string)
		upStr, _ := d["upl"].(string)
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
		if strings.ToLower(sideStr) == "short" {
			side = core.SideSell
		}
		out = append(out, core.Position{Symbol: sym, Side: side, Quantity: qty, EntryPrice: ep, Unrealized: up})
	}
	return out, nil
}

// Symbol conversion (static demo): only BTCUSDT <-> BTC-USDT
func (f futAPI) FutureNativeSymbol(canonical string) string {
	if strings.EqualFold(canonical, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx futuresAPI: unsupported canonical symbol %s", canonical))
}

func (f futAPI) FutureCanonicalSymbol(native string) string {
	if strings.EqualFold(native, "BTC-USDT") {
		return "BTC-USDT"
	}
	panic(fmt.Errorf("okx futuresAPI: unsupported native symbol %s", native))
}

func (f futAPI) Ticker(ctx context.Context, symbol string) (core.Ticker, error) {
	var arr []struct {
		BidPx string `json:"bidPx"`
		AskPx string `json:"askPx"`
	}
	if err := f.p.do(ctx, http.MethodGet, "/api/v5/market/ticker", map[string]string{"instId": core.CanonicalToOKX(symbol) + "-SWAP"}, nil, false, &arr); err != nil {
		return core.Ticker{}, err
	}
	var bid, ask *big.Rat
	if len(arr) > 0 {
		bid, _ = parseDecimalToRat(arr[0].BidPx)
		ask, _ = parseDecimalToRat(arr[0].AskPx)
	}
	return core.Ticker{Symbol: symbol, Bid: bid, Ask: ask, Time: time.Now()}, nil
}

// WebSocket methods implemented in ws.go

// register
