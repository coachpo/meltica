package routing

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"net/http"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	capabilities "github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/internal/numeric"
)

func TestTradePrecisionHarness(t *testing.T) {
	profiles := []instrumentProfile{
		{
			name:       "spot",
			symbol:     "BTC-USDT",
			market:     core.MarketSpot,
			qtyScale:   6,
			qtyMin:     0,
			qtyMax:     3,
			priceScale: 2,
			priceMin:   1000,
			priceMax:   60000,
		},
		{
			name:       "linear",
			symbol:     "ETH-USDT",
			market:     core.MarketLinearFutures,
			qtyScale:   3,
			qtyMin:     1,
			qtyMax:     20,
			priceScale: 1,
			priceMin:   100,
			priceMax:   8000,
		},
		{
			name:       "inverse",
			symbol:     "BTC-USD",
			market:     core.MarketInverseFutures,
			qtyScale:   0,
			qtyMin:     1,
			qtyMax:     10,
			priceScale: 1,
			priceMin:   500,
			priceMax:   50000,
		},
	}

	const iterations = 10000
	totalChecks := 0
	totalFailures := 0

	seed := time.Now().UnixNano()
	for idx, profile := range profiles {
		runner := newProfileRunner(t, profile, seed+int64(idx))
		for i := 0; i < iterations; i++ {
			checks, failures := runner.runIteration()
			totalChecks += checks
			totalFailures += failures
		}
	}

	allowed := int(math.Ceil(float64(totalChecks) * 0.0005))
	if totalFailures > allowed {
		t.Fatalf("precision harness failure ratio %.5f exceeds 0.0005 (failures=%d, checks=%d, allowed=%d)", float64(totalFailures)/float64(totalChecks), totalFailures, totalChecks, allowed)
	}
	if totalChecks == 0 {
		t.Fatalf("precision harness executed zero checks")
	}
	t.Logf("precision harness verified %d checks with %d failures (allowed %d)", totalChecks, totalFailures, allowed)
}

type instrumentProfile struct {
	name       string
	symbol     string
	market     core.Market
	qtyScale   int
	qtyMin     int64
	qtyMax     int64
	priceScale int
	priceMin   int64
	priceMax   int64
}

type profileRunner struct {
	profile instrumentProfile
	router  *OrderRouter
	rng     *rand.Rand
	iter    int
	t       *testing.T
}

func newProfileRunner(t *testing.T, profile instrumentProfile, seed int64) *profileRunner {
	t.Helper()
	state := &precisionState{orders: make(map[string]precisionSnapshot)}
	dispatcher := &precisionDispatcher{state: state}
	translator := &precisionTranslator{profile: profile, state: state}
	caps, requirements := capabilityProfile(profile.market)
	options := []OrderRouterOption{WithCapabilities(caps)}
	for action, req := range requirements {
		options = append(options, WithCapabilityRequirement(action, req...))
	}
	router, err := NewOrderRouter(dispatcher, translator, options...)
	if err != nil {
		t.Fatalf("NewOrderRouter(%s): %v", profile.name, err)
	}
	return &profileRunner{
		profile: profile,
		router:  router,
		rng:     rand.New(rand.NewSource(seed)),
		t:       t,
	}
}

