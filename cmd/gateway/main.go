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

	"github.com/coachpo/meltica/internal/adapters/binance"
	"github.com/coachpo/meltica/internal/adapters/fake"
	"github.com/coachpo/meltica/internal/adapters/shared"
	"github.com/coachpo/meltica/internal/bus/controlbus"
	"github.com/coachpo/meltica/internal/bus/databus"
	"github.com/coachpo/meltica/internal/config"
	"github.com/coachpo/meltica/internal/dispatcher"
	"github.com/coachpo/meltica/internal/lambda"
	"github.com/coachpo/meltica/internal/lambda/strategies"
	"github.com/coachpo/meltica/internal/pool"
	"github.com/coachpo/meltica/internal/schema"
	"github.com/coachpo/meltica/internal/telemetry"
	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/iter"
)

func main() {
	cfgPath := flag.String("config", "", "Path to streaming configuration file (default: streaming.yml or streaming.yaml alongside binary)")
	providers := flag.String("providers", "fake", "Comma-separated provider types: fake,binance (runs all simultaneously)")
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
	// Object pools for memory efficiency - avoid allocations on hot paths:
	//
	// WsFrame (200 capacity):
	//   - Used by WebSocket parsers to receive raw frames before canonicalization
	//   - Shared across all exchanges (Binance, Coinbase, Kraken, etc.)
	//   - Hot path: Every WebSocket message allocates/returns one frame
	//
	// ParseFrame (200 capacity):
	//   - Frame during parsing phase, before canonicalization to Event
	//   - Holds raw exchange-specific JSON during transformation
	//   - Exchange-agnostic: Supports all provider types (Binance, Coinbase, etc.)
	//   - Data flow: WsFrame → Parser → ParseFrame → Event
	//   - Hot path: Temporary holder during JSON parsing and normalization
	//
	// Event (1000 capacity):
	//   - The canonical event objects sent through the system
	//   - Highest capacity: Main message type flowing through all components
	//
	// OrderRequest (20 capacity):
	//   - Order request objects for trading operations
	//   - Lower capacity: Orders are less frequent than market data
	//
	registerPool("WsFrame", 200, func() interface{} { return new(schema.WsFrame) })
	registerPool("ParseFrame", 200, func() interface{} { return new(schema.ParseFrame) })
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

	// Create multiple providers running simultaneously
	providerList := parseProviders(*providers)
	logger.Printf("starting providers: %v", providerList)

	activeProviders, mergedEvents, mergedErrors := createMultipleProviders(ctx, providerList, poolMgr, logger)
	if len(activeProviders) == 0 {
		logger.Fatal("no providers started successfully")
	}
	logger.Printf("providers running: %d", len(activeProviders))

	//nolint:exhaustruct // optional fields use zero values
	runtimeCfg := config.DispatcherRuntimeConfig{
		StreamOrdering: config.StreamOrderingConfig{
			LatenessTolerance: 150 * time.Millisecond,
			FlushInterval:     50 * time.Millisecond,
			MaxBufferSize:     1024,
		},
	}

	// Dispatcher consumes merged event stream from all providers
	dispatcherRuntime := dispatcher.NewRuntime(bus, table, poolMgr, runtimeCfg, nil)
	dispatchErrs := dispatcherRuntime.Start(ctx, mergedEvents)

	// Log errors from all providers
	lifecycle.Go(func() {
		logErrors(logger, "providers", mergedErrors)
	})
	lifecycle.Go(func() {
		logErrors(logger, "dispatcher", dispatchErrs)
	})

	// Use first provider for subscription management and order submission
	// TODO: Support multi-provider routing in the future
	primaryProvider := activeProviders[0].provider
	subscriptionManager := shared.NewSubscriptionManager(primaryProvider)
	tradingState := dispatcher.NewTradingState()
	for _, route := range table.Routes() {
		// Activate route on all providers
		for _, p := range activeProviders {
			if err := p.provider.SubscribeRoute(route); err != nil {
				logger.Printf("subscribe route %s on %s: %v", route.Type, p.name, err)
			}
		}
	}

	// Create lambdas for all providers and symbols
	lambdaConfigs := []lambda.Config{
		{Symbol: "BTC-USDT", Provider: "fake"},
		{Symbol: "ETH-USDT", Provider: "fake"},
		{Symbol: "XRP-USDT", Provider: "fake"},
		{Symbol: "BTC-USDT", Provider: "binance"},
		{Symbol: "ETH-USDT", Provider: "binance"},
		{Symbol: "XRP-USDT", Provider: "binance"},
	}

	// Create lambda for each config - they'll filter by provider name
	for _, cfg := range lambdaConfigs {
		// Find the provider for this lambda
		var lambdaProvider MarketDataProvider
		for _, p := range activeProviders {
			if p.name == cfg.Provider {
				lambdaProvider = p.provider
				break
			}
		}
		if lambdaProvider == nil {
			continue // Provider not active
		}

		lam := lambda.NewBaseLambda("", cfg, bus, controlBus, lambdaProvider, poolMgr, &strategies.Logging{Logger: logger})

		if lambdaErrs, err := lam.Start(ctx); err != nil {
			logger.Fatalf("start lambda %s/%s: %v", cfg.Provider, cfg.Symbol, err)
		} else {
			lifecycle.Go(func() {
				for err := range lambdaErrs {
					if err != nil {
						logger.Printf("lambda %s/%s: %v", cfg.Provider, cfg.Symbol, err)
					}
				}
			})
		}
	}

	controller := dispatcher.NewController(
		table,
		controlBus,
		subscriptionManager,
		dispatcher.WithOrderSubmitter(primaryProvider),
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
		controlBus:          controlBus,
		bus:                 bus,
		poolMgr:             poolMgr,
		telemetryProvider:   telemetryProvider,
		dispatcherRuntime:   dispatcherRuntime,
		subscriptionManager: subscriptionManager,
	})

	logger.Printf("shutdown completed in %v", time.Since(shutdownStart))
}

