package httpserver

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/coachpo/meltica/internal/app/dispatcher"
	"github.com/coachpo/meltica/internal/app/lambda/js"
	lambdaruntime "github.com/coachpo/meltica/internal/app/lambda/runtime"
	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/domain/schema"
	"github.com/coachpo/meltica/internal/infra/bus/eventbus"
	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/coachpo/meltica/internal/infra/pool"
	strategiestest "github.com/coachpo/meltica/internal/testutil/strategies"
)

type controlPlaneRouteClass string

const (
	controlPlaneRouteReadOnly controlPlaneRouteClass = "read-only"
	controlPlaneRouteMutable  controlPlaneRouteClass = "mutable"
)

type controlPlaneRouteExpectation struct {
	status int
	body   string
}

type controlPlaneRouteInventoryEntry struct {
	family         string
	path           string
	class          controlPlaneRouteClass
	allowed        map[string]controlPlaneRouteExpectation
	unsupported    []string
	expectedAllow  string
	futureAuthNote string
}

func controlPlaneRouteInventory() []controlPlaneRouteInventoryEntry {
	return []controlPlaneRouteInventoryEntry{
		{
			family: "strategy catalog",
			path:   strategiesPath,
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy detail",
			path:   strategyDetailPrefix + "logging",
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy modules collection",
			path:   strategyModulesPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:  {status: http.StatusOK},
				http.MethodPost: {status: http.StatusBadRequest, body: `{}`},
			},
			unsupported:    []string{http.MethodPut, http.MethodDelete},
			expectedAllow:  "GET, POST",
			futureAuthNote: "POST creates or refreshes persisted strategy module source",
		},
		{
			family: "strategy module resource",
			path:   strategyModulePrefix + "logging",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:    {status: http.StatusOK},
				http.MethodPut:    {status: http.StatusBadRequest, body: `{}`},
				http.MethodDelete: {status: http.StatusBadRequest},
			},
			unsupported:    []string{http.MethodPost},
			expectedAllow:  "GET, PUT, DELETE",
			futureAuthNote: "PUT updates module source and DELETE removes a module",
		},
		{
			family: "strategy module source",
			path:   strategyModulePrefix + "logging" + strategySourceSuffix,
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy module usage",
			path:   strategyModulePrefix + "logging" + strategyUsageSuffix,
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy module tag",
			path:   strategyModulePrefix + "logging/tags/v1.0.1",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPut:    {status: http.StatusBadRequest, body: `{}`},
				http.MethodDelete: {status: http.StatusBadRequest},
			},
			unsupported:    []string{http.MethodGet, http.MethodPost},
			expectedAllow:  "PUT, DELETE",
			futureAuthNote: "PUT assigns a tag and DELETE removes a tag alias",
		},
		{
			family: "strategy refresh",
			path:   strategyRefreshPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPost: {status: http.StatusOK, body: `{}`},
			},
			unsupported:    []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			expectedAllow:  "POST",
			futureAuthNote: "POST reloads strategy registry state",
		},
		{
			family: "strategy registry export",
			path:   strategyRegistryPath,
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "providers collection",
			path:   providersPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:  {status: http.StatusOK},
				http.MethodPost: {status: http.StatusBadRequest, body: `{}`},
			},
			unsupported:    []string{http.MethodPut, http.MethodDelete},
			expectedAllow:  "GET, POST",
			futureAuthNote: "POST creates provider runtime definitions",
		},
		{
			family: "provider resource",
			path:   providerDetailPrefix + "binance",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:    {status: http.StatusOK},
				http.MethodPut:    {status: http.StatusBadRequest, body: `{}`},
				http.MethodDelete: {status: http.StatusConflict},
			},
			unsupported:    []string{http.MethodPost},
			expectedAllow:  "DELETE, GET, PUT",
			futureAuthNote: "PUT updates provider config and DELETE removes provider runtime definitions",
		},
		{
			family: "provider start action",
			path:   providerDetailPrefix + "binance/start",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPost: {status: http.StatusAccepted},
			},
			unsupported:    []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			expectedAllow:  "POST",
			futureAuthNote: "POST starts a configured provider asynchronously",
		},
		{
			family: "provider stop action",
			path:   providerDetailPrefix + "running/stop",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPost: {status: http.StatusOK},
			},
			unsupported:    []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			expectedAllow:  "POST",
			futureAuthNote: "POST stops a running provider",
		},
		{
			family: "provider balances",
			path:   providerDetailPrefix + "binance/balances",
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "adapters collection",
			path:   adaptersPath,
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "adapter detail",
			path:   adapterDetailPrefix + "stub",
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusNotFound},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy instances collection",
			path:   instancesPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:  {status: http.StatusOK},
				http.MethodPost: {status: http.StatusBadRequest, body: `{}`},
			},
			unsupported:    []string{http.MethodPut, http.MethodDelete},
			expectedAllow:  "GET, POST",
			futureAuthNote: "POST creates strategy instances",
		},
		{
			family: "strategy instance resource",
			path:   instanceDetailPrefix + "logging-alpha",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:    {status: http.StatusOK},
				http.MethodPut:    {status: http.StatusBadRequest, body: `{}`},
				http.MethodDelete: {status: http.StatusOK},
			},
			unsupported:    []string{http.MethodPost},
			expectedAllow:  "DELETE, GET, PUT",
			futureAuthNote: "PUT updates and DELETE removes strategy instances",
		},
		{
			family: "strategy instance start action",
			path:   instanceDetailPrefix + "logging-alpha/start",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPost: {status: http.StatusBadRequest},
			},
			unsupported:    []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			expectedAllow:  "POST",
			futureAuthNote: "POST starts a strategy instance",
		},
		{
			family: "strategy instance stop action",
			path:   instanceDetailPrefix + "running-alpha/stop",
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodPost: {status: http.StatusOK},
			},
			unsupported:    []string{http.MethodGet, http.MethodPut, http.MethodDelete},
			expectedAllow:  "POST",
			futureAuthNote: "POST stops a strategy instance",
		},
		{
			family: "strategy instance orders",
			path:   instanceDetailPrefix + "logging-alpha/orders",
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "strategy instance executions",
			path:   instanceDetailPrefix + "logging-alpha/executions",
			class:  controlPlaneRouteReadOnly,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
			},
			unsupported:   []string{http.MethodPost, http.MethodPut, http.MethodDelete},
			expectedAllow: "GET",
		},
		{
			family: "risk limits",
			path:   riskLimitsPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet: {status: http.StatusOK},
				http.MethodPut: {status: http.StatusBadRequest, body: `{}`},
			},
			unsupported:    []string{http.MethodPost, http.MethodDelete},
			expectedAllow:  "GET, PUT",
			futureAuthNote: "PUT updates runtime risk limits",
		},
		{
			family: "context backup",
			path:   contextBackupPath,
			class:  controlPlaneRouteMutable,
			allowed: map[string]controlPlaneRouteExpectation{
				http.MethodGet:  {status: http.StatusOK},
				http.MethodPost: {status: http.StatusOK, body: `{}`},
			},
			unsupported:    []string{http.MethodPut, http.MethodDelete},
			expectedAllow:  "GET, POST",
			futureAuthNote: "POST restores providers, instances, and risk state",
		},
	}
}

func TestControlPlaneRouteInventory(t *testing.T) {
	inventory := controlPlaneRouteInventory()
	if len(inventory) != 24 {
		t.Fatalf("expected 24 registered route families from NewHandler, got %d", len(inventory))
	}
	seen := make(map[string]struct{}, len(inventory))
	var mutableCount int
	var readOnlyCount int
	for _, route := range inventory {
		t.Run(route.family, func(t *testing.T) {
			if route.path == "" {
				t.Fatal("route path must be documented")
			}
			if _, ok := seen[route.family]; ok {
				t.Fatalf("duplicate route family %q", route.family)
			}
			seen[route.family] = struct{}{}
			if len(route.allowed) == 0 {
				t.Fatal("route must document at least one allowed method")
			}
			switch route.class {
			case controlPlaneRouteMutable:
				mutableCount++
				if route.futureAuthNote == "" {
					t.Fatal("mutable routes must document why future auth applies")
				}
				if !routeHasMutatingVerb(route) {
					t.Fatal("mutable route must include POST, PUT, or DELETE")
				}
			case controlPlaneRouteReadOnly:
				readOnlyCount++
				if route.futureAuthNote != "" {
					t.Fatalf("read-only route should not carry auth note %q", route.futureAuthNote)
				}
			default:
				t.Fatalf("unknown route class %q", route.class)
			}
		})
	}
	if mutableCount == 0 || readOnlyCount == 0 {
		t.Fatalf("expected both mutable and read-only routes, got mutable=%d readOnly=%d", mutableCount, readOnlyCount)
	}
}