func (r *profileRunner) runIteration() (checks int, failures int) {
	r.iter++
	ctx := context.Background()
	qty := randomRational(r.rng, r.profile.qtyScale, r.profile.qtyMin, r.profile.qtyMax)
	price := randomRational(r.rng, r.profile.priceScale, r.profile.priceMin, r.profile.priceMax)
	clientID := fmt.Sprintf("%s-%d", r.profile.name, r.iter)
	side := core.SideBuy
	if r.iter%2 == 0 {
		side = core.SideSell
	}
	orderReq := core.OrderRequest{
		Symbol:      r.profile.symbol,
		Side:        side,
		Type:        core.TypeLimit,
		TimeInForce: core.GTC,
		Quantity:    qty,
		Price:       price,
		ClientID:    clientID,
	}
	placed, err := r.router.PlaceOrder(ctx, orderReq)
	if err != nil {
		r.t.Fatalf("PlaceOrder(%s): %v", r.profile.name, err)
	}
	checks++
	if !ratEqual(orderReq.Price, placed.AvgPrice) {
		failures++
	}
	checks++
	if !ratEqual(orderReq.Quantity, placed.FilledQty) {
		failures++
	}

	amend := r.randomAmendRequest(placed.ID, orderReq.Symbol, orderReq.Quantity, orderReq.Price)
	amended, err := r.router.AmendOrder(ctx, amend)
	if err != nil {
		r.t.Fatalf("AmendOrder(%s): %v", r.profile.name, err)
	}
	checks++
	if !ratEqual(amend.NewPrice, amended.AvgPrice) {
		failures++
	}
	checks++
	if !ratEqual(amend.NewQuantity, amended.FilledQty) {
		failures++
	}

	queried, err := r.router.GetOrder(ctx, OrderQueryRequest{Symbol: orderReq.Symbol, OrderID: placed.ID})
	if err != nil {
		r.t.Fatalf("GetOrder(%s): %v", r.profile.name, err)
	}
	checks++
	if !ratEqual(amend.NewPrice, queried.AvgPrice) {
		failures++
	}
	checks++
	if !ratEqual(amend.NewQuantity, queried.FilledQty) {
		failures++
	}

	if err := r.router.CancelOrder(ctx, OrderCancelRequest{Symbol: orderReq.Symbol, OrderID: placed.ID}); err != nil {
		r.t.Fatalf("CancelOrder(%s): %v", r.profile.name, err)
	}

	return checks, failures
}

func (r *profileRunner) randomAmendRequest(orderID, symbol string, baseQty, basePrice *big.Rat) OrderAmendRequest {
	stepQty := randomStep(r.rng, r.profile.qtyScale)
	newQty := new(big.Rat).Add(new(big.Rat).Set(baseQty), stepQty)
	stepPrice := randomStep(r.rng, r.profile.priceScale)
	newPrice := new(big.Rat).Add(new(big.Rat).Set(basePrice), stepPrice)
	return OrderAmendRequest{
		Symbol:         symbol,
		OrderID:        orderID,
		ClientOrderID:  "",
		NewQuantity:    newQty,
		NewPrice:       newPrice,
		NewTimeInForce: core.GTC,
	}
}

func capabilityProfile(market core.Market) (capabilities.Set, map[OrderAction][]capabilities.Capability) {
	switch market {
	case core.MarketLinearFutures:
		return capabilities.Of(
				capabilities.CapabilityLinearTradingREST,
				capabilities.CapabilityTradingLinearAmend,
				capabilities.CapabilityTradingLinearCancel,
			), map[OrderAction][]capabilities.Capability{
				ActionPlace:  {capabilities.CapabilityLinearTradingREST},
				ActionAmend:  {capabilities.CapabilityLinearTradingREST, capabilities.CapabilityTradingLinearAmend},
				ActionGet:    {capabilities.CapabilityLinearTradingREST},
				ActionCancel: {capabilities.CapabilityTradingLinearCancel},
			}
	case core.MarketInverseFutures:
		return capabilities.Of(
				capabilities.CapabilityInverseTradingREST,
				capabilities.CapabilityTradingInverseAmend,
				capabilities.CapabilityTradingInverseCancel,
			), map[OrderAction][]capabilities.Capability{
				ActionPlace:  {capabilities.CapabilityInverseTradingREST},
				ActionAmend:  {capabilities.CapabilityInverseTradingREST, capabilities.CapabilityTradingInverseAmend},
				ActionGet:    {capabilities.CapabilityInverseTradingREST},
				ActionCancel: {capabilities.CapabilityTradingInverseCancel},
			}
	default:
		return capabilities.Of(
				capabilities.CapabilitySpotTradingREST,
				capabilities.CapabilityTradingSpotAmend,
				capabilities.CapabilityTradingSpotCancel,
			), map[OrderAction][]capabilities.Capability{
				ActionPlace:  {capabilities.CapabilitySpotTradingREST},
				ActionAmend:  {capabilities.CapabilitySpotTradingREST, capabilities.CapabilityTradingSpotAmend},
				ActionGet:    {capabilities.CapabilitySpotTradingREST},
				ActionCancel: {capabilities.CapabilityTradingSpotCancel},
			}
	}
}

