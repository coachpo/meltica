// Package httpserver exposes HTTP handlers for managing lambda strategies and risk settings.
package httpserver

import (
	"net/http"
	"strings"

	"github.com/coachpo/meltica/internal/app/lambda/runtime"
	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/infra/config"
)

const (
	maxJSONBodyBytes int64 = 1 << 20 // 1 MiB

	strategiesPath       = "/strategies"
	strategyDetailPrefix = strategiesPath + "/"
	strategyModulesPath  = strategiesPath + "/modules"
	strategyModulePrefix = strategyModulesPath + "/"
	strategyRefreshPath  = strategiesPath + "/refresh"
	strategyRegistryPath = strategiesPath + "/registry"
	strategySourceSuffix = "/source"
	strategyUsageSuffix  = "/usage"

	providersPath        = "/providers"
	providerDetailPrefix = providersPath + "/"

	adaptersPath        = "/adapters"
	adapterDetailPrefix = adaptersPath + "/"

	instancesPath        = "/strategy/instances"
	instanceDetailPrefix = instancesPath + "/"

	riskLimitsPath    = "/risk/limits"
	contextBackupPath = "/context/backup"

	instanceOrdersSuffix     = "orders"
	instanceExecutionsSuffix = "executions"
	providerBalancesSuffix   = "balances"

	defaultOrdersLimit     = 50
	defaultExecutionsLimit = 100
	defaultBalancesLimit   = 100
	maxListLimit           = 500
)

type handlerFunc func(http.ResponseWriter, *http.Request)

type httpServer struct {
	manager       *runtime.Manager
	providers     *provider.Manager
	orderStore    orderstore.Store
	baseProviders map[string]struct{}
}

type providerPayload struct {
	Name    string                 `json:"name"`
	Adapter providerAdapterPayload `json:"adapter"`
	Enabled *bool                  `json:"enabled,omitempty"`
}

type providerAdapterPayload struct {
	Identifier string         `json:"identifier"`
	Config     map[string]any `json:"config"`
}

type contextBackup struct {
	Providers []config.ProviderSpec `json:"providers,omitempty"`
	Lambdas   []config.LambdaSpec   `json:"lambdas,omitempty"`
	Risk      config.RiskConfig     `json:"risk"`
}

type strategyModulePayload struct {
	Source string `json:"source"`
}

type strategyTagPayload struct {
	Hash    string `json:"hash"`
	Refresh *bool  `json:"refresh,omitempty"`
}

type strategyRefreshPayload struct {
	Hashes     []string `json:"hashes"`
	Strategies []string `json:"strategies"`
}

type instanceLinks struct {
	Self  string `json:"self,omitempty"`
	Usage string `json:"usage,omitempty"`
}

type instanceSummaryResponse struct {
	runtime.InstanceSummary
	Links instanceLinks `json:"links"`
}

type instanceSnapshotResponse struct {
	runtime.InstanceSnapshot
	Links instanceLinks `json:"links"`
}

// NewHandler creates an HTTP handler for lambda management operations.
func NewHandler(appCfg config.AppConfig, manager *runtime.Manager, providers *provider.Manager, orders orderstore.Store) http.Handler {
	baseProviders := make(map[string]struct{}, len(appCfg.Providers))
	for name := range appCfg.Providers {
		normalized := strings.ToLower(strings.TrimSpace(string(name)))
		if normalized != "" {
			baseProviders[normalized] = struct{}{}
		}
	}
	server := &httpServer{
		manager:       manager,
		providers:     providers,
		orderStore:    orders,
		baseProviders: baseProviders,
	}
	mux := http.NewServeMux()

	mux.Handle(strategiesPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.getStrategies,
	}))
	mux.Handle(strategyDetailPrefix, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.getStrategy,
	}))
	mux.Handle(strategyModulesPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet:  server.listStrategyModules,
		http.MethodPost: server.createStrategyModule,
	}))
	mux.Handle(strategyModulePrefix, http.HandlerFunc(server.handleStrategyModule))
	mux.Handle(strategyRefreshPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodPost: server.refreshStrategies,
	}))
	mux.Handle(strategyRegistryPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.exportStrategyRegistry,
	}))

	mux.Handle(providersPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet:  server.listProviders,
		http.MethodPost: server.createProvider,
	}))
	mux.Handle(providerDetailPrefix, http.HandlerFunc(server.handleProvider))

	mux.Handle(adaptersPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.listAdapters,
	}))
	mux.Handle(adapterDetailPrefix, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.getAdapter,
	}))

	mux.Handle(instancesPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet:  server.listInstances,
		http.MethodPost: server.createInstance,
	}))
	mux.Handle(instanceDetailPrefix, http.HandlerFunc(server.handleInstance))

	mux.Handle(riskLimitsPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet: server.getRiskLimits,
		http.MethodPut: server.updateRiskLimits,
	}))

	mux.Handle(contextBackupPath, server.methodHandlers(map[string]handlerFunc{
		http.MethodGet:  server.handleContextBackupExport,
		http.MethodPost: server.handleContextBackupRestore,
	}))

	return withCORS(withMutableRouteAuth(mux, appCfg.APIServer.AuthToken))
}
