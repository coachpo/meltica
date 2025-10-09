package routing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/coachpo/meltica/core"
	capabilities "github.com/coachpo/meltica/core/exchanges/capabilities"
	"github.com/coachpo/meltica/errs"
)

func TestOrderRouterPlaceLimit(t *testing.T) {
	fixture := loadOrderFixture(t, "place_limit.json")
	dispatcher := &fixtureDispatcher{t: t, fixture: fixture}
	translator := &fixtureTranslator{place: fixture}

	router := mustNewOrderRouter(t, dispatcher, translator, capabilities.Of(
		capabilities.CapabilitySpotTradingREST,
		capabilities.CapabilityTradingSpotCancel,
		capabilities.CapabilityTradingSpotAmend,
	))

	req := fixture.buildOrderRequest(t)
	order, err := router.PlaceOrder(context.Background(), req)
	if err != nil {
		t.Fatalf("PlaceOrder returned error: %v", err)
	}
	fixture.assertOrder(t, order)
}

func TestOrderRouterAmendOrder(t *testing.T) {
	place := loadOrderFixture(t, "place_limit.json")
	amend := loadOrderFixture(t, "amend_price.json")
	dispatchMap := map[string]orderFixture{
		dispatcherKey(place.REST.Method, place.REST.Path): place,
		dispatcherKey(amend.REST.Method, amend.REST.Path): amend,
	}
	dispatcher := &multiFixtureDispatcher{t: t, fixtures: dispatchMap}
	translator := &fixtureTranslator{place: place, amend: amend}

	router := mustNewOrderRouter(t, dispatcher, translator, capabilities.Of(
		capabilities.CapabilitySpotTradingREST,
		capabilities.CapabilityTradingSpotAmend,
		capabilities.CapabilityTradingSpotCancel,
	))

	// Place baseline order to prime translator state.
	_, err := router.PlaceOrder(context.Background(), place.buildOrderRequest(t))
	if err != nil {
		t.Fatalf("PlaceOrder returned error: %v", err)
	}

	update := amend.buildAmendRequest(t)
	order, err := router.AmendOrder(context.Background(), update)
	if err != nil {
		t.Fatalf("AmendOrder returned error: %v", err)
	}
	amend.assertOrder(t, order)
}

func TestOrderRouterGetOrder(t *testing.T) {
	query := loadOrderFixture(t, "get_status.json")
	dispatcher := &fixtureDispatcher{t: t, fixture: query}
	translator := &fixtureTranslator{query: query}

	router := mustNewOrderRouter(t, dispatcher, translator, capabilities.Of(
		capabilities.CapabilitySpotTradingREST,
		capabilities.CapabilityTradingSpotAmend,
		capabilities.CapabilityTradingSpotCancel,
	))

	result, err := router.GetOrder(context.Background(), query.buildQueryRequest())
	if err != nil {
		t.Fatalf("GetOrder returned error: %v", err)
	}
	query.assertOrder(t, result)
}

func TestOrderRouterCancelOrder(t *testing.T) {
	cancel := loadOrderFixture(t, "cancel.json")
	dispatcher := &fixtureDispatcher{t: t, fixture: cancel}
	translator := &fixtureTranslator{cancel: cancel}

	router := mustNewOrderRouter(t, dispatcher, translator, capabilities.Of(
		capabilities.CapabilitySpotTradingREST,
		capabilities.CapabilityTradingSpotCancel,
	))

	if err := router.CancelOrder(context.Background(), cancel.buildCancelRequest()); err != nil {
		t.Fatalf("CancelOrder returned error: %v", err)
	}
}