func TestMutableRouteMatrix(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.class != controlPlaneRouteMutable {
			continue
		}
		for method, expected := range route.allowed {
			if !isMutatingMethod(method) {
				continue
			}
			route := route
			method := method
			expected := expected
			t.Run(route.family+" "+method, func(t *testing.T) {
				handler := newControlPlaneRouteInventoryHandler(t)
				res := serveControlPlaneInventoryRequest(handler, method, route.path, expected.body)
				if res.Code != expected.status {
					t.Fatalf("%s %s expected status %d, got %d (%s)", method, route.path, expected.status, res.Code, res.Body.String())
				}
			})
		}
	}
}

func TestMutableRouteMethodGuards(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.class != controlPlaneRouteMutable {
			continue
		}
		assertUnsupportedRouteMethods(t, route)
	}
}

func TestReadOnlyRouteMethodGuards(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.class != controlPlaneRouteReadOnly {
			continue
		}
		for method, expected := range route.allowed {
			if method != http.MethodGet {
				continue
			}
			route := route
			expected := expected
			t.Run(route.family+" GET", func(t *testing.T) {
				handler := newControlPlaneRouteInventoryHandler(t)
				res := serveControlPlaneInventoryRequest(handler, http.MethodGet, route.path, expected.body)
				if res.Code != expected.status {
					t.Fatalf("GET %s expected status %d, got %d (%s)", route.path, expected.status, res.Code, res.Body.String())
				}
			})
		}
		assertUnsupportedRouteMethods(t, route)
	}
}

func TestMutableRoutesRequireAuth(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.class != controlPlaneRouteMutable {
			continue
		}
		for method, expected := range route.allowed {
			if !isMutatingMethod(method) {
				continue
			}
			route := route
			method := method
			expected := expected
			t.Run(route.family+" "+method, func(t *testing.T) {
				handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")

				missing := serveControlPlaneInventoryRequest(handler, method, route.path, expected.body)
				assertAuthRequired(t, missing)

				invalid := serveControlPlaneInventoryRequestWithToken(handler, method, route.path, expected.body, "wrong-token")
				assertAuthRequired(t, invalid)

				valid := serveControlPlaneInventoryRequestWithToken(handler, method, route.path, expected.body, "control-token")
				if valid.Code != expected.status {
					t.Fatalf("authorized %s %s expected status %d, got %d (%s)", method, route.path, expected.status, valid.Code, valid.Body.String())
				}
			})
		}
	}
}

func TestProviderActionsRequireAuth(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.family != "provider start action" && route.family != "provider stop action" {
			continue
		}
		expected := route.allowed[http.MethodPost]
		route := route
		t.Run(route.family, func(t *testing.T) {
			handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
			missing := serveControlPlaneInventoryRequest(handler, http.MethodPost, route.path, expected.body)
			assertAuthRequired(t, missing)
			valid := serveControlPlaneInventoryRequestWithToken(handler, http.MethodPost, route.path, expected.body, "control-token")
			if valid.Code != expected.status {
				t.Fatalf("authorized provider action expected status %d, got %d (%s)", expected.status, valid.Code, valid.Body.String())
			}
		})
	}
}

func TestBackupRestoreRequiresAuth(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
	missing := serveControlPlaneInventoryRequest(handler, http.MethodPost, contextBackupPath, `{}`)
	assertAuthRequired(t, missing)
	valid := serveControlPlaneInventoryRequestWithToken(handler, http.MethodPost, contextBackupPath, `{}`, "control-token")
	if valid.Code != http.StatusOK {
		t.Fatalf("authorized backup restore expected status 200, got %d (%s)", valid.Code, valid.Body.String())
	}
}

func TestReadOnlyRoutesRemainOpen(t *testing.T) {
	for _, route := range controlPlaneRouteInventory() {
		if route.class != controlPlaneRouteReadOnly {
			continue
		}
		for method, expected := range route.allowed {
			route := route
			method := method
			expected := expected
			t.Run(route.family+" "+method, func(t *testing.T) {
				handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
				res := serveControlPlaneInventoryRequest(handler, method, route.path, expected.body)
				if res.Code != expected.status {
					t.Fatalf("unauthenticated read-only %s %s expected status %d, got %d (%s)", method, route.path, expected.status, res.Code, res.Body.String())
				}
			})
		}
	}
}

func TestStrategyCatalogReadOnlyUnaffected(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
	res := serveControlPlaneInventoryRequest(handler, http.MethodGet, strategiesPath, "")
	if res.Code != http.StatusOK {
		t.Fatalf("unauthenticated strategy catalog expected status 200, got %d (%s)", res.Code, res.Body.String())
	}
}

func assertAuthRequired(t *testing.T, res *httptest.ResponseRecorder) {
	t.Helper()
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d (%s)", res.Code, res.Body.String())
	}
	if challenge := res.Header().Get("WWW-Authenticate"); challenge != `Bearer realm="meltica-control-plane"` {
		t.Fatalf("expected bearer challenge, got %q", challenge)
	}
}

func assertUnsupportedRouteMethods(t *testing.T, route controlPlaneRouteInventoryEntry) {
	t.Helper()
	for _, method := range route.unsupported {
		t.Run(route.family+" rejects "+method, func(t *testing.T) {
			handler := newControlPlaneRouteInventoryHandler(t)
			res := serveControlPlaneInventoryRequest(handler, method, route.path, "")
			if res.Code != http.StatusMethodNotAllowed {
				t.Fatalf("%s %s expected status 405, got %d (%s)", method, route.path, res.Code, res.Body.String())
			}
			if allow := res.Header().Get("Allow"); allow != route.expectedAllow {
				t.Fatalf("%s %s expected Allow %q, got %q", method, route.path, route.expectedAllow, allow)
			}
		})
	}
}

func routeHasMutatingVerb(route controlPlaneRouteInventoryEntry) bool {
	for method := range route.allowed {
		if isMutatingMethod(method) {
			return true
		}
	}
	return false
}

func isMutatingMethod(method string) bool {
	return slices.Contains([]string{http.MethodPost, http.MethodPut, http.MethodDelete}, method)
}

func serveControlPlaneInventoryRequest(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	return serveControlPlaneInventoryRequestWithToken(handler, method, path, body, "")
}

func serveControlPlaneInventoryRequestWithToken(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	if body == "" && isMutatingMethod(method) {
		body = `{}`
	}
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func newControlPlaneRouteInventoryHandler(t *testing.T) http.Handler {
	t.Helper()
	return newControlPlaneRouteInventoryHandlerWithAuth(t, "")
}

func httpTestRiskConfig() config.RiskConfig {
	return config.RiskConfig{
		MaxPositionSize:     "10",
		MaxNotionalValue:    "1000",
		NotionalCurrency:    "USD",
		OrderThrottle:       5,
		OrderBurst:          1,
		MaxConcurrentOrders: 0,
		PriceBandPercent:    1,
	}
}

func newControlPlaneRouteInventoryHandlerWithAuth(t *testing.T, authToken string) http.Handler {
	t.Helper()
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		APIServer:  config.APIServerConfig{AuthToken: authToken},
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}
	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", 8, 8, func() any { return new(schema.Event) }); err != nil {
		t.Fatalf("register Event pool: %v", err)
	}
	if err := poolMgr.RegisterPool("OrderRequest", 4, 4, func() any { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register OrderRequest pool: %v", err)
	}
	bus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:    16,
		FanoutWorkers: 1,
		Pools:         poolMgr,
	})
	t.Cleanup(bus.Close)
	table := dispatcher.NewTable()
	registry := provider.NewRegistry()
	registry.Register("stub", func(ctx context.Context, pools *pool.PoolManager, cfg map[string]any) (provider.Instance, error) {
		name, _ := cfg["provider_name"].(string)
		if name == "" {
			name = "stub"
		}
		return &httpTestProviderInstance{name: name}, nil
	})
	providerManager := provider.NewManager(registry, poolMgr, bus, table, log.New(ioDiscards{}, "", 0))
	logger := log.New(ioDiscards{}, "", 0)
	registrar := dispatcher.NewRegistrar(table, providerManager)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providerManager, logger, registrar)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	for _, spec := range []config.ProviderSpec{
		{Name: "binance", Adapter: "stub", Config: map[string]any{"identifier": "stub", "provider_name": "binance"}},
		{Name: "running", Adapter: "stub", Config: map[string]any{"identifier": "stub", "provider_name": "running"}},
	} {
		if _, err := providerManager.Create(context.Background(), spec, false); err != nil {
			t.Fatalf("create provider %s: %v", spec.Name, err)
		}
	}
	for _, spec := range []config.LambdaSpec{
		controlPlaneRouteInventoryLambdaSpec("logging-alpha", "binance"),
		controlPlaneRouteInventoryLambdaSpec("running-alpha", "running"),
	} {
		if _, err := lambdaManager.Create(spec); err != nil {
			t.Fatalf("create lambda %s: %v", spec.ID, err)
		}
	}
	if _, err := providerManager.StartProviderAsync("running"); err != nil {
		t.Fatalf("start running provider: %v", err)
	}
	waitForProviderRunning(t, providerManager, "running")
	if err := lambdaManager.Start(context.Background(), "running-alpha"); err != nil {
		t.Fatalf("start running lambda: %v", err)
	}
	return NewHandler(appCfg, lambdaManager, providerManager, &stubOrderStore{})
}