type precisionTranslator struct {
	profile instrumentProfile
	state   *precisionState
}

func (t *precisionTranslator) PrepareCreate(ctx context.Context, req core.OrderRequest) (DispatchSpec, error) {
	message := RESTMessage{
		API:    string(t.profile.market),
		Method: http.MethodPost,
		Path:   "/order",
		Signed: true,
		Query: map[string]string{
			"symbol":   req.Symbol,
			"side":     string(req.Side),
			"quantity": numeric.Format(req.Quantity, t.profile.qtyScale),
			"price":    numeric.Format(req.Price, t.profile.priceScale),
			"client":   req.ClientID,
		},
	}
	if tif := string(req.TimeInForce); tif != "" {
		message.Query["tif"] = tif
	}
	env := &precisionOrderEnvelope{}
	return DispatchSpec{
		Message: message,
		Into:    env,
		Decode:  t.decodeOrder(req.Symbol),
	}, nil
}

func (t *precisionTranslator) PrepareAmend(ctx context.Context, req OrderAmendRequest) (DispatchSpec, error) {
	query := map[string]string{
		"orderId": req.OrderID,
	}
	if req.NewQuantity != nil {
		query["quantity"] = numeric.Format(req.NewQuantity, t.profile.qtyScale)
	}
	if req.NewPrice != nil {
		query["price"] = numeric.Format(req.NewPrice, t.profile.priceScale)
	}
	if tif := string(req.NewTimeInForce); tif != "" {
		query["tif"] = tif
	}
	env := &precisionOrderEnvelope{}
	return DispatchSpec{
		Message: RESTMessage{API: string(t.profile.market), Method: http.MethodPut, Path: "/order", Signed: true, Query: query},
		Into:    env,
		Decode:  t.decodeOrder(req.Symbol),
	}, nil
}

func (t *precisionTranslator) PrepareGet(ctx context.Context, req OrderQueryRequest) (DispatchSpec, error) {
	query := map[string]string{
		"orderId": req.OrderID,
	}
	env := &precisionOrderEnvelope{}
	return DispatchSpec{
		Message: RESTMessage{API: string(t.profile.market), Method: http.MethodGet, Path: "/order", Query: query},
		Into:    env,
		Decode:  t.decodeOrder(req.Symbol),
	}, nil
}

func (t *precisionTranslator) PrepareCancel(ctx context.Context, req OrderCancelRequest) (DispatchSpec, error) {
	query := map[string]string{
		"orderId": req.OrderID,
	}
	return DispatchSpec{
		Message: RESTMessage{API: string(t.profile.market), Method: http.MethodDelete, Path: "/order", Signed: true, Query: query},
	}, nil
}

func (t *precisionTranslator) decodeOrder(symbol string) func(any) (core.Order, error) {
	return func(out any) (core.Order, error) {
		env, ok := out.(*precisionOrderEnvelope)
		if !ok || env == nil {
			return core.Order{}, fmt.Errorf("precision decode: unexpected envelope %T", out)
		}
		order := core.Order{ID: env.OrderID, Symbol: symbol, Status: core.OrderNew}
		if env.Status != "" {
			if status, err := OrderStatusFromString(env.Status); err == nil {
				order.Status = status
			}
		}
		if qty, ok := numeric.Parse(env.ExecutedQty); ok {
			order.FilledQty = qty
		}
		if price, ok := numeric.Parse(env.AvgPrice); ok {
			order.AvgPrice = price
		}
		return order, nil
	}
}

type precisionDispatcher struct {
	state *precisionState
}