func TestOrderRouterCapabilityMissing(t *testing.T) {
	cancel := loadOrderFixture(t, "cancel.json")
	dispatcher := &fixtureDispatcher{t: t, fixture: cancel}
	translator := &fixtureTranslator{cancel: cancel}

	router := mustNewOrderRouter(t, dispatcher, translator, capabilities.Of(capabilities.CapabilitySpotTradingREST))

	err := router.CancelOrder(context.Background(), cancel.buildCancelRequest())
	if err == nil {
		t.Fatal("expected capability error but got nil")
	}
	var e *errs.E
	if !errors.As(err, &e) {
		t.Fatalf("expected errs.E, got %T", err)
	}
	if e.Canonical != errs.CanonicalCapabilityMissing {
		t.Fatalf("expected canonical capability missing, got %s", e.Canonical)
	}
}

func TestOrderRouterCapabilitiesReported(t *testing.T) {
	set := capabilities.Of(capabilities.CapabilitySpotTradingREST)
	router, err := NewOrderRouter(noopDispatcher{}, noopTranslator{}, WithCapabilities(set))
	if err != nil {
		t.Fatalf("NewOrderRouter returned error: %v", err)
	}
	if router.Capabilities() != set {
		t.Fatalf("expected capability set %v, got %v", set, router.Capabilities())
	}
}

// --- Fixtures ----------------------------------------------------------------

type orderFixture struct {
	Name      string          `json:"name"`
	Canonical canonicalSpec   `json:"canonical"`
	REST      restSpec        `json:"rest"`
	Response  json.RawMessage `json:"response"`
	Expected  expectedOrder   `json:"expected"`
}

type canonicalSpec struct {
	Symbol       string `json:"symbol"`
	Side         string `json:"side"`
	Type         string `json:"type"`
	TimeInForce  string `json:"time_in_force"`
	Quantity     string `json:"quantity"`
	Price        string `json:"price"`
	ClientID     string `json:"client_id"`
	OrderID      string `json:"order_id"`
	OrigClientID string `json:"orig_client_id"`
	NewQuantity  string `json:"new_quantity"`
	NewPrice     string `json:"new_price"`
}

