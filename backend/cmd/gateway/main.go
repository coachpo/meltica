// Command gateway launches the Meltica runtime entrypoint.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/coachpo/meltica/internal/app/dispatcher"
	lambdaruntime "github.com/coachpo/meltica/internal/app/lambda/runtime"
	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/app/risk"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/domain/outboxstore"
	"github.com/coachpo/meltica/internal/domain/providerstore"
	"github.com/coachpo/meltica/internal/domain/schema"
	"github.com/coachpo/meltica/internal/domain/strategystore"
	"github.com/coachpo/meltica/internal/infra/adapters"
	"github.com/coachpo/meltica/internal/infra/bus/eventbus"
	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/coachpo/meltica/internal/infra/persistence/migrations"
	postgresstore "github.com/coachpo/meltica/internal/infra/persistence/postgres"
	"github.com/coachpo/meltica/internal/infra/pool"
	httpserver "github.com/coachpo/meltica/internal/infra/server/http"
	"github.com/coachpo/meltica/internal/infra/telemetry"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sourcegraph/conc"
)

const (
	defaultConfigPath            = "config/app.yaml"
	configPathEnvVar             = "MELTICA_CONFIG_PATH"
	gatewayLoggerPrefix          = "gateway "
	eventPoolName                = "Event"
	orderRequestPoolName         = "OrderRequest"
	shutdownTimeout              = 30 * time.Second
	controlServerShutdownTimeout = 5 * time.Second
	lifecycleShutdownTimeout     = 10 * time.Second
	dataBusShutdownTimeout       = 2 * time.Second
	poolManagerShutdownTimeout   = 5 * time.Second
	telemetryShutdownTimeout     = 5 * time.Second
	controlReadHeaderTimeout     = 5 * time.Second
	databaseConnectTimeout       = 15 * time.Second
	databaseShutdownTimeout      = 5 * time.Second
)

type gatewayRuntime struct {
	composeGateway          composeGatewayFunc
	startAPIServer          startAPIServerFunc
	performGracefulShutdown gracefulShutdownFunc
	newShutdownContext      shutdownContextFunc
}

type composeGatewayFunc func(context.Context, *log.Logger, string, context.CancelFunc) (gatewayComposition, error)
type startAPIServerFunc func(*conc.WaitGroup, *log.Logger, apiServerStarter)
type gracefulShutdownFunc func(context.Context, *log.Logger, gracefulShutdownConfig)
type shutdownContextFunc func() (context.Context, context.CancelFunc)

type gatewayComposition struct {
	apiServer *http.Server
	lifecycle *conc.WaitGroup
	shutdown  gracefulShutdownConfig
}

func defaultGatewayRuntime() gatewayRuntime {
	return gatewayRuntime{
		composeGateway:          composeGateway,
		startAPIServer:          startAPIServer,
		performGracefulShutdown: performGracefulShutdown,
		newShutdownContext:      newGatewayShutdownContext,
	}
}

func (rt gatewayRuntime) withDefaults() gatewayRuntime {
	if rt.composeGateway == nil {
		rt.composeGateway = composeGateway
	}
	if rt.startAPIServer == nil {
		rt.startAPIServer = startAPIServer
	}
	if rt.performGracefulShutdown == nil {
		rt.performGracefulShutdown = performGracefulShutdown
	}
	if rt.newShutdownContext == nil {
		rt.newShutdownContext = newGatewayShutdownContext
	}
	return rt
}

func main() {
	cfgPathFlag := parseFlags()
	ctx, cancel := newSignalContext()
	defer cancel()

	logger := newGatewayLogger()
	if err := runGateway(ctx, cancel, logger, resolveConfigPath(cfgPathFlag), defaultGatewayRuntime()); err != nil {
		logger.Fatal(err)
	}
}