func controlPlaneRouteInventoryLambdaSpec(id, providerName string) config.LambdaSpec {
	return config.LambdaSpec{
		ID:       id,
		Strategy: config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
		ProviderSymbols: map[string]config.ProviderSymbols{
			providerName: {Symbols: []string{"BTC-USDT"}},
		},
		Providers: []string{providerName},
	}
}

func waitForProviderRunning(t *testing.T, manager *provider.Manager, name string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		meta, ok := manager.ProviderMetadataFor(name)
		if ok && meta.Status == provider.StatusRunning && meta.Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("expected provider %s to transition to running", name)
}

func TestDecodeRiskConfig_NormalizesAllowedOrderTypes(t *testing.T) {
	payload := `{
		"maxPositionSize": "10",
		"maxNotionalValue": "100",
		"notionalCurrency": "USD",
		"orderThrottle": 5,
		"orderBurst": 1,
		"maxConcurrentOrders": 0,
		"priceBandPercent": 0,
		"allowedOrderTypes": [" limit", "LIMIT", "Market", "market ", "Stop "],
		"killSwitchEnabled": false,
		"maxRiskBreaches": 0,
		"circuitBreaker": {
			"enabled": false,
			"threshold": 0,
			"cooldown": ""
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/risk", strings.NewReader(payload))
	cfg, err := decodeRiskConfig(req)
	if err != nil {
		t.Fatalf("decodeRiskConfig: %v", err)
	}
	expected := []string{"limit", "Market", "Stop"}
	if !reflect.DeepEqual(cfg.AllowedOrderTypes, expected) {
		t.Fatalf("expected allowed order types %v, got %v", expected, cfg.AllowedOrderTypes)
	}
}

func TestBuildContextBackup(t *testing.T) {
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Environment: config.EnvDev,
		Eventbus: config.EventbusConfig{
			BufferSize: 16,
		},
		Pools: config.PoolConfig{
			Event: config.ObjectPoolConfig{
				Size:          8,
				WaitQueueSize: 8,
			},
			OrderRequest: config.ObjectPoolConfig{
				Size:          4,
				WaitQueueSize: 4,
			},
		},
		Risk: config.RiskConfig{
			MaxPositionSize:     "10",
			MaxNotionalValue:    "1000",
			NotionalCurrency:    "USD",
			OrderThrottle:       5,
			OrderBurst:          1,
			MaxConcurrentOrders: 0,
			PriceBandPercent:    1.0,
			AllowedOrderTypes:   []string{"Limit"},
			KillSwitchEnabled:   true,
			MaxRiskBreaches:     1,
			CircuitBreaker: config.CircuitBreakerConfig{
				Enabled:   true,
				Threshold: 1,
				Cooldown:  "30s",
			},
		},
		APIServer: config.APIServerConfig{
			Addr: ":0",
		},
		Telemetry: config.TelemetryConfig{
			OTLPEndpoint:  "http://localhost:4318",
			ServiceName:   "test-gateway",
			OTLPInsecure:  true,
			EnableMetrics: true,
		},
		Strategies: config.StrategiesConfig{Directory: strategyDir},
	}

	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", appCfg.Pools.Event.Size, appCfg.Pools.Event.QueueSize(), func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register Event pool: %v", err)
	}
	if err := poolMgr.RegisterPool("OrderRequest", appCfg.Pools.OrderRequest.Size, appCfg.Pools.OrderRequest.QueueSize(), func() interface{} { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register OrderRequest pool: %v", err)
	}

	bus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:    appCfg.Eventbus.BufferSize,
		FanoutWorkers: appCfg.Eventbus.FanoutWorkerCount(),
		Pools:         poolMgr,
	})

	table := dispatcher.NewTable()
	providerManager := provider.NewManager(nil, poolMgr, bus, table, log.New(ioDiscards{}, "", 0))

	// Register a provider spec with sensitive fields.
	providerSpec := config.ProviderSpec{
		Name:    "binance",
		Adapter: "binance",
		Config: map[string]any{
			"identifier": "binance",
			"config": map[string]any{
				"api_key":    "raw-backup-api-key",
				"api_secret": "raw-backup-api-secret",
				"depth":      100,
			},
		},
	}
	if _, err := providerManager.Create(context.Background(), providerSpec, false); err != nil {
		t.Fatalf("Create provider spec failed: %v", err)
	}

	logger := log.New(ioDiscards{}, "", 0)
	registrar := dispatcher.NewRegistrar(table, providerManager)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providerManager, logger, registrar)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	lambdaSpec := config.LambdaSpec{
		ID:        "alpha",
		Strategy:  config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
		Providers: []string{"binance"},
		ProviderSymbols: map[string]config.ProviderSymbols{
			"binance": {Symbols: []string{"BTC-USDT"}},
		},
	}
	if _, err := lambdaManager.Create(lambdaSpec); err != nil {
		t.Fatalf("Create lambda spec: %v", err)
	}

	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	snapshot := server.buildContextBackup()

	if len(snapshot.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(snapshot.Providers))
	}

	providerCfg, ok := snapshot.Providers[0].Config["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested config map, got %T", snapshot.Providers[0].Config["config"])
	}
	for _, key := range []string{"api_key", "api_secret"} {
		if _, present := providerCfg[key]; present {
			t.Fatalf("expected %s to be removed from exported provider config", key)
		}
	}
	backupJSON, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal backup snapshot: %v", err)
	}
	backupText := string(backupJSON)
	for _, fragment := range []string{"api_key", "api_secret", "raw-backup-api-key", "raw-backup-api-secret"} {
		if strings.Contains(backupText, fragment) {
			t.Fatalf("expected backup export not to contain %q, got %s", fragment, backupText)
		}
	}
	switch depth := providerCfg["depth"].(type) {
	case float64:
		if depth != 100 {
			t.Fatalf("expected depth 100, got %v", depth)
		}
	case int:
		if depth != 100 {
			t.Fatalf("expected depth 100, got %v", depth)
		}
	default:
		t.Fatalf("expected numeric depth, got %T", depth)
	}

	if len(snapshot.Lambdas) != 1 {
		t.Fatalf("expected 1 lambda snapshot, got %d", len(snapshot.Lambdas))
	}
	if snapshot.Lambdas[0].ID != "alpha" {
		t.Fatalf("expected lambda id alpha, got %s", snapshot.Lambdas[0].ID)
	}

	if snapshot.Risk.MaxPositionSize != appCfg.Risk.MaxPositionSize {
		t.Fatalf("expected risk maxPositionSize %s, got %s", appCfg.Risk.MaxPositionSize, snapshot.Risk.MaxPositionSize)
	}

	expectedNotional := decimal.RequireFromString(appCfg.Risk.MaxNotionalValue)
	actualNotional := decimal.RequireFromString(snapshot.Risk.MaxNotionalValue)
	if !expectedNotional.Equal(actualNotional) {
		t.Fatalf("expected maxNotionalValue %s, got %s", expectedNotional, actualNotional)
	}
}

func TestWriteStrategyModuleErrorReturnsDiagnostics(t *testing.T) {
	server := &httpServer{}
	recorder := httptest.NewRecorder()
	diag := js.Diagnostic{
		Stage:   js.DiagnosticStageValidation,
		Message: "displayName required",
		Line:    0,
		Column:  0,
		Hint:    "metadata.displayName",
	}
	diagErr := js.NewDiagnosticError("metadata validation failed", nil, diag)

	server.writeStrategyModuleError(recorder, diagErr)

	result := recorder.Result()
	t.Cleanup(func() { _ = result.Body.Close() })
	if result.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected status 422, got %d", result.StatusCode)
	}
	var payload map[string]any
	if err := json.NewDecoder(result.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["error"] != "strategy_validation_failed" {
		t.Fatalf("expected error strategy_validation_failed, got %v", payload["error"])
	}
	if payload["message"] != "metadata validation failed" {
		t.Fatalf("expected message preserved, got %v", payload["message"])
	}
	diagnostics, ok := payload["diagnostics"].([]any)
	if !ok || len(diagnostics) == 0 {
		t.Fatalf("expected diagnostics in payload")
	}
	first, ok := diagnostics[0].(map[string]any)
	if !ok {
		t.Fatalf("unexpected diagnostic payload type %T", diagnostics[0])
	}
	if first["stage"] != string(js.DiagnosticStageValidation) {
		t.Fatalf("expected validation stage, got %v", first["stage"])
	}
	if first["message"] != diag.Message {
		t.Fatalf("expected diagnostic message %q, got %v", diag.Message, first["message"])
	}
}

func TestApplyContextBackupRestoresState(t *testing.T) {
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}

	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", 8, 8, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register Event pool: %v", err)
	}
	if err := poolMgr.RegisterPool("OrderRequest", 4, 4, func() interface{} { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register OrderRequest pool: %v", err)
	}

	bus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:    16,
		FanoutWorkers: 1,
		Pools:         poolMgr,
	})

	table := dispatcher.NewTable()
	providerManager := provider.NewManager(nil, poolMgr, bus, table, log.New(ioDiscards{}, "", 0))
	logger := log.New(ioDiscards{}, "", 0)
	registrar := dispatcher.NewRegistrar(table, providerManager)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providerManager, logger, registrar)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	payload := contextBackup{
		Providers: []config.ProviderSpec{
			{
				Name:    "binance",
				Adapter: "binance",
				Config: map[string]any{
					"identifier": "binance",
					"config": map[string]any{
						"depth": 100,
					},
				},
			},
		},
		Lambdas: []config.LambdaSpec{
			{
				ID:       "alpha",
				Strategy: config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
				ProviderSymbols: map[string]config.ProviderSymbols{
					"binance": {
						Symbols: []string{"BTC-USDT"},
					},
				},
				Providers: []string{"binance"},
			},
		},
		Risk: config.RiskConfig{
			MaxPositionSize:  "20",
			MaxNotionalValue: "2000",
			NotionalCurrency: "USD",
			OrderThrottle:    10,
			OrderBurst:       2,
		},
	}

	if err := server.applyContextBackup(context.Background(), payload); err != nil {
		t.Fatalf("applyContextBackup failed: %v", err)
	}

	detail, ok := providerManager.ProviderMetadataFor("binance")
	if !ok {
		t.Fatal("expected provider binance to exist after restore")
	}
	if detail.Running {
		t.Fatal("expected provider to be stopped after restore")
	}

	snapshot, ok := lambdaManager.Instance("alpha")
	if !ok {
		t.Fatal("expected lambda alpha to exist after restore")
	}
	if snapshot.Running {
		t.Fatal("expected lambda alpha to be stopped after restore")
	}

	limits := lambdaManager.RiskLimits()
	if !limits.MaxPositionSize.Equal(decimal.RequireFromString("20")) {
		t.Fatalf("expected max position size 20, got %s", limits.MaxPositionSize.String())
	}
	if !limits.MaxNotionalValue.Equal(decimal.RequireFromString("2000")) {
		t.Fatalf("expected max notional value 2000, got %s", limits.MaxNotionalValue.String())
	}

	movePayload := contextBackup{
		Providers: []config.ProviderSpec{
			{
				Name:    "coinbase",
				Adapter: "coinbase",
				Config: map[string]any{
					"identifier": "coinbase",
					"config": map[string]any{
						"depth": 50,
					},
				},
			},
		},
		Lambdas: []config.LambdaSpec{
			{
				ID:       "alpha",
				Strategy: config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
				ProviderSymbols: map[string]config.ProviderSymbols{
					"coinbase": {
						Symbols: []string{"ETH-USD"},
					},
				},
			},
		},
		Risk: payload.Risk,
	}
	if err := server.applyContextBackup(context.Background(), movePayload); err != nil {
		t.Fatalf("applyContextBackup provider move failed: %v", err)
	}
	if _, ok := providerManager.ProviderMetadataFor("binance"); ok {
		t.Fatal("expected old provider binance to be removed after valid move")
	}
	if _, ok := providerManager.ProviderMetadataFor("coinbase"); !ok {
		t.Fatal("expected new provider coinbase to exist after valid move")
	}
	moved, ok := lambdaManager.Instance("alpha")
	if !ok {
		t.Fatal("expected lambda alpha to remain after valid provider move")
	}
	if !slices.Equal(moved.Providers, []string{"coinbase"}) {
		t.Fatalf("expected lambda alpha providers [coinbase], got %v", moved.Providers)
	}
}

func TestApplyContextBackupNoMutationOnFailure(t *testing.T) {
	t.Run("invalid lambda strategy", func(t *testing.T) {
		server, providerManager, lambdaManager := newContextBackupRestoreServer(t)
		for _, name := range []string{"old", "stale"} {
			createContextBackupProvider(t, providerManager, name)
		}
		for _, spec := range []config.LambdaSpec{
			controlPlaneRouteInventoryLambdaSpec("mover", "old"),
			controlPlaneRouteInventoryLambdaSpec("stale-lambda", "stale"),
		} {
			if _, err := lambdaManager.Create(spec); err != nil {
				t.Fatalf("create lambda %s: %v", spec.ID, err)
			}
		}

		payload := contextBackup{
			Providers: []config.ProviderSpec{contextBackupProviderSpec("target")},
			Lambdas: []config.LambdaSpec{
				{
					ID:       "mover",
					Strategy: config.LambdaStrategySpec{Identifier: "missing", Config: map[string]any{}},
					ProviderSymbols: map[string]config.ProviderSymbols{
						"target": {Symbols: []string{"BTC-USDT"}},
					},
				},
			},
			Risk: config.RiskConfig{
				MaxPositionSize:  "20",
				MaxNotionalValue: "2000",
				NotionalCurrency: "USD",
				OrderThrottle:    10,
				OrderBurst:       2,
			},
		}

		err := server.applyContextBackup(context.Background(), payload)
		if err == nil {
			t.Fatal("expected restore preflight to reject invalid lambda strategy")
		}
		if !strings.Contains(err.Error(), "lambda mover strategy \"missing\" not registered") {
			t.Fatalf("expected missing strategy error, got %v", err)
		}
		if _, ok := providerManager.ProviderMetadataFor("target"); ok {
			t.Fatal("did not expect target provider to be created after failed restore")
		}
		for _, name := range []string{"old", "stale"} {
			if _, ok := providerManager.ProviderMetadataFor(name); !ok {
				t.Fatalf("expected provider %s to remain after failed restore", name)
			}
		}
		if _, ok := lambdaManager.Instance("stale-lambda"); !ok {
			t.Fatal("expected stale lambda to remain after failed restore")
		}
		mover, ok := lambdaManager.Instance("mover")
		if !ok {
			t.Fatal("expected mover lambda to remain after failed restore")
		}
		if !slices.Equal(mover.Providers, []string{"old"}) {
			t.Fatalf("expected mover providers to remain [old], got %v", mover.Providers)
		}
		limits := lambdaManager.RiskLimits()
		if !limits.MaxPositionSize.Equal(decimal.RequireFromString("10")) {
			t.Fatalf("expected risk limits to remain unchanged, got %s", limits.MaxPositionSize)
		}
	})

	t.Run("provider update while starting", func(t *testing.T) {
		strategyDir := strategiestest.WriteStubStrategies(t)
		appCfg := config.AppConfig{
			Strategies: config.StrategiesConfig{Directory: strategyDir},
			Risk:       httpTestRiskConfig(),
		}
		registry := provider.NewRegistry()
		started := make(chan struct{}, 1)
		registry.Register("stub", func(ctx context.Context, pools *pool.PoolManager, cfg map[string]any) (provider.Instance, error) {
			select {
			case started <- struct{}{}:
			default:
			}
			<-ctx.Done()
			return nil, ctx.Err()
		})
		logger := log.New(ioDiscards{}, "", 0)
		providerManager := provider.NewManager(registry, nil, nil, dispatcher.NewTable(), logger)
		lifecycleCtx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		providerManager.SetLifecycleContext(lifecycleCtx)
		lambdaManager, err := lambdaruntime.NewManager(appCfg, nil, nil, providerManager, logger, nil)
		if err != nil {
			t.Fatalf("NewManager: %v", err)
		}
		server := &httpServer{
			manager:       lambdaManager,
			providers:     providerManager,
			orderStore:    nil,
			baseProviders: map[string]struct{}{},
		}
		for _, name := range []string{"blocked", "stale"} {
			createContextBackupProvider(t, providerManager, name)
		}
		if _, err := lambdaManager.Create(controlPlaneRouteInventoryLambdaSpec("stale-lambda", "stale")); err != nil {
			t.Fatalf("create stale lambda: %v", err)
		}
		if _, err := providerManager.StartProviderAsync("blocked"); err != nil {
			t.Fatalf("start blocked provider: %v", err)
		}
		waitForProviderFactory(t, started)

		payload := contextBackup{
			Providers: []config.ProviderSpec{contextBackupProviderSpec("blocked")},
			Risk: config.RiskConfig{
				MaxPositionSize:  "20",
				MaxNotionalValue: "2000",
				NotionalCurrency: "USD",
				OrderThrottle:    10,
				OrderBurst:       2,
			},
		}
		err = server.applyContextBackup(context.Background(), payload)
		if err == nil {
			t.Fatal("expected restore preflight to reject starting provider update")
		}
		if !strings.Contains(err.Error(), "provider blocked is starting") {
			t.Fatalf("expected starting provider error, got %v", err)
		}
		if _, ok := providerManager.ProviderMetadataFor("stale"); !ok {
			t.Fatal("expected stale provider to remain after failed restore")
		}
		if _, ok := lambdaManager.Instance("stale-lambda"); !ok {
			t.Fatal("expected stale lambda to remain after failed restore")
		}
		limits := lambdaManager.RiskLimits()
		if !limits.MaxPositionSize.Equal(decimal.RequireFromString("10")) {
			t.Fatalf("expected risk limits to remain unchanged, got %s", limits.MaxPositionSize)
		}
	})
}

func TestApplyContextBackupRejectsUnknownProvider(t *testing.T) {
	server, providerManager, lambdaManager := newContextBackupRestoreServer(t)
	createContextBackupProvider(t, providerManager, "existing")
	if _, err := lambdaManager.Create(controlPlaneRouteInventoryLambdaSpec("existing", "existing")); err != nil {
		t.Fatalf("create existing lambda: %v", err)
	}

	payload := contextBackup{
		Providers: []config.ProviderSpec{contextBackupProviderSpec("existing")},
		Lambdas: []config.LambdaSpec{
			{
				ID:       "bad",
				Strategy: config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
				ProviderSymbols: map[string]config.ProviderSymbols{
					"missing": {Symbols: []string{"BTC-USDT"}},
				},
			},
		},
	}

	err := server.applyContextBackup(context.Background(), payload)
	if err == nil {
		t.Fatal("expected restore preflight to reject unknown provider reference")
	}
	if !strings.Contains(err.Error(), "lambda bad references unknown provider missing") {
		t.Fatalf("expected unknown provider error, got %v", err)
	}
	if _, ok := lambdaManager.Instance("bad"); ok {
		t.Fatal("did not expect invalid lambda to be restored")
	}
	if _, ok := lambdaManager.Instance("existing"); !ok {
		t.Fatal("expected existing lambda to remain after failed restore")
	}
	if _, ok := providerManager.ProviderMetadataFor("existing"); !ok {
		t.Fatal("expected existing provider to remain after failed restore")
	}
}

func newContextBackupRestoreServer(t *testing.T) (*httpServer, *provider.Manager, *lambdaruntime.Manager) {
	t.Helper()
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}
	logger := log.New(ioDiscards{}, "", 0)
	providerManager := provider.NewManager(nil, nil, nil, dispatcher.NewTable(), logger)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, nil, nil, providerManager, logger, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}
	return server, providerManager, lambdaManager
}

func contextBackupProviderSpec(name string) config.ProviderSpec {
	return config.ProviderSpec{
		Name:    name,
		Adapter: "stub",
		Config: map[string]any{
			"identifier":    "stub",
			"provider_name": name,
		},
	}
}

func createContextBackupProvider(t *testing.T, manager *provider.Manager, name string) {
	t.Helper()
	if _, err := manager.Create(context.Background(), contextBackupProviderSpec(name), false); err != nil {
		t.Fatalf("create provider %s: %v", name, err)
	}
}

func TestBuildProviderSpecFromPayload_SanitizesEmptyConfig(t *testing.T) {
	payload := providerPayload{
		Name: "binance-ui-test",
		Adapter: providerAdapterPayload{
			Identifier: "binance",
			Config: map[string]any{
				"api_key":     "",
				"api_secret":  "   ",
				"recv_window": "5s",
				"list":        []any{" first ", " ", "second"},
				"nested": map[string]any{
					"alpha": "  ",
					"beta":  "value",
				},
			},
		},
	}

	spec, enabled, err := buildProviderSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("buildProviderSpecFromPayload returned error: %v", err)
	}
	if !enabled {
		t.Fatalf("expected provider to default to enabled")
	}
	if spec.Adapter != "binance" {
		t.Fatalf("expected adapter binance, got %s", spec.Adapter)
	}

	cfg, ok := spec.Config["config"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested config map, got %T", spec.Config["config"])
	}
	if _, exists := cfg["api_key"]; exists {
		t.Fatalf("expected empty api_key to be removed, found %v", cfg["api_key"])
	}
	if _, exists := cfg["api_secret"]; exists {
		t.Fatalf("expected empty api_secret to be removed, found %v", cfg["api_secret"])
	}
	if recvWindow, ok := cfg["recv_window"].(string); !ok || recvWindow != "5s" {
		t.Fatalf("expected recv_window to remain trimmed string, got %#v", cfg["recv_window"])
	}
	list, ok := cfg["list"].([]any)
	if !ok {
		t.Fatalf("expected list to be []any, got %T", cfg["list"])
	}
	if len(list) != 2 || list[0] != "first" || list[1] != "second" {
		t.Fatalf("expected cleaned list [first second], got %#v", list)
	}
	nested, ok := cfg["nested"].(map[string]any)
	if !ok {
		t.Fatalf("expected nested map, got %T", cfg["nested"])
	}
	if _, present := nested["alpha"]; present {
		t.Fatalf("expected empty nested value to be pruned, nested=%#v", nested)
	}
	if nested["beta"] != "value" {
		t.Fatalf("expected nested beta to be preserved, nested=%#v", nested)
	}
}

func TestBuildProviderSpecFromPayload_OmitsEmptyConfig(t *testing.T) {
	payload := providerPayload{
		Name: "binance-ui-test",
		Adapter: providerAdapterPayload{
			Identifier: "binance",
			Config: map[string]any{
				"api_key": "",
				"nested": map[string]any{
					"secret": " ",
				},
			},
		},
	}

	spec, _, err := buildProviderSpecFromPayload(payload)
	if err != nil {
		t.Fatalf("buildProviderSpecFromPayload returned error: %v", err)
	}
	if _, ok := spec.Config["config"]; ok {
		t.Fatalf("expected empty config map to be omitted, got %#v", spec.Config["config"])
	}
}

func TestHandleProviderDeleteBlockedWhenInUse(t *testing.T) {
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}

	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", 8, 8, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register Event pool: %v", err)
	}
	if err := poolMgr.RegisterPool("OrderRequest", 4, 4, func() interface{} { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register OrderRequest pool: %v", err)
	}

	bus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:    16,
		FanoutWorkers: 1,
		Pools:         poolMgr,
	})

	table := dispatcher.NewTable()
	providerManager := provider.NewManager(nil, poolMgr, bus, table, log.New(ioDiscards{}, "", 0))
	logger := log.New(ioDiscards{}, "", 0)
	registrar := dispatcher.NewRegistrar(table, providerManager)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providerManager, logger, registrar)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	providerSpec := config.ProviderSpec{
		Name:    "binance",
		Adapter: "binance",
		Config: map[string]any{
			"identifier": "binance",
		},
	}
	if _, err := providerManager.Create(context.Background(), providerSpec, false); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	lambdaSpec := config.LambdaSpec{
		ID:        "logging-alpha",
		Strategy:  config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
		Providers: []string{"binance"},
		ProviderSymbols: map[string]config.ProviderSymbols{
			"binance": {Symbols: []string{"BTC-USDT"}},
		},
	}
	if _, err := lambdaManager.Create(lambdaSpec); err != nil {
		t.Fatalf("create lambda: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/providers/binance", nil)
	res := httptest.NewRecorder()
	server.handleProviderResource(res, req, "binance")
	if res.Code != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d (%s)", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "logging-alpha") {
		t.Fatalf("expected dependent instance to be reported, body=%s", res.Body.String())
	}

	listReq := httptest.NewRequest(http.MethodGet, "/providers", nil)
	listRes := httptest.NewRecorder()
	server.listProviders(listRes, listReq)
	if listRes.Code != http.StatusOK {
		t.Fatalf("list providers unexpected status %d", listRes.Code)
	}
	var payload struct {
		Providers []struct {
			Name                   string   `json:"name"`
			DependentInstanceCount int      `json:"dependentInstanceCount"`
			DependentInstances     []string `json:"dependentInstances"`
		}
	}
	if err := json.Unmarshal(listRes.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode providers: %v", err)
	}
	if len(payload.Providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(payload.Providers))
	}
	if payload.Providers[0].DependentInstanceCount != 1 {
		t.Fatalf("expected dependent instance count 1, got %d", payload.Providers[0].DependentInstanceCount)
	}
	if len(payload.Providers[0].DependentInstances) != 1 || payload.Providers[0].DependentInstances[0] != "logging-alpha" {
		t.Fatalf("unexpected dependent instances %#v", payload.Providers[0].DependentInstances)
	}
}

func TestProviderUsageInfersProvidersFromScope(t *testing.T) {
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}

	poolMgr := pool.NewPoolManager()
	if err := poolMgr.RegisterPool("Event", 8, 8, func() interface{} { return new(schema.Event) }); err != nil {
		t.Fatalf("register Event pool: %v", err)
	}
	if err := poolMgr.RegisterPool("OrderRequest", 4, 4, func() interface{} { return new(schema.OrderRequest) }); err != nil {
		t.Fatalf("register OrderRequest pool: %v", err)
	}

	bus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:    16,
		FanoutWorkers: 1,
		Pools:         poolMgr,
	})
	t.Cleanup(bus.Close)

	table := dispatcher.NewTable()
	providerManager := provider.NewManager(nil, poolMgr, bus, table, log.New(ioDiscards{}, "", 0))
	logger := log.New(ioDiscards{}, "", 0)
	registrar := dispatcher.NewRegistrar(table, providerManager)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providerManager, logger, registrar)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if _, err := providerManager.Create(context.Background(), config.ProviderSpec{
		Name:    "binance",
		Adapter: "binance",
		Config: map[string]any{
			"identifier": "binance",
		},
	}, false); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	lambdaSpec := config.LambdaSpec{
		ID:       "logging-beta",
		Strategy: config.LambdaStrategySpec{Identifier: "logging", Config: map[string]any{}},
		ProviderSymbols: map[string]config.ProviderSymbols{
			"binance": {Symbols: []string{"BTC-USDT"}},
		},
	}
	if _, err := lambdaManager.Create(lambdaSpec); err != nil {
		t.Fatalf("create lambda: %v", err)
	}

	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	summaries := lambdaManager.Instances()
	var found bool
	for _, summary := range summaries {
		if summary.ID == "logging-beta" {
			found = true
			if !slices.Equal(summary.Providers, []string{"binance"}) {
				t.Fatalf("expected summary providers from scope [binance], got %v", summary.Providers)
			}
		}
	}
	if !found {
		t.Fatal("expected logging-beta summary")
	}

	usage := server.providerUsage()
	dependents, ok := usage["binance"]
	if !ok {
		t.Fatalf("expected binance dependencies, got %#v", usage)
	}
	if len(dependents) != 1 || dependents[0] != "logging-beta" {
		t.Fatalf("unexpected dependents for binance: %#v", dependents)
	}
}

func TestCreateProviderRespondsAcceptedPending(t *testing.T) {
	registry := provider.NewRegistry()
	started := make(chan struct{}, 1)
	registry.Register("stub", func(ctx context.Context, pools *pool.PoolManager, cfg map[string]any) (provider.Instance, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		name, _ := cfg["provider_name"].(string)
		if name == "" {
			name = "stub"
		}
		return &httpTestProviderInstance{name: name}, nil
	})

	logger := log.New(ioDiscards{}, "", 0)
	providerManager := provider.NewManager(registry, nil, nil, dispatcher.NewTable(), logger)

	server := &httpServer{
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	body := `{"name":"stub","adapter":{"identifier":"stub","config":{}},"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/providers", strings.NewReader(body))
	res := httptest.NewRecorder()

	server.createProvider(res, req)

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if location := res.Header().Get("Location"); location != "/providers/stub" {
		t.Fatalf("expected Location header /providers/stub, got %q", location)
	}

	var detail provider.RuntimeDetail
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.Name != "stub" {
		t.Fatalf("expected provider name stub, got %s", detail.Name)
	}
	if detail.Status != provider.StatusPending {
		t.Fatalf("expected pending status, got %s", detail.Status)
	}
	if detail.Running {
		t.Fatal("expected provider not running immediately after creation")
	}

	waitForProviderFactory(t, started)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		meta, ok := providerManager.ProviderMetadataFor("stub")
		if ok && meta.Status == provider.StatusRunning && meta.Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected provider to transition to running state")
}