type restSpec struct {
	API     string            `json:"api"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Signed  bool              `json:"signed"`
	Query   map[string]string `json:"query"`
	Headers map[string]string `json:"headers"`
}

type expectedOrder struct {
	ID            string `json:"id"`
	Status        string `json:"status"`
	FilledQty     string `json:"filled_quantity"`
	AveragePrice  string `json:"average_price"`
	VenueTimeNano int64  `json:"venue_time_unix_nano"`
}

func loadOrderFixture(t *testing.T, name string) orderFixture {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(filename), "testdata", "orders", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	var fixture orderFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode fixture %s: %v", name, err)
	}
	if fixture.Name == "" {
		fixture.Name = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return fixture
}

func (f orderFixture) buildOrderRequest(t *testing.T) core.OrderRequest {
	return core.OrderRequest{
		Symbol:      f.Canonical.Symbol,
		Side:        mustOrderSide(t, f.Canonical.Side),
		Type:        mustOrderType(t, f.Canonical.Type),
		TimeInForce: mustTimeInForce(t, f.Canonical.TimeInForce),
		Quantity:    parseRat(t, f.Canonical.Quantity),
		Price:       parseRat(t, f.Canonical.Price),
		ClientID:    f.Canonical.ClientID,
	}
}

func (f orderFixture) buildAmendRequest(t *testing.T) OrderAmendRequest {
	return OrderAmendRequest{
		Symbol:         f.Canonical.Symbol,
		OrderID:        f.Canonical.OrderID,
		ClientOrderID:  f.Canonical.OrigClientID,
		NewQuantity:    parseRat(t, f.Canonical.NewQuantity),
		NewPrice:       parseRat(t, f.Canonical.NewPrice),
		NewTimeInForce: mustTimeInForce(t, f.Canonical.TimeInForce),
	}
}

func (f orderFixture) buildQueryRequest() OrderQueryRequest {
	return OrderQueryRequest{
		Symbol:        f.Canonical.Symbol,
		OrderID:       f.Canonical.OrderID,
		ClientOrderID: f.Canonical.OrigClientID,
	}
}

func (f orderFixture) buildCancelRequest() OrderCancelRequest {
	return OrderCancelRequest{
		Symbol:        f.Canonical.Symbol,
		OrderID:       f.Canonical.OrderID,
		ClientOrderID: f.Canonical.OrigClientID,
	}
}

func (f orderFixture) assertOrder(t *testing.T, order core.Order) {
	t.Helper()
	if order.Symbol != f.Canonical.Symbol {
		t.Fatalf("expected symbol %s, got %s", f.Canonical.Symbol, order.Symbol)
	}
	if f.Expected.ID != "" && order.ID != f.Expected.ID {
		t.Fatalf("expected order id %s, got %s", f.Expected.ID, order.ID)
	}
	if f.Expected.Status != "" {
		expectedStatus := mustOrderStatus(t, f.Expected.Status)
		if order.Status != expectedStatus {
			t.Fatalf("expected status %s, got %s", expectedStatus, order.Status)
		}
	}
	if f.Expected.FilledQty != "" {
		expected := parseRat(t, f.Expected.FilledQty)
		if order.FilledQty == nil || order.FilledQty.Cmp(expected) != 0 {
			t.Fatalf("expected filled qty %s, got %v", expected.RatString(), order.FilledQty)
		}
	}
	if f.Expected.AveragePrice != "" {
		expected := parseRat(t, f.Expected.AveragePrice)
		if order.AvgPrice == nil || order.AvgPrice.Cmp(expected) != 0 {
			t.Fatalf("expected avg price %s, got %v", expected.RatString(), order.AvgPrice)
		}
	}
}

// --- Translator & dispatcher -------------------------------------------------

type fixtureTranslator struct {
	place  orderFixture
	amend  orderFixture
	query  orderFixture
	cancel orderFixture

	plcd bool
}

func (f *fixtureTranslator) PrepareCreate(ctx context.Context, req core.OrderRequest) (DispatchSpec, error) {
	fixture := f.place
	if fixture.Name == "" {
		return DispatchSpec{}, errors.New("place fixture not configured")
	}
	if err := fixture.validateOrderRequest(req); err != nil {
		return DispatchSpec{}, err
	}
	return DispatchSpec{
		Message: fixture.toRESTMessage(),
		Into: &struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
			Symbol  string `json:"symbol"`
		}{},
		Decode: func(out any) (core.Order, error) {
			res := out.(*struct {
				OrderID string `json:"orderId"`
				Status  string `json:"status"`
				Symbol  string `json:"symbol"`
			})
			status, err := OrderStatusFromString(res.Status)
			if err != nil {
				return core.Order{}, err
			}
			return core.Order{ID: res.OrderID, Symbol: fixture.Canonical.Symbol, Status: status}, nil
		},
	}, nil
}

func (f *fixtureTranslator) PrepareAmend(ctx context.Context, req OrderAmendRequest) (DispatchSpec, error) {
	fixture := f.amend
	if fixture.Name == "" {
		return DispatchSpec{}, errors.New("amend fixture not configured")
	}
	if err := fixture.validateAmendRequest(req); err != nil {
		return DispatchSpec{}, err
	}
	return DispatchSpec{
		Message: fixture.toRESTMessage(),
		Into: &struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
		}{},
		Decode: func(out any) (core.Order, error) {
			res := out.(*struct {
				OrderID string `json:"orderId"`
				Status  string `json:"status"`
			})
			status, err := OrderStatusFromString(res.Status)
			if err != nil {
				return core.Order{}, err
			}
			return core.Order{ID: res.OrderID, Symbol: fixture.Canonical.Symbol, Status: status}, nil
		},
	}, nil
}

func (f *fixtureTranslator) PrepareGet(ctx context.Context, req OrderQueryRequest) (DispatchSpec, error) {
	fixture := f.query
	if fixture.Name == "" {
		return DispatchSpec{}, errors.New("query fixture not configured")
	}
	if err := fixture.validateQueryRequest(req); err != nil {
		return DispatchSpec{}, err
	}
	return DispatchSpec{
		Message: fixture.toRESTMessage(),
		Into: &struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
		}{},
		Decode: func(out any) (core.Order, error) {
			res := out.(*struct {
				OrderID string `json:"orderId"`
				Status  string `json:"status"`
			})
			status, err := OrderStatusFromString(res.Status)
			if err != nil {
				return core.Order{}, err
			}
			order := core.Order{ID: res.OrderID, Symbol: fixture.Canonical.Symbol, Status: status}
			return order, nil
		},
	}, nil
}

func (f *fixtureTranslator) PrepareCancel(ctx context.Context, req OrderCancelRequest) (DispatchSpec, error) {
	fixture := f.cancel
	if fixture.Name == "" {
		return DispatchSpec{}, errors.New("cancel fixture not configured")
	}
	if err := fixture.validateCancelRequest(req); err != nil {
		return DispatchSpec{}, err
	}
	return DispatchSpec{Message: fixture.toRESTMessage()}, nil
}

// Additional helper methods on orderFixture for validation and conversion.

func (f orderFixture) toRESTMessage() RESTMessage {
	msg := RESTMessage{
		API:    f.REST.API,
		Method: f.REST.Method,
		Path:   f.REST.Path,
		Query:  copyStringMap(f.REST.Query),
		Signed: f.REST.Signed,
		Header: make(http.Header),
	}
	for k, v := range f.REST.Headers {
		msg.Header.Set(k, v)
	}
	return msg
}

func (f orderFixture) validateOrderRequest(req core.OrderRequest) error {
	if f.Canonical.Symbol != "" && req.Symbol != f.Canonical.Symbol {
		return fmt.Errorf("unexpected symbol: got %s want %s", req.Symbol, f.Canonical.Symbol)
	}
	if f.Canonical.Side != "" {
		expected, err := OrderSideFromString(f.Canonical.Side)
		if err != nil {
			return err
		}
		if req.Side != expected {
			return fmt.Errorf("unexpected side: got %s want %s", req.Side, expected)
		}
	}
	if f.Canonical.Type != "" {
		expected, err := OrderTypeFromString(f.Canonical.Type)
		if err != nil {
			return err
		}
		if req.Type != expected {
			return fmt.Errorf("unexpected order type: got %s want %s", req.Type, expected)
		}
	}
	if f.Canonical.TimeInForce != "" {
		expected, err := TimeInForceFromString(f.Canonical.TimeInForce)
		if err != nil {
			return err
		}
		if req.TimeInForce != expected {
			return fmt.Errorf("unexpected tif: got %s want %s", req.TimeInForce, expected)
		}
	}
	if err := compareRat(req.Quantity, f.Canonical.Quantity, "quantity"); err != nil {
		return err
	}
	if err := compareRat(req.Price, f.Canonical.Price, "price"); err != nil {
		return err
	}
	if f.Canonical.ClientID != "" && req.ClientID != f.Canonical.ClientID {
		return fmt.Errorf("unexpected client id: got %s want %s", req.ClientID, f.Canonical.ClientID)
	}
	return nil
}

type noopTranslator struct{}

func (noopTranslator) PrepareCreate(context.Context, core.OrderRequest) (DispatchSpec, error) {
	return DispatchSpec{}, nil
}

func (noopTranslator) PrepareAmend(context.Context, OrderAmendRequest) (DispatchSpec, error) {
	return DispatchSpec{}, nil
}

func (noopTranslator) PrepareGet(context.Context, OrderQueryRequest) (DispatchSpec, error) {
	return DispatchSpec{}, nil
}

func (noopTranslator) PrepareCancel(context.Context, OrderCancelRequest) (DispatchSpec, error) {
	return DispatchSpec{}, nil
}

type noopDispatcher struct{}

func (noopDispatcher) Dispatch(context.Context, RESTMessage, any) error { return nil }

func (f orderFixture) validateAmendRequest(req OrderAmendRequest) error {
	if f.Canonical.Symbol != "" && req.Symbol != f.Canonical.Symbol {
		return fmt.Errorf("unexpected amend symbol: got %s want %s", req.Symbol, f.Canonical.Symbol)
	}
	if f.Canonical.OrderID != "" && req.OrderID != f.Canonical.OrderID {
		return fmt.Errorf("unexpected amend order id: got %s want %s", req.OrderID, f.Canonical.OrderID)
	}
	if f.Canonical.OrigClientID != "" && req.ClientOrderID != f.Canonical.OrigClientID {
		return fmt.Errorf("unexpected amend client id: got %s want %s", req.ClientOrderID, f.Canonical.OrigClientID)
	}
	if err := compareRat(req.NewQuantity, f.Canonical.NewQuantity, "new quantity"); err != nil {
		return err
	}
	if err := compareRat(req.NewPrice, f.Canonical.NewPrice, "new price"); err != nil {
		return err
	}
	if f.Canonical.TimeInForce != "" {
		expected, err := TimeInForceFromString(f.Canonical.TimeInForce)
		if err != nil {
			return err
		}
		if req.NewTimeInForce != expected {
			return fmt.Errorf("unexpected amend tif: got %s want %s", req.NewTimeInForce, expected)
		}
	}
	return nil
}

func (f orderFixture) validateQueryRequest(req OrderQueryRequest) error {
	if f.Canonical.Symbol != "" && req.Symbol != f.Canonical.Symbol {
		return fmt.Errorf("unexpected query symbol: got %s want %s", req.Symbol, f.Canonical.Symbol)
	}
	if f.Canonical.OrderID != "" && req.OrderID != f.Canonical.OrderID {
		return fmt.Errorf("unexpected query order id: got %s want %s", req.OrderID, f.Canonical.OrderID)
	}
	if f.Canonical.OrigClientID != "" && req.ClientOrderID != f.Canonical.OrigClientID {
		return fmt.Errorf("unexpected query client id: got %s want %s", req.ClientOrderID, f.Canonical.OrigClientID)
	}
	return nil
}

func (f orderFixture) validateCancelRequest(req OrderCancelRequest) error {
	if f.Canonical.Symbol != "" && req.Symbol != f.Canonical.Symbol {
		return fmt.Errorf("unexpected cancel symbol: got %s want %s", req.Symbol, f.Canonical.Symbol)
	}
	if f.Canonical.OrderID != "" && req.OrderID != f.Canonical.OrderID {
		return fmt.Errorf("unexpected cancel order id: got %s want %s", req.OrderID, f.Canonical.OrderID)
	}
	if f.Canonical.OrigClientID != "" && req.ClientOrderID != f.Canonical.OrigClientID {
		return fmt.Errorf("unexpected cancel client id: got %s want %s", req.ClientOrderID, f.Canonical.OrigClientID)
	}
	return nil
}

func (f orderFixture) assertRESTMessage(t *testing.T, msg RESTMessage) {
	if msg.API != f.REST.API {
		t.Fatalf("unexpected api: got %s want %s", msg.API, f.REST.API)
	}
	if msg.Method != f.REST.Method {
		t.Fatalf("unexpected method: got %s want %s", msg.Method, f.REST.Method)
	}
	if msg.Path != f.REST.Path {
		t.Fatalf("unexpected path: got %s want %s", msg.Path, f.REST.Path)
	}
	if msg.Signed != f.REST.Signed {
		t.Fatalf("unexpected signed: got %v want %v", msg.Signed, f.REST.Signed)
	}
	if len(msg.Query) != len(f.REST.Query) {
		t.Fatalf("unexpected query size: got %d want %d", len(msg.Query), len(f.REST.Query))
	}
	for k, v := range f.REST.Query {
		if got := msg.Query[k]; got != v {
			t.Fatalf("query mismatch for %s: got %s want %s", k, got, v)
		}
	}
	for k, v := range f.REST.Headers {
		if got := msg.Header.Get(k); got != v {
			t.Fatalf("header mismatch for %s: got %s want %s", k, got, v)
		}
	}
}

// --- Dispatchers ------------------------------------------------------------

type fixtureDispatcher struct {
	t       *testing.T
	fixture orderFixture
}

func (d *fixtureDispatcher) Dispatch(_ context.Context, msg RESTMessage, out any) error {
	d.fixture.assertRESTMessage(d.t, msg)
	if out != nil && len(d.fixture.Response) > 0 {
		if err := json.Unmarshal(d.fixture.Response, out); err != nil {
			return err
		}
	}
	return nil
}

type multiFixtureDispatcher struct {
	t        *testing.T
	fixtures map[string]orderFixture
}

func (d *multiFixtureDispatcher) Dispatch(ctx context.Context, msg RESTMessage, out any) error {
	key := dispatcherKey(msg.Method, msg.Path)
	fixture, ok := d.fixtures[key]
	if !ok {
		d.t.Fatalf("unexpected dispatch key %s", key)
	}
	fd := &fixtureDispatcher{t: d.t, fixture: fixture}
	return fd.Dispatch(ctx, msg, out)
}

func dispatcherKey(method, path string) string {
	return method + " " + path
}

func copyStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// --- Helpers ----------------------------------------------------------------

func mustNewOrderRouter(t *testing.T, dispatcher RESTDispatcher, translator OrderTranslator, caps capabilities.Set) *OrderRouter {
	router, err := NewOrderRouter(dispatcher, translator,
		WithCapabilities(caps),
		WithCapabilityRequirement(ActionPlace, capabilities.CapabilitySpotTradingREST),
		WithCapabilityRequirement(ActionAmend, capabilities.CapabilitySpotTradingREST, capabilities.CapabilityTradingSpotAmend),
		WithCapabilityRequirement(ActionGet, capabilities.CapabilitySpotTradingREST),
		WithCapabilityRequirement(ActionCancel, capabilities.CapabilityTradingSpotCancel),
	)
	if err != nil {
		t.Fatalf("NewOrderRouter error: %v", err)
	}
	return router
}

func parseRat(t *testing.T, value string) *big.Rat {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	r := new(big.Rat)
	if _, ok := r.SetString(value); !ok {
		t.Fatalf("invalid rational %q", value)
	}
	return r
}

func compareRat(actual *big.Rat, expected string, label string) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	exp := new(big.Rat)
	if _, ok := exp.SetString(expected); !ok {
		return fmt.Errorf("invalid expected %s value %q", label, expected)
	}
	if actual == nil || actual.Cmp(exp) != 0 {
		return fmt.Errorf("unexpected %s: got %v want %s", label, actual, exp)
	}
	return nil
}

func mustOrderSide(t *testing.T, value string) core.OrderSide {
	side, err := OrderSideFromString(value)
	if err != nil {
		t.Fatalf("parse order side: %v", err)
	}
	return side
}

func mustOrderType(t *testing.T, value string) core.OrderType {
	ot, err := OrderTypeFromString(value)
	if err != nil {
		t.Fatalf("parse order type: %v", err)
	}
	return ot
}

func mustTimeInForce(t *testing.T, value string) core.TimeInForce {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	tif, err := TimeInForceFromString(value)
	if err != nil {
		t.Fatalf("parse tif: %v", err)
	}
	return tif
}

func mustOrderStatus(t *testing.T, value string) core.OrderStatus {
	status, err := OrderStatusFromString(value)
	if err != nil {
		t.Fatalf("parse status: %v", err)
	}
	return status
}