func runGateway(ctx context.Context, cancel context.CancelFunc, logger *log.Logger, cfgPath string, runtime gatewayRuntime) error {
	if cancel == nil {
		cancel = func() {}
	}
	runtime = runtime.withDefaults()

	composition, err := runtime.composeGateway(ctx, logger, cfgPath, cancel)
	if err != nil {
		return err
	}

	runtime.startAPIServer(composition.lifecycle, logger, composition.apiServer)
	logger.Printf("control API listening on %s", composition.apiServer.Addr)

	logger.Print("gateway started; awaiting shutdown signal")
	<-ctx.Done()
	logger.Print("shutdown signal received, initiating graceful shutdown")

	shutdownCtx, shutdownCancel := runtime.newShutdownContext()
	defer shutdownCancel()

	shutdownStart := time.Now()
	runtime.performGracefulShutdown(shutdownCtx, logger, composition.shutdown)

	logger.Printf("shutdown completed in %v", time.Since(shutdownStart))
	return nil
}

func newGatewayShutdownContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), shutdownTimeout)
}

func composeGateway(ctx context.Context, logger *log.Logger, cfgPath string, mainCancel context.CancelFunc) (gatewayComposition, error) {
	appCfg, err := config.Load(ctx, cfgPath)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("load config: %w", err)
	}
	logger.Printf("configuration loaded: env=%s, providers=%d",
		appCfg.Environment, len(appCfg.Providers))

	if err := validateStartupRiskConfig(appCfg.Risk); err != nil {
		return gatewayComposition{}, fmt.Errorf("validate risk config: %w", err)
	}

	logger.Printf("providers configured: %d", len(appCfg.Providers))

	if err := runDatabaseMigrations(ctx, logger, appCfg.Database); err != nil {
		return gatewayComposition{}, fmt.Errorf("apply database migrations: %w", err)
	}

	dbPool, err := initDatabase(ctx, logger, appCfg.Database)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("connect database: %w", err)
	}
	providerStore := postgresstore.NewProviderStore(dbPool)
	strategyStore := postgresstore.NewStrategyStore(dbPool)
	orderStore := postgresstore.NewOrderStore(dbPool)
	outboxStore := postgresstore.NewOutboxStore(dbPool)

	telemetryProvider, err := initTelemetry(ctx, logger, appCfg)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("initialize telemetry: %w", err)
	}

	poolMgr, err := buildPoolManager(appCfg.Pools)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("initialise pools: %w", err)
	}

	lifecycle := &conc.WaitGroup{}
	bus := newEventBus(appCfg.Eventbus, poolMgr, outboxStore, logger)

	table := dispatcher.NewTable()
	providerManager, err := initProviders(ctx, logger, appCfg, poolMgr, table, bus, providerStore)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("initialise providers: %w", err)
	}

	registrar := dispatcher.NewRegistrar(table, providerManager)

	lambdaManager, err := startLambdaManager(ctx, appCfg, bus, poolMgr, providerManager, registrar, logger, strategyStore, orderStore)
	if err != nil {
		return gatewayComposition{}, fmt.Errorf("initialise lambdas: %w", err)
	}
	logger.Printf("strategy instances registered: %d", len(lambdaManager.Instances()))

	apiServer := buildAPIServer(appCfg, lambdaManager, providerManager, orderStore)
	return gatewayComposition{
		apiServer: apiServer,
		lifecycle: lifecycle,
		shutdown: gracefulShutdownConfig{
			server:     apiServer,
			mainCancel: mainCancel,
			lifecycle:  lifecycle,
			dataBus:    bus,
			poolMgr:    poolMgr,
			telemetry:  telemetryProvider,
			dbPool:     dbPool,
		},
	}, nil
}

func parseFlags() string {
	cfgPath := flag.String("config", "", fmt.Sprintf("Path to application configuration file (default: %s)", defaultConfigPath))
	flag.Parse()
	return *cfgPath
}

func newSignalContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
}

func newGatewayLogger() *log.Logger {
	return log.New(os.Stdout, gatewayLoggerPrefix, log.LstdFlags|log.Lmicroseconds)
}

func validateStartupRiskConfig(cfg config.RiskConfig) error {
	_, err := risk.ParseLimits(riskLimitsConfigFromConfig(cfg))
	if err != nil {
		return fmt.Errorf("validate startup risk config: %w", err)
	}
	return nil
}