func (d *precisionDispatcher) Dispatch(ctx context.Context, msg RESTMessage, out any) error {
	switch msg.Method {
	case http.MethodPost:
		return d.dispatchCreate(msg, out)
	case http.MethodPut:
		return d.dispatchAmend(msg, out)
	case http.MethodGet:
		return d.dispatchGet(msg, out)
	case http.MethodDelete:
		return d.dispatchCancel(msg)
	default:
		return fmt.Errorf("precision dispatcher: unsupported method %s", msg.Method)
	}
}

func (d *precisionDispatcher) dispatchCreate(msg RESTMessage, out any) error {
	d.state.nextID++
	id := fmt.Sprintf("order-%d", d.state.nextID)
	snap := precisionSnapshot{
		quantity: msg.Query["quantity"],
		price:    msg.Query["price"],
	}
	d.state.orders[id] = snap
	if env, ok := out.(*precisionOrderEnvelope); ok && env != nil {
		env.OrderID = id
		env.ExecutedQty = snap.quantity
		env.AvgPrice = snap.price
		env.Status = "NEW"
	}
	return nil
}

func (d *precisionDispatcher) dispatchAmend(msg RESTMessage, out any) error {
	id := msg.Query["orderId"]
	snap := d.state.orders[id]
	if q := msg.Query["quantity"]; q != "" {
		snap.quantity = q
	}
	if p := msg.Query["price"]; p != "" {
		snap.price = p
	}
	d.state.orders[id] = snap
	if env, ok := out.(*precisionOrderEnvelope); ok && env != nil {
		env.OrderID = id
		env.ExecutedQty = snap.quantity
		env.AvgPrice = snap.price
		env.Status = "FILLED"
	}
	return nil
}

func (d *precisionDispatcher) dispatchGet(msg RESTMessage, out any) error {
	id := msg.Query["orderId"]
	snap := d.state.orders[id]
	if env, ok := out.(*precisionOrderEnvelope); ok && env != nil {
		env.OrderID = id
		env.ExecutedQty = snap.quantity
		env.AvgPrice = snap.price
		env.Status = "FILLED"
	}
	return nil
}

func (d *precisionDispatcher) dispatchCancel(msg RESTMessage) error {
	id := msg.Query["orderId"]
	delete(d.state.orders, id)
	return nil
}

type precisionState struct {
	nextID int
	orders map[string]precisionSnapshot
}

type precisionSnapshot struct {
	quantity string
	price    string
}

type precisionOrderEnvelope struct {
	OrderID     string `json:"orderId"`
	ExecutedQty string `json:"executedQty"`
	AvgPrice    string `json:"avgPrice"`
	Status      string `json:"status"`
}

func ratEqual(a, b *big.Rat) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Cmp(b) == 0
}

func randomRational(rng *rand.Rand, scale int, minWhole, maxWhole int64) *big.Rat {
	if maxWhole < minWhole {
		swap := minWhole
		minWhole = maxWhole
		maxWhole = swap
	}
	den := pow10Int(scale)
	wholeRange := maxWhole - minWhole + 1
	if wholeRange <= 0 {
		wholeRange = 1
	}
	whole := rng.Int63n(wholeRange) + minWhole
	var frac int64
	if den > 1 {
		frac = rng.Int63n(den)
	}
	num := whole*den + frac
	if num <= 0 {
		num = 1
	}
	return new(big.Rat).SetFrac(big.NewInt(num), big.NewInt(den))
}

func randomStep(rng *rand.Rand, scale int) *big.Rat {
	den := pow10Int(scale)
	if den == 1 {
		num := rng.Int63n(5) + 1
		return new(big.Rat).SetFrac(big.NewInt(num), big.NewInt(1))
	}
	upper := den / 10
	if upper <= 0 {
		upper = 1
	}
	num := rng.Int63n(upper) + 1
	return new(big.Rat).SetFrac(big.NewInt(num), big.NewInt(den))
}

func pow10Int(scale int) int64 {
	result := int64(1)
	for i := 0; i < scale; i++ {
		result *= 10
	}
	return result
}