// MarketDataProvider defines the interface for market data providers.
type MarketDataProvider interface {
	Events() <-chan *schema.Event
	Errors() <-chan error
	SubmitOrder(ctx context.Context, req schema.OrderRequest) error
	SubscribeRoute(route dispatcher.Route) error
	UnsubscribeRoute(typ schema.CanonicalType) error
}

type gracefulShutdownConfig struct {
	controlServer       *http.Server
	mainContextCancel   context.CancelFunc
	lifecycle           *conc.WaitGroup
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

func createProvider(ctx context.Context, providerType string, poolMgr *pool.PoolManager, logger *log.Logger) (MarketDataProvider, string, error) {
	switch providerType {
	case "fake":
		//nolint:exhaustruct // optional fields use zero values
		provider := fake.NewProvider(fake.Options{
			Name:           "fake",
			TickerInterval: 1000 * time.Microsecond,
			TradeInterval:  1000 * time.Microsecond,
			Pools:          poolMgr,
		})
		if err := provider.Start(ctx); err != nil {
			return nil, "", fmt.Errorf("start fake provider: %w", err)
		}
		return provider, "fake", nil

	case "binance":
		// Production Binance provider with real WebSocket and REST connections
		parser := binance.NewParserWithPool("binance", poolMgr)

		// Get API credentials from environment
		apiKey := os.Getenv("BINANCE_API_KEY")
		secretKey := os.Getenv("BINANCE_SECRET_KEY")
		useTestnet := os.Getenv("BINANCE_USE_TESTNET") == "true"

		// Create WebSocket provider with reconnection and rate limiting
		//nolint:exhaustruct // optional fields use defaults
		wsProvider := binance.NewBinanceWSProvider(binance.WSProviderConfig{
			UseTestnet:    useTestnet,
			APIKey:        apiKey,
			MaxReconnects: 10,
		})

		// Create REST fetcher with authentication
		restFetcher := binance.NewBinanceRESTFetcher(apiKey, secretKey, useTestnet)

		// Create WebSocket client with real provider
		wsClient := binance.NewWSClient("binance", wsProvider, parser, time.Now, poolMgr)

		// Create REST client with real fetcher
		restClient := binance.NewRESTClient(restFetcher, parser, time.Now)

		// Configure market data streams
		topics := []string{
			"btcusdt@depth@100ms", // Orderbook depth updates
			"ethusdt@depth@100ms",
			"xrpusdt@depth@100ms",
			"btcusdt@aggTrade", // Aggregate trades
			"ethusdt@aggTrade",
			"xrpusdt@aggTrade",
			"btcusdt@ticker", // 24hr ticker
			"ethusdt@ticker",
			"xrpusdt@ticker",
			"btcusdt@kline_1m", // 1-minute klines
			"ethusdt@kline_1m",
			"xrpusdt@kline_1m",
		}

		// Configure REST snapshot pollers
		snapshots := []binance.RESTPoller{
			{
				Name:     "orderbook",
				Endpoint: "https://api.binance.com/api/v3/depth?symbol=BTCUSDT&limit=1000",
				Interval: 30 * time.Second,
				Parser:   "orderbook",
			},
			{
				Name:     "orderbook",
				Endpoint: "https://api.binance.com/api/v3/depth?symbol=ETHUSDT&limit=1000",
				Interval: 30 * time.Second,
				Parser:   "orderbook",
			},
			{
				Name:     "orderbook",
				Endpoint: "https://api.binance.com/api/v3/depth?symbol=XRPUSDT&limit=1000",
				Interval: 30 * time.Second,
				Parser:   "orderbook",
			},
		}

		if useTestnet {
			// Adjust endpoints for testnet
			for i := range snapshots {
				snapshots[i].Endpoint = "https://testnet.binance.vision" + snapshots[i].Endpoint[len("https://api.binance.com"):]
			}
		}

		//nolint:exhaustruct // optional fields use zero values
		provider := binance.NewProvider("binance", wsClient, restClient, binance.ProviderOptions{
			Topics:    topics,
			Snapshots: snapshots,
			Pools:     poolMgr,
		})

		if err := provider.Start(ctx); err != nil {
			return nil, "", fmt.Errorf("start binance provider: %w", err)
		}

		logger.Printf("binance: testnet=%v, api_key=%s, streams=%d",
			useTestnet,
			maskAPIKey(apiKey),
			len(topics))

		return provider, "binance", nil

	default:
		return nil, "", fmt.Errorf("unknown provider type: %s (supported: fake, binance)", providerType)
	}
}

func maskAPIKey(key string) string {
	if key == "" {
		return "<none>"
	}
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

// parseProviders splits comma-separated provider types.
func parseProviders(input string) []string {
	if input == "" {
		return []string{"fake"}
	}
	parts := []string{}
	for _, part := range []string{"fake", "binance"} {
		if contains(input, part) {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{"fake"}
	}
	return parts
}

func contains(s, substr string) bool {
	if len(s) == 0 || len(substr) == 0 {
		return false
	}
	if s == substr {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Check if starts with substr
	if s[:len(substr)] == substr {
		return true
	}
	// Check if ends with substr
	if s[len(s)-len(substr):] == substr {
		return true
	}
	// Check if substr appears with comma (comma-separated list)
	if len(s) > len(substr) {
		// Check "substr," at start
		if len(s) >= len(substr)+1 && s[:len(substr)+1] == substr+"," {
			return true
		}
		// Check ",substr" at end
		if len(s) >= len(substr)+1 && s[len(s)-len(substr)-1:] == ","+substr {
			return true
		}
	}
	return false
}

type providerInstance struct {
	name     string
	provider MarketDataProvider
}

// createMultipleProviders starts all requested providers and merges their event streams.
func createMultipleProviders(ctx context.Context, providerTypes []string, poolMgr *pool.PoolManager, logger *log.Logger) ([]providerInstance, <-chan *schema.Event, <-chan error) {
	instances := []providerInstance{}
	eventChannels := []<-chan *schema.Event{}
	errorChannels := []<-chan error{}

	for _, provType := range providerTypes {
		provider, name, err := createProvider(ctx, provType, poolMgr, logger)
		if err != nil {
			logger.Printf("failed to create %s provider: %v", provType, err)
			continue
		}
		instances = append(instances, providerInstance{
			name:     name,
			provider: provider,
		})
		eventChannels = append(eventChannels, provider.Events())
		errorChannels = append(errorChannels, provider.Errors())
		logger.Printf("provider %s started successfully", name)
	}

	// Merge all event channels into one
	mergedEvents := make(chan *schema.Event, 512)
	mergedErrors := make(chan error, 64)

	// Fan-in events from all providers
	for _, evtChan := range eventChannels {
		evtChan := evtChan
		go func(ch <-chan *schema.Event) {
			for evt := range ch {
				select {
				case mergedEvents <- evt:
				case <-ctx.Done():
					return
				}
			}
		}(evtChan)
	}

	// Fan-in errors from all providers
	for i, errChan := range errorChannels {
		provName := instances[i].name
		go func(ch <-chan error, name string) {
			for err := range ch {
				select {
				case mergedErrors <- fmt.Errorf("%s: %w", name, err):
				case <-ctx.Done():
					return
				default:
				}
			}
		}(errChan, provName)
	}

	// Close merged channels when all sources are done
	go func() {
		<-ctx.Done()
		close(mergedEvents)
		close(mergedErrors)
	}()

	return instances, mergedEvents, mergedErrors
}