func riskLimitsConfigFromConfig(cfg config.RiskConfig) risk.LimitsConfig {
	return risk.LimitsConfig{
		MaxPositionSize:     cfg.MaxPositionSize,
		MaxNotionalValue:    cfg.MaxNotionalValue,
		NotionalCurrency:    cfg.NotionalCurrency,
		OrderThrottle:       cfg.OrderThrottle,
		OrderBurst:          cfg.OrderBurst,
		MaxConcurrentOrders: cfg.MaxConcurrentOrders,
		PriceBandPercent:    cfg.PriceBandPercent,
		AllowedOrderTypes:   cfg.AllowedOrderTypes,
		KillSwitchEnabled:   cfg.KillSwitchEnabled,
		MaxRiskBreaches:     cfg.MaxRiskBreaches,
		CircuitBreaker: risk.CircuitBreakerConfig{
			Enabled:   cfg.CircuitBreaker.Enabled,
			Threshold: cfg.CircuitBreaker.Threshold,
			Cooldown:  cfg.CircuitBreaker.Cooldown,
		},
	}
}

func initTelemetry(ctx context.Context, logger *log.Logger, appCfg config.AppConfig) (*telemetry.Provider, error) {
	telemetryCfg := telemetry.DefaultConfig()
	if appCfg.Telemetry.OTLPEndpoint != "" {
		telemetryCfg.OTLPEndpoint = appCfg.Telemetry.OTLPEndpoint
	}
	if appCfg.Telemetry.ServiceName != "" {
		telemetryCfg.ServiceName = appCfg.Telemetry.ServiceName
	}
	telemetryCfg.Environment = string(appCfg.Environment)
	telemetryCfg.OTLPInsecure = appCfg.Telemetry.OTLPInsecure
	telemetryCfg.EnableMetrics = appCfg.Telemetry.EnableMetrics

	provider, err := telemetry.NewProvider(ctx, telemetryCfg)
	if err != nil {
		return nil, fmt.Errorf("initialize telemetry provider: %w", err)
	}

	if telemetryCfg.Enabled {
		logger.Printf("telemetry initialized: endpoint=%s, service=%s", telemetryCfg.OTLPEndpoint, telemetryCfg.ServiceName)
	} else {
		logger.Printf("telemetry disabled")
	}
	return provider, nil
}

func initDatabase(ctx context.Context, logger *log.Logger, dbCfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dbCfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse database dsn: %w", err)
	}

	poolCfg.MaxConns = dbCfg.MaxConns
	poolCfg.MinConns = dbCfg.MinConns
	poolCfg.MaxConnLifetime = dbCfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = dbCfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = dbCfg.HealthCheckPeriod
	poolCfg.ConnConfig.ConnectTimeout = databaseConnectTimeout

	connectCtx, cancel := context.WithTimeout(ctx, databaseConnectTimeout)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(connectCtx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}

	pingCtx, pingCancel := context.WithTimeout(ctx, databaseConnectTimeout)
	defer pingCancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("database ping: %w", err)
	}

	logger.Printf("database connected: maxConns=%d minConns=%d runMigrations=%t",
		poolCfg.MaxConns, poolCfg.MinConns, dbCfg.RunMigrations)

	postgresstore.ObservePoolMetrics(pool, "primary")

	return pool, nil
}

func buildPoolManager(cfg config.PoolConfig) (*pool.PoolManager, error) {
	manager := pool.NewPoolManager()
	eventQueueSize := cfg.Event.QueueSize()
	if err := manager.RegisterPool(eventPoolName, cfg.Event.Size, eventQueueSize, func() interface{} { return new(schema.Event) }); err != nil {
		return nil, fmt.Errorf("register Event pool: %w", err)
	}
	orderQueueSize := cfg.OrderRequest.QueueSize()
	if err := manager.RegisterPool(orderRequestPoolName, cfg.OrderRequest.Size, orderQueueSize, func() interface{} { return new(schema.OrderRequest) }); err != nil {
		return nil, fmt.Errorf("register OrderRequest pool: %w", err)
	}
	return manager, nil
}

func newEventBus(cfg config.EventbusConfig, pools *pool.PoolManager, outbox outboxstore.Store, logger *log.Logger) eventbus.Bus {
	memoryBus := eventbus.NewMemoryBus(eventbus.MemoryConfig{
		BufferSize:               cfg.BufferSize,
		FanoutWorkers:            cfg.FanoutWorkerCount(),
		ExtensionPayloadCapBytes: cfg.ExtensionPayloadCapBytes,
		Pools:                    pools,
	})
	return eventbus.NewDurableBus(
		memoryBus,
		outbox,
		eventbus.WithDurableLogger(logger),
		eventbus.WithDurablePoolManager(pools),
		eventbus.WithExtensionPayloadCapBytes(cfg.ExtensionPayloadCapBytes),
	)
}

