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
	"syscall"
	"time"

	"github.com/coachpo/meltica/internal/adapters/fake"
	"github.com/coachpo/meltica/internal/adapters/shared"
	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/config"
	"github.com/coachpo/meltica/internal/consumer"
	"github.com/coachpo/meltica/internal/dispatcher"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/telemetry"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
)

func main() {
	cfgPath := flag.String("config", "", "Path to streaming configuration file (default: streaming.yml or streaming.yaml alongside binary)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	streamingCfg, err := config.LoadStreamingConfig(ctx, resolveConfigPath(*cfgPath))
	if err != nil {
		log.Fatalf("load streaming config: %v", err)
	}

	logger := log.New(os.Stdout, "gateway ", log.LstdFlags|log.Lmicroseconds)
	logger.Printf("configuration loaded: routes=%d", len(streamingCfg.Dispatcher.Routes))

	telemetryCfg := telemetry.DefaultConfig()
	telemetryProvider, err := telemetry.NewProvider(ctx, telemetryCfg)
	if err != nil {
		log.Fatalf("initialize telemetry: %v", err)
	}
	if telemetryCfg.Enabled {
		logger.Printf("telemetry initialized: endpoint=%s", telemetryCfg.OTLPEndpoint)
	} else {
		logger.Printf("telemetry disabled")
	}

	poolMgr := pool.NewPoolManager()
	registerPool := func(name string, capacity int, factory func() interface{}) {
		if err := poolMgr.RegisterPool(name, capacity, factory); err != nil {
			log.Fatalf("register pool %s: %v", name, err)
		}
	}
	registerPool("WsFrame", 200, func() interface{} { return new(schema.WsFrame) })
	registerPool("ProviderRaw", 200, func() interface{} { return new(schema.ProviderRaw) })
	registerPool("Event", 1000, func() interface{} { return new(schema.Event) })
	registerPool("OrderRequest", 20, func() interface{} { return new(schema.OrderRequest) })

	var lifecycle conc.WaitGroup

	bus := databus.NewMemoryBus(databus.MemoryConfig{
		BufferSize:    streamingCfg.Databus.BufferSize,
		FanoutWorkers: 8,
		Pools:         poolMgr,
	})

	controlBus := controlbus.NewMemoryBus(controlbus.MemoryConfig{BufferSize: 16})

	table := dispatcher.NewTable()
	for name, cfg := range streamingCfg.Dispatcher.Routes {
		if err := table.Upsert(routeFromConfig(name, cfg)); err != nil {
			log.Fatalf("load route %s: %v", name, err)
		}
	}

	//nolint:exhaustruct // optional fields use zero values
	provider := fake.NewProvider(fake.Options{
		Name:               "fake",
		TickerInterval:     1000 * time.Microsecond,
		TradeInterval:      1000 * time.Microsecond,
		BookUpdateInterval: 1000 * time.Microsecond,
		Pools:              poolMgr,
	})
	if err := provider.Start(ctx); err != nil {
		logger.Fatalf("start provider: %v", err)
	}

	//nolint:exhaustruct // optional fields use zero values
	runtimeCfg := config.DispatcherRuntimeConfig{
		StreamOrdering: config.StreamOrderingConfig{
			LatenessTolerance: 150 * time.Millisecond,
			FlushInterval:     50 * time.Millisecond,
			MaxBufferSize:     1024,
		},
	}

	dispatcherRuntime := dispatcher.NewRuntime(bus, table, poolMgr, runtimeCfg, nil)
	dispatchErrs := dispatcherRuntime.Start(ctx, provider.Events())

	lifecycle.Go(func() {
		logErrors(logger, "provider", provider.Errors())
	})
	lifecycle.Go(func() {
		logErrors(logger, "dispatcher", dispatchErrs)
	})

	subscriptionManager := shared.NewSubscriptionManager(provider)
	tradingState := dispatcher.NewTradingState()
	for _, route := range table.Routes() {
		if err := subscriptionManager.Activate(ctx, route); err != nil {
			logger.Printf("subscribe route %s: %v", route.Type, err)
		}
	}

	lambdaConfigs := []consumer.LambdaConfig{
		{Symbol: "BTC-USDT", Provider: "fake"},
		{Symbol: "ETH-USDT", Provider: "fake"},
		{Symbol: "XRP-USDT", Provider: "fake"},
	}

	lambdas := make([]*consumer.Lambda, 0, len(lambdaConfigs))
	for _, cfg := range lambdaConfigs {
		lambda := consumer.NewLambda("", cfg, bus, controlBus, provider, poolMgr, logger)
		lambdas = append(lambdas, lambda)
		
		if lambdaErrs, err := lambda.Start(ctx); err != nil {
			logger.Fatalf("start lambda %s: %v", cfg.Symbol, err)
		} else {
			lifecycle.Go(func() {
				for err := range lambdaErrs {
					if err != nil {
						logger.Printf("lambda %s: %v", cfg.Symbol, err)
					}
				}
			})
		}
	}
	controller := dispatcher.NewController(
		table,
		controlBus,
		subscriptionManager,
		dispatcher.WithOrderSubmitter(provider),
		dispatcher.WithTradingState(tradingState),
		dispatcher.WithControlPublisher(bus, poolMgr),
	)
	lifecycle.Go(func() {
		if err := controller.Start(ctx); err != nil && err != context.Canceled {
			logger.Printf("controller: %v", err)
		}
	})

	controlAddr := ":8880"
	controlHandler := dispatcher.NewControlHTTPHandler(controlBus)
	controlServer := new(http.Server)
	controlServer.Addr = controlAddr
	controlServer.Handler = controlHandler
	controlServer.ReadHeaderTimeout = 5 * time.Second
	lifecycle.Go(func() {
		if err := controlServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("control server: %v", err)
		}
	})
	logger.Printf("control API listening on %s", controlAddr)

	logger.Print("gateway started; awaiting shutdown signal")
	<-ctx.Done()
	logger.Print("shutdown signal received, initiating graceful shutdown")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	shutdownStart := time.Now()
	performGracefulShutdown(shutdownCtx, logger, gracefulShutdownConfig{
		controlServer:       controlServer,
		mainContextCancel:   cancel,
		lifecycle:           &lifecycle,
		provider:            provider,
		controlBus:          controlBus,
		bus:                 bus,
		poolMgr:             poolMgr,
		telemetryProvider:   telemetryProvider,
		dispatcherRuntime:   dispatcherRuntime,
		subscriptionManager: subscriptionManager,
	})

	logger.Printf("shutdown completed in %v", time.Since(shutdownStart))
}