func TestStartProviderActionReturnsAccepted(t *testing.T) {
	registry := provider.NewRegistry()
	started := make(chan struct{}, 1)
	registry.Register("stub", func(ctx context.Context, pools *pool.PoolManager, cfg map[string]any) (provider.Instance, error) {
		select {
		case started <- struct{}{}:
		default:
		}
		name, _ := cfg["provider_name"].(string)
		if name == "" {
			name = "stub"
		}
		return &httpTestProviderInstance{name: name}, nil
	})

	logger := log.New(ioDiscards{}, "", 0)
	providerManager := provider.NewManager(registry, nil, nil, dispatcher.NewTable(), logger)

	spec := config.ProviderSpec{
		Name:    "stub",
		Adapter: "stub",
		Config: map[string]any{
			"identifier":    "stub",
			"provider_name": "stub",
		},
	}
	if _, err := providerManager.Create(context.Background(), spec, false); err != nil {
		t.Fatalf("create provider: %v", err)
	}

	server := &httpServer{
		providers:     providerManager,
		orderStore:    nil,
		baseProviders: map[string]struct{}{},
	}

	req := httptest.NewRequest(http.MethodPost, "/providers/stub/start", nil)
	res := httptest.NewRecorder()

	server.handleProviderAction(res, req, "stub", "start")

	if res.Code != http.StatusAccepted {
		t.Fatalf("expected status 202, got %d", res.Code)
	}
	if location := res.Header().Get("Location"); location != "/providers/stub" {
		t.Fatalf("expected Location header /providers/stub, got %q", location)
	}

	var detail provider.RuntimeDetail
	if err := json.Unmarshal(res.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if detail.Status != provider.StatusStarting {
		t.Fatalf("expected starting status, got %s", detail.Status)
	}
	if detail.Running {
		t.Fatal("expected provider not running during startup")
	}

	waitForProviderFactory(t, started)

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		meta, ok := providerManager.ProviderMetadataFor("stub")
		if ok && meta.Status == provider.StatusRunning && meta.Running {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected provider to transition to running state")
}

func TestCreateInstance(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")

	missingAuth := serveJSONRequest(handler, http.MethodPost, instancesPath, createInstancePayload("route-created", "binance"), "")
	assertAuthRequired(t, missingAuth)

	invalid := serveJSONRequest(handler, http.MethodPost, instancesPath, `{}`, "control-token")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid create payload status 400, got %d (%s)", invalid.Code, invalid.Body.String())
	}

	created := serveJSONRequest(handler, http.MethodPost, instancesPath, createInstancePayload("route-created", "binance"), "control-token")
	if created.Code != http.StatusCreated {
		t.Fatalf("expected create status 201, got %d (%s)", created.Code, created.Body.String())
	}
	var payload instanceSnapshotResponse
	if err := json.Unmarshal(created.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if payload.ID != "route-created" || !slices.Equal(payload.Providers, []string{"binance"}) {
		t.Fatalf("unexpected created instance payload: %#v", payload)
	}
	if payload.Links.Self != instanceDetailPrefix+"route-created" || payload.Links.Usage == "" {
		t.Fatalf("expected instance links in create response, got %#v", payload.Links)
	}
}

func TestUpdateInstance(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
	path := instanceDetailPrefix + "logging-alpha"

	missingAuth := serveJSONRequest(handler, http.MethodPut, path, createInstancePayload("logging-alpha", "binance"), "")
	assertAuthRequired(t, missingAuth)

	mismatch := serveJSONRequest(handler, http.MethodPut, path, createInstancePayload("different-id", "binance"), "control-token")
	if mismatch.Code != http.StatusBadRequest {
		t.Fatalf("expected id mismatch status 400, got %d (%s)", mismatch.Code, mismatch.Body.String())
	}

	updated := serveJSONRequest(handler, http.MethodPut, path, updateInstancePayload("logging-alpha", "binance"), "control-token")
	if updated.Code != http.StatusOK {
		t.Fatalf("expected update status 200, got %d (%s)", updated.Code, updated.Body.String())
	}
	var payload instanceSnapshotResponse
	if err := json.Unmarshal(updated.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if payload.ID != "logging-alpha" || payload.Strategy.Config["logger_prefix"] != "[updated]" {
		t.Fatalf("unexpected updated instance payload: %#v", payload)
	}
}

func TestDeleteInstance(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
	path := instanceDetailPrefix + "logging-alpha"

	missingAuth := serveJSONRequest(handler, http.MethodDelete, path, "", "")
	assertAuthRequired(t, missingAuth)

	deleted := serveJSONRequest(handler, http.MethodDelete, path, "", "control-token")
	if deleted.Code != http.StatusOK {
		t.Fatalf("expected delete status 200, got %d (%s)", deleted.Code, deleted.Body.String())
	}
	var payload map[string]string
	if err := json.Unmarshal(deleted.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode delete response: %v", err)
	}
	if payload["status"] != "removed" || payload["id"] != "logging-alpha" {
		t.Fatalf("unexpected delete response: %#v", payload)
	}

	missing := serveJSONRequest(handler, http.MethodGet, path, "", "")
	if missing.Code != http.StatusNotFound {
		t.Fatalf("expected deleted instance GET status 404, got %d (%s)", missing.Code, missing.Body.String())
	}
}

func TestInstanceExecutionsEndpoint(t *testing.T) {
	store := &stubOrderStore{
		executions: []orderstore.ExecutionRecord{
			{
				Execution: orderstore.Execution{
					OrderID:     "ord-1",
					Provider:    "binance",
					ExecutionID: "exec-1",
					Quantity:    "0.5",
					Price:       "21000",
					Liquidity:   "maker",
					TradedAt:    1_700_000_100,
					Metadata:    map[string]any{"note": "fill"},
				},
				StrategyInstance: "demo",
				CreatedAt:        1_700_000_101,
			},
		},
	}
	handler := NewHandler(config.AppConfig{}, nil, nil, store)

	res := serveJSONRequest(handler, http.MethodGet, "/strategy/instances/demo/executions?provider=binance&orderId=ord-1&limit=9999", "", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}
	if store.lastExecutionQuery.StrategyInstance != "demo" || store.lastExecutionQuery.Provider != "binance" || store.lastExecutionQuery.OrderID != "ord-1" || store.lastExecutionQuery.Limit != maxListLimit {
		t.Fatalf("unexpected execution query: %#v", store.lastExecutionQuery)
	}
	var payload struct {
		Executions []orderstore.ExecutionRecord `json:"executions"`
		Count      int                          `json:"count"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 1 || len(payload.Executions) != 1 || payload.Executions[0].ExecutionID != "exec-1" {
		t.Fatalf("unexpected executions payload: %#v", payload)
	}

	invalid := serveJSONRequest(handler, http.MethodGet, "/strategy/instances/demo/executions?limit=bogus", "", "")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid limit status 400, got %d (%s)", invalid.Code, invalid.Body.String())
	}
}

func TestRefreshStrategies(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")

	missingAuth := serveJSONRequest(handler, http.MethodPost, strategyRefreshPath, `{}`, "")
	assertAuthRequired(t, missingAuth)

	invalid := serveJSONRequest(handler, http.MethodPost, strategyRefreshPath, `{"unknown":true}`, "control-token")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid refresh payload status 400, got %d (%s)", invalid.Code, invalid.Body.String())
	}

	refreshed := serveJSONRequest(handler, http.MethodPost, strategyRefreshPath, `{}`, "control-token")
	if refreshed.Code != http.StatusOK {
		t.Fatalf("expected refresh status 200, got %d (%s)", refreshed.Code, refreshed.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(refreshed.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode refresh response: %v", err)
	}
	if payload["status"] != "refreshed" {
		t.Fatalf("expected refreshed status, got %#v", payload)
	}
}

func TestUpdateRiskLimits(t *testing.T) {
	handler := newControlPlaneRouteInventoryHandlerWithAuth(t, "control-token")
	validRisk := `{"maxPositionSize":"25","maxNotionalValue":"2500","notionalCurrency":"USD","orderThrottle":7,"orderBurst":2,"maxConcurrentOrders":3,"priceBandPercent":1.5,"allowedOrderTypes":[" limit ","LIMIT","Market"],"killSwitchEnabled":true,"maxRiskBreaches":2,"circuitBreaker":{"enabled":true,"threshold":2,"cooldown":"45s"}}`

	missingAuth := serveJSONRequest(handler, http.MethodPut, riskLimitsPath, validRisk, "")
	assertAuthRequired(t, missingAuth)

	invalid := serveJSONRequest(handler, http.MethodPut, riskLimitsPath, strings.Replace(validRisk, `"maxPositionSize":"25"`, `"maxPositionSize":"twenty-five"`, 1), "control-token")
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid risk status 400, got %d (%s)", invalid.Code, invalid.Body.String())
	}
	if !strings.Contains(invalid.Body.String(), "maxPositionSize") {
		t.Fatalf("expected semantic validation error to mention maxPositionSize, got %s", invalid.Body.String())
	}

	updated := serveJSONRequest(handler, http.MethodPut, riskLimitsPath, validRisk, "control-token")
	if updated.Code != http.StatusOK {
		t.Fatalf("expected risk update status 200, got %d (%s)", updated.Code, updated.Body.String())
	}
	var payload struct {
		Status string            `json:"status"`
		Limits config.RiskConfig `json:"limits"`
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode risk response: %v", err)
	}
	if payload.Status != "updated" || payload.Limits.MaxPositionSize != "25" || payload.Limits.CircuitBreaker.Cooldown != "45s" {
		t.Fatalf("unexpected risk update response: %#v", payload)
	}
	if !slices.Equal(payload.Limits.AllowedOrderTypes, []string{"limit", "Market"}) {
		t.Fatalf("expected normalized order types [limit Market], got %v", payload.Limits.AllowedOrderTypes)
	}
}

func serveJSONRequest(handler http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	reader := strings.NewReader(body)
	if body == "" {
		reader = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func createInstancePayload(id, providerName string) string {
	return `{"id":"` + id + `","strategy":{"identifier":"logging","config":{}},"scope":{"` + providerName + `":{"symbols":["BTC-USDT"]}}}`
}

func updateInstancePayload(id, providerName string) string {
	return `{"id":"` + id + `","strategy":{"identifier":"logging","config":{"logger_prefix":"[updated]"}},"scope":{"` + providerName + `":{"symbols":["BTC-USDT"]}}}`
}

func TestInstanceOrdersEndpointReturnsRecords(t *testing.T) {
	store := &stubOrderStore{
		orders: []orderstore.OrderRecord{
			{
				Order: orderstore.Order{
					ID:               "ord-1",
					Provider:         "binance",
					StrategyInstance: "demo",
					ClientOrderID:    "ord-1",
					Symbol:           "BTC-USDT",
					Side:             "BUY",
					Type:             "LIMIT",
					Quantity:         "1.000",
					Price:            strPtr("21000"),
					State:            "ACK",
					PlacedAt:         1_700_000_000,
					Metadata:         map[string]any{"note": "test"},
				},
				AcknowledgedAt: strInt64Ptr(1_700_000_001),
				CreatedAt:      1_700_000_000,
				UpdatedAt:      1_700_000_010,
			},
		},
	}
	handler := NewHandler(config.AppConfig{}, nil, nil, store)

	req := httptest.NewRequest(http.MethodGet, "/strategy/instances/demo/orders?limit=9999", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload struct {
		Orders []orderstore.OrderRecord `json:"orders"`
		Count  int                      `json:"count"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 1 {
		t.Fatalf("expected count 1, got %d", payload.Count)
	}
	if len(payload.Orders) != 1 || payload.Orders[0].Order.ID != "ord-1" {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestInstanceOrdersEndpointInvalidLimit(t *testing.T) {
	handler := NewHandler(config.AppConfig{}, nil, nil, &stubOrderStore{})
	req := httptest.NewRequest(http.MethodGet, "/strategy/instances/demo/orders?limit=bogus", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", res.Code)
	}
}

func TestProviderBalancesEndpointReturnsRecords(t *testing.T) {
	store := &stubOrderStore{
		balances: []orderstore.BalanceRecord{
			{
				BalanceSnapshot: orderstore.BalanceSnapshot{
					Provider:   "binance",
					Asset:      "USDT",
					Total:      "1000",
					Available:  "500",
					SnapshotAt: 1_700_000_000,
					Metadata:   map[string]any{"note": "snapshot"},
				},
				CreatedAt: 1_700_000_000,
				UpdatedAt: 1_700_000_100,
			},
		},
	}
	handler := NewHandler(config.AppConfig{}, nil, nil, store)

	req := httptest.NewRequest(http.MethodGet, "/providers/binance/balances", nil)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", res.Code, res.Body.String())
	}

	var payload struct {
		Balances []orderstore.BalanceRecord `json:"balances"`
		Count    int                        `json:"count"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Count != 1 || len(payload.Balances) != 1 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
	if payload.Balances[0].BalanceSnapshot.Provider != "binance" {
		t.Fatalf("unexpected provider in payload: %#v", payload.Balances[0])
	}
}

type stubOrderStore struct {
	orders             []orderstore.OrderRecord
	executions         []orderstore.ExecutionRecord
	balances           []orderstore.BalanceRecord
	lastExecutionQuery orderstore.ExecutionQuery
}

func (s *stubOrderStore) CreateOrder(context.Context, orderstore.Order) error             { return nil }
func (s *stubOrderStore) UpdateOrder(context.Context, orderstore.OrderUpdate) error       { return nil }
func (s *stubOrderStore) RecordExecution(context.Context, orderstore.Execution) error     { return nil }
func (s *stubOrderStore) UpsertBalance(context.Context, orderstore.BalanceSnapshot) error { return nil }
func (s *stubOrderStore) WithTransaction(ctx context.Context, fn func(context.Context, orderstore.Tx) error) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, s)
}
func (s *stubOrderStore) ListOrders(context.Context, orderstore.OrderQuery) ([]orderstore.OrderRecord, error) {
	return s.orders, nil
}
func (s *stubOrderStore) ListExecutions(_ context.Context, query orderstore.ExecutionQuery) ([]orderstore.ExecutionRecord, error) {
	s.lastExecutionQuery = query
	return s.executions, nil
}
func (s *stubOrderStore) ListBalances(context.Context, orderstore.BalanceQuery) ([]orderstore.BalanceRecord, error) {
	return s.balances, nil
}

func strPtr(value string) *string {
	return &value
}

func strInt64Ptr(value int64) *int64 {
	return &value
}

type ioDiscards struct{}

func (ioDiscards) Write(p []byte) (int, error) {
	return len(p), nil
}

func waitForProviderFactory(t *testing.T, started <-chan struct{}) {
	t.Helper()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for provider factory")
	}
}

type httpTestProviderInstance struct {
	name string
}

func (i *httpTestProviderInstance) Name() string                    { return i.name }
func (i *httpTestProviderInstance) Start(ctx context.Context) error { return nil }
func (i *httpTestProviderInstance) Events() <-chan *schema.Event    { return nil }
func (i *httpTestProviderInstance) Errors() <-chan error            { return nil }
func (i *httpTestProviderInstance) SubmitOrder(ctx context.Context, req schema.OrderRequest) error {
	return nil
}
func (i *httpTestProviderInstance) SubscribeRoute(route dispatcher.Route) error   { return nil }
func (i *httpTestProviderInstance) UnsubscribeRoute(route dispatcher.Route) error { return nil }
func (i *httpTestProviderInstance) Instruments() []schema.Instrument              { return nil }

func TestFilterModuleSummaries(t *testing.T) {
	modules := []js.ModuleSummary{
		{
			Name:      "alpha",
			Revisions: []js.ModuleRevision{{Hash: "sha256:a"}},
		},
		{
			Name:      "beta",
			Revisions: []js.ModuleRevision{{Hash: "sha256:b"}},
		},
	}

	values := url.Values{}
	values.Set("hash", "sha256:b")
	filtered, total, _, _, err := filterModuleSummaries(modules, values)
	if err != nil {
		t.Fatalf("hash filter: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("expected single module after hash filter, got total=%d filtered=%d", total, len(filtered))
	}
	if len(filtered[0].Revisions) != 1 || filtered[0].Revisions[0].Hash != "sha256:b" {
		t.Fatalf("expected revision filtered to sha256:b, got %+v", filtered[0].Revisions)
	}

	values = url.Values{}
	var offset, limit int
	values.Set("strategy", "alpha")
	values.Set("limit", "1")
	values.Set("offset", "0")
	filtered, total, offset, limit, err = filterModuleSummaries(modules, values)
	if err != nil {
		t.Fatalf("pagination filter: %v", err)
	}
	if total != 1 || len(filtered) != 1 {
		t.Fatalf("expected single module for strategy alpha, got total=%d filtered=%d", total, len(filtered))
	}
	if offset != 0 || limit != 1 {
		t.Fatalf("expected offset=0 limit=1, got offset=%d limit=%d", offset, limit)
	}

	values = url.Values{}
	values.Set("limit", "-1")
	if _, _, _, _, err := filterModuleSummaries(modules, values); err == nil {
		t.Fatalf("expected error for negative limit")
	}
}

func TestListStrategyModulesRejectsRunningOnly(t *testing.T) {
	server := &httpServer{}
	req := httptest.NewRequest(http.MethodGet, "/strategies/modules?runningOnly=true", nil)
	rec := httptest.NewRecorder()
	server.listStrategyModules(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for runningOnly, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestBuildUsageSelector(t *testing.T) {
	if sel := buildUsageSelector("noop@hash", "noop", "hash"); sel != "noop@hash" {
		t.Fatalf("expected selector passthrough, got %s", sel)
	}
	if sel := buildUsageSelector("", "Logging", "sha256:abc"); sel != "logging@sha256:abc" {
		t.Fatalf("expected auto selector, got %s", sel)
	}
	if sel := buildUsageSelector("", "", "sha256:abc"); sel != "" {
		t.Fatalf("expected empty selector when identifier missing, got %s", sel)
	}
	expected := strategyModulePrefix + url.PathEscape("logging@sha256:abc") + strategyUsageSuffix
	if url := buildModuleUsageURL("logging@sha256:abc"); url != expected {
		t.Fatalf("unexpected usage URL %s", url)
	}
}

func TestHandleStrategyModuleTagRoutes(t *testing.T) {
	server := &httpServer{}
	req := httptest.NewRequest(http.MethodDelete, "/strategies/modules/logging/tags/v1.0.1", nil)
	rec := httptest.NewRecorder()
	server.handleStrategyModule(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 due to nil manager, got %d (%s)", rec.Code, rec.Body.String())
	}
}

func TestDecodeRiskConfigRejectsInvalidSemanticValues(t *testing.T) {
	basePayload := `{
		"maxPositionSize": "10",
		"maxNotionalValue": "100",
		"notionalCurrency": "USD",
		"orderThrottle": 5,
		"orderBurst": 1,
		"maxConcurrentOrders": 0,
		"priceBandPercent": 0,
		"allowedOrderTypes": ["Limit"],
		"killSwitchEnabled": false,
		"maxRiskBreaches": 0,
		"circuitBreaker": {
			"enabled": true,
			"threshold": 1,
			"cooldown": "30s"
		}
	}`
	tests := []struct {
		name    string
		payload string
		wantErr string
	}{
		{
			name:    "invalid decimal",
			payload: strings.Replace(basePayload, `"maxPositionSize": "10"`, `"maxPositionSize": "ten"`, 1),
			wantErr: "maxPositionSize",
		},
		{
			name:    "invalid duration",
			payload: strings.Replace(basePayload, `"cooldown": "30s"`, `"cooldown": "later"`, 1),
			wantErr: "circuitBreaker.cooldown",
		},
		{
			name:    "zero order burst",
			payload: strings.Replace(basePayload, `"orderBurst": 1`, `"orderBurst": 0`, 1),
			wantErr: "orderBurst",
		},
		{
			name:    "negative max risk breaches",
			payload: strings.Replace(basePayload, `"maxRiskBreaches": 0`, `"maxRiskBreaches": -1`, 1),
			wantErr: "maxRiskBreaches",
		},
		{
			name:    "negative circuit breaker threshold",
			payload: strings.Replace(basePayload, `"threshold": 1`, `"threshold": -1`, 1),
			wantErr: "circuitBreaker.threshold",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, riskLimitsPath, strings.NewReader(tt.payload))
			_, err := decodeRiskConfig(req)
			if err == nil {
				t.Fatal("expected decodeRiskConfig to reject invalid risk config")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestApplyContextBackupRejectsInvalidRisk(t *testing.T) {
	strategyDir := strategiestest.WriteStubStrategies(t)
	appCfg := config.AppConfig{
		Strategies: config.StrategiesConfig{Directory: strategyDir},
		Risk:       httpTestRiskConfig(),
	}
	logger := log.New(ioDiscards{}, "", 0)
	providerManager := provider.NewManager(nil, nil, nil, dispatcher.NewTable(), logger)
	lambdaManager, err := lambdaruntime.NewManager(appCfg, nil, nil, providerManager, logger, nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	server := &httpServer{
		manager:       lambdaManager,
		providers:     providerManager,
		baseProviders: map[string]struct{}{},
	}
	payload := contextBackup{Risk: httpTestRiskConfig()}
	payload.Risk.MaxPositionSize = "ten"

	err = server.applyContextBackup(context.Background(), payload)
	if err == nil {
		t.Fatal("expected context backup restore to reject invalid risk config")
	}
	if !strings.Contains(err.Error(), "maxPositionSize") {
		t.Fatalf("expected maxPositionSize error, got %v", err)
	}
	limits := lambdaManager.RiskLimits()
	if !limits.MaxPositionSize.Equal(decimal.RequireFromString("10")) {
		t.Fatalf("expected existing risk limits to remain unchanged, got %s", limits.MaxPositionSize)
	}
}