func initProviders(ctx context.Context, logger *log.Logger, appCfg config.AppConfig, poolMgr *pool.PoolManager, table *dispatcher.Table, bus eventbus.Bus, store providerstore.Store) (*provider.Manager, error) {
	registry := provider.NewRegistry()
	adapters.RegisterAll(registry)

	opts := []provider.Option{}
	if store != nil {
		opts = append(opts, provider.WithPersistence(store))
	}
	manager := provider.NewManager(registry, poolMgr, bus, table, logger, opts...)
	manager.SetLifecycleContext(ctx)
	restoreProviderSnapshots(ctx, logger, store, manager)
	specs, err := config.BuildProviderSpecs(appCfg.Providers)
	if err != nil {
		return nil, fmt.Errorf("build provider specs: %w", err)
	}
	started := 0
	for _, spec := range specs {
		if manager.HasProvider(spec.Name) {
			if _, err := manager.Update(ctx, spec, true); err != nil {
				return nil, fmt.Errorf("update provider %s: %w", spec.Name, err)
			}
		} else {
			if _, err := manager.Create(ctx, spec, true); err != nil {
				return nil, fmt.Errorf("create provider %s: %w", spec.Name, err)
			}
		}
		started++
	}
	if started > 0 {
		logger.Printf("providers started: %d", len(manager.Providers()))
	} else {
		logger.Printf("no providers configured; skipping provider startup")
	}

	return manager, nil
}

func restoreProviderSnapshots(ctx context.Context, logger *log.Logger, store providerstore.Store, manager *provider.Manager) {
	if store == nil || manager == nil {
		return
	}
	snapshots, err := store.LoadProviders(ctx)
	if err != nil {
		if logger != nil {
			logger.Printf("provider persistence load failed: %v", err)
		}
		return
	}
	if len(snapshots) == 0 {
		return
	}
	for _, snapshot := range snapshots {
		manager.Restore(snapshot)
	}
	if logger != nil {
		logger.Printf("provider snapshots restored: %d", len(snapshots))
	}
}

func restoreStrategySnapshots(ctx context.Context, logger *log.Logger, store strategystore.Store, manager *lambdaruntime.Manager) {
	if store == nil || manager == nil {
		return
	}
	snapshots, err := store.Load(ctx)
	if err != nil {
		if logger != nil {
			logger.Printf("strategy persistence load failed: %v", err)
		}
		return
	}
	if len(snapshots) == 0 {
		return
	}
	for _, snapshot := range snapshots {
		manager.RestoreSnapshot(ctx, snapshot)
	}
	if logger != nil {
		logger.Printf("strategy snapshots restored: %d", len(snapshots))
	}
}

func startLambdaManager(ctx context.Context, appCfg config.AppConfig, bus eventbus.Bus, poolMgr *pool.PoolManager, providers *provider.Manager, registrar lambdaruntime.RouteRegistrar, logger *log.Logger, strategyStore strategystore.Store, orderStore orderstore.Store) (*lambdaruntime.Manager, error) {
	manager, err := lambdaruntime.NewManager(appCfg, bus, poolMgr, providers, logger, registrar,
		lambdaruntime.WithStrategyStore(strategyStore),
		lambdaruntime.WithOrderStore(orderStore),
	)
	if err != nil {
		return nil, fmt.Errorf("init lambda manager: %w", err)
	}
	manager.SetLifecycleContext(ctx)
	restoreStrategySnapshots(ctx, logger, strategyStore, manager)
	return manager, nil
}

func buildAPIServer(appCfg config.AppConfig, lambdaManager *lambdaruntime.Manager, providerManager *provider.Manager, orderStore orderstore.Store) *http.Server {
	handler := httpserver.NewHandler(appCfg, lambdaManager, providerManager, orderStore)

	return &http.Server{
		Addr:                         appCfg.APIServer.Addr,
		Handler:                      handler,
		DisableGeneralOptionsHandler: false,
		TLSConfig:                    nil,
		ReadTimeout:                  0,
		WriteTimeout:                 0,
		IdleTimeout:                  0,
		MaxHeaderBytes:               0,
		TLSNextProto:                 nil,
		ConnState:                    nil,
		ErrorLog:                     nil,
		BaseContext:                  nil,
		ConnContext:                  nil,
		HTTP2:                        nil,
		Protocols:                    nil,
		ReadHeaderTimeout:            controlReadHeaderTimeout,
	}
}