type gracefulShutdownConfig struct {
	controlServer       *http.Server
	mainContextCancel   context.CancelFunc
	lifecycle           *conc.WaitGroup
	provider            *fake.Provider
	controlBus          controlbus.Bus
	bus                 databus.Bus
	poolMgr             *pool.PoolManager
	telemetryProvider   *telemetry.Provider
	dispatcherRuntime   *dispatcher.Runtime
	subscriptionManager *shared.SubscriptionManager
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

	shutdownStep("stopping control server", 5*time.Second, func(stepCtx context.Context) error {
		return cfg.controlServer.Shutdown(stepCtx)
	})

	logger.Print("shutdown: cancelling main context")
	cfg.mainContextCancel()

	shutdownStep("waiting for lifecycle goroutines", 10*time.Second, func(stepCtx context.Context) error {
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

	shutdownStep("closing control bus", 2*time.Second, func(stepCtx context.Context) error {
		done := make(chan struct{})
		go func() {
			cfg.controlBus.Close()
			close(done)
		}()
		select {
		case <-done:
			return nil
		case <-stepCtx.Done():
			return stepCtx.Err()
		}
	})

	shutdownStep("closing data bus", 2*time.Second, func(stepCtx context.Context) error {
		done := make(chan struct{})
		go func() {
			cfg.bus.Close()
			close(done)
		}()
		select {
		case <-done:
			return nil
		case <-stepCtx.Done():
			return stepCtx.Err()
		}
	})

	shutdownStep("shutting down pool manager", 5*time.Second, func(stepCtx context.Context) error {
		return cfg.poolMgr.Shutdown(stepCtx)
	})

	shutdownStep("shutting down telemetry", 5*time.Second, func(stepCtx context.Context) error {
		return cfg.telemetryProvider.Shutdown(stepCtx)
	})
}

func routeFromConfig(name string, cfg config.RouteConfig) dispatcher.Route {
	filters := iter.Map(cfg.Filters, func(f *config.FilterRuleConfig) dispatcher.FilterRule {
		return dispatcher.FilterRule{Field: f.Field, Op: f.Op, Value: f.Value}
	})
	restFns := iter.Map(cfg.RestFns, func(rf *config.RestFnConfig) dispatcher.RestFn {
		return dispatcher.RestFn{Name: rf.Name, Endpoint: rf.Endpoint, Interval: rf.Interval, Parser: rf.Parser}
	})
	return dispatcher.Route{
		Type:     schema.CanonicalType(name),
		WSTopics: cfg.WSTopics,
		RestFns:  restFns,
		Filters:  filters,
	}
}

func logErrors(logger *log.Logger, stage string, errs <-chan error) {
	for err := range errs {
		if err != nil {
			logger.Printf("%s: %v", stage, err)
		}
	}
}

func resolveConfigPath(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("failed to determine executable path: %v", err)
	}
	execDir := filepath.Dir(execPath)

	for _, name := range []string{"streaming.yml", "streaming.yaml"} {
		path := filepath.Join(execDir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	log.Fatalf("config file not found: expected streaming.yml or streaming.yaml in %s", execDir)
	return ""
}