type apiServerStarter interface {
	ListenAndServe() error
}

func startAPIServer(lifecycle *conc.WaitGroup, logger *log.Logger, server apiServerStarter) {
	lifecycle.Go(func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("control server: %v", err)
		}
	})
}

type gracefulServer interface {
	Shutdown(context.Context) error
}

type lifecycleWaiter interface {
	Wait()
}

type dataBusCloser interface {
	Close()
}

type poolManagerShutdowner interface {
	Shutdown(context.Context) error
}

type telemetryShutdowner interface {
	Shutdown(context.Context) error
}

type databaseCloser interface {
	Close()
}

type gracefulShutdownConfig struct {
	server     gracefulServer
	mainCancel context.CancelFunc
	lifecycle  lifecycleWaiter
	dataBus    dataBusCloser
	poolMgr    poolManagerShutdowner
	telemetry  telemetryShutdowner
	dbPool     databaseCloser
}

func performGracefulShutdown(ctx context.Context, logger *log.Logger, cfg gracefulShutdownConfig) {
	shutdownStep := func(name string, timeout time.Duration, fn func(context.Context) error) {
		stepCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		logger.Printf("shutdown: %s...", name)
		if err := fn(stepCtx); err != nil {
			logger.Printf("shutdown: %s failed: %v", name, err)
		} else {
			logger.Printf("shutdown: %s completed", name)
		}
	}

	if cfg.server != nil {
		shutdownStep("stopping control server", controlServerShutdownTimeout, func(stepCtx context.Context) error {
			return cfg.server.Shutdown(stepCtx)
		})
	}

	logger.Print("shutdown: cancelling main context")
	if cfg.mainCancel != nil {
		cfg.mainCancel()
	}

	if cfg.lifecycle != nil {
		shutdownStep("waiting for lifecycle goroutines", lifecycleShutdownTimeout, func(stepCtx context.Context) error {
			done := make(chan struct{})
			go func() {
				cfg.lifecycle.Wait()
				close(done)
			}()
			select {
			case <-done:
				return nil
			case <-stepCtx.Done():
				return fmt.Errorf("timeout waiting for goroutines: %w", stepCtx.Err())
			}
		})
	}

	if cfg.dataBus != nil {
		shutdownStep("closing data bus", dataBusShutdownTimeout, func(stepCtx context.Context) error {
			done := make(chan struct{})
			go func() {
				cfg.dataBus.Close()
				close(done)
			}()
			select {
			case <-done:
				return nil
			case <-stepCtx.Done():
				return stepCtx.Err()
			}
		})
	}

	if cfg.poolMgr != nil {
		shutdownStep("shutting down pool manager", poolManagerShutdownTimeout, func(stepCtx context.Context) error {
			return cfg.poolMgr.Shutdown(stepCtx)
		})
	}

	if cfg.telemetry != nil {
		shutdownStep("shutting down telemetry", telemetryShutdownTimeout, func(stepCtx context.Context) error {
			return cfg.telemetry.Shutdown(stepCtx)
		})
	}

	if cfg.dbPool != nil {
		shutdownStep("closing database pool", databaseShutdownTimeout, func(context.Context) error {
			cfg.dbPool.Close()
			return nil
		})
	}
}

func resolveConfigPath(flagValue string) string {
	if strings.TrimSpace(flagValue) != "" {
		return filepath.Clean(flagValue)
	}

	if envPath := strings.TrimSpace(os.Getenv(configPathEnvVar)); envPath != "" {
		return filepath.Clean(envPath)
	}

	return filepath.Clean(defaultConfigPath)
}

func runDatabaseMigrations(ctx context.Context, logger *log.Logger, dbCfg config.DatabaseConfig) error {
	if !dbCfg.RunMigrations {
		logger.Printf("database migrations disabled; skipping")
		return nil
	}

	if err := migrations.Apply(ctx, dbCfg.DSN, "", logger); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}
