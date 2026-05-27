// Package runtime manages lambda lifecycle orchestration and strategy execution.
package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"

	"github.com/coachpo/meltica/internal/app/dispatcher"
	"github.com/coachpo/meltica/internal/app/lambda/core"
	"github.com/coachpo/meltica/internal/app/lambda/js"
	"github.com/coachpo/meltica/internal/app/lambda/strategies"
	"github.com/coachpo/meltica/internal/app/provider"
	"github.com/coachpo/meltica/internal/app/risk"
	"github.com/coachpo/meltica/internal/domain/orderstore"
	"github.com/coachpo/meltica/internal/domain/strategystore"
	"github.com/coachpo/meltica/internal/infra/bus/eventbus"
	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/coachpo/meltica/internal/infra/pool"
)

var (
	// ErrInstanceExists is returned when attempting to create an instance that already exists.
	ErrInstanceExists = errors.New("strategy instance already exists")
	// ErrInstanceNotFound is returned when attempting to access an instance that doesn't exist.
	ErrInstanceNotFound = errors.New("strategy instance not found")
	// ErrInstanceAlreadyRunning is returned when attempting to start an already running instance.
	ErrInstanceAlreadyRunning = errors.New("strategy instance already running")
	// ErrInstanceNotRunning is returned when attempting to stop an instance that isn't running.
	ErrInstanceNotRunning = errors.New("strategy instance not running")
)

// StrategyFactory creates trading strategy instances from configuration.
type StrategyFactory func(config map[string]any) (core.TradingStrategy, error)

// StrategyDefinition combines strategy metadata with a factory function.
type StrategyDefinition struct {
	meta    strategies.Metadata
	factory StrategyFactory
}

// Metadata returns the strategy metadata.
func (d StrategyDefinition) Metadata() strategies.Metadata {
	return strategies.CloneMetadata(d.meta)
}

// ProviderCatalog provides access to available providers.
type ProviderCatalog interface {
	Provider(name string) (provider.Instance, bool)
}

// RouteRegistrar manages dynamic route registration for providers.
type RouteRegistrar interface {
	RegisterLambda(ctx context.Context, lambdaID string, providers []string, routes []dispatcher.RouteDeclaration) error
	RegisterLambdaBatch(ctx context.Context, regs []dispatcher.LambdaBatchRegistration) error
	UnregisterLambda(ctx context.Context, lambdaID string) error
}

// Manager coordinates lambda lifecycle and strategy execution.
type Manager struct {
	mu sync.RWMutex

	lifecycleMu  sync.RWMutex
	lifecycleCtx context.Context

	bus              eventbus.Bus
	pools            *pool.PoolManager
	providers        ProviderCatalog
	logger           *log.Logger
	registrar        RouteRegistrar
	riskManager      *risk.Manager
	jsLoader         *js.Loader
	dynamic          map[string]struct{}
	baseline         map[string]struct{}
	dynamicInstances map[string]struct{}
	clock            func() time.Time
	strategyDir      string
	base             map[string]StrategyDefinition

	strategies    map[string]StrategyDefinition
	specs         map[string]config.LambdaSpec
	instances     map[string]*lambdaInstance
	strategyStore strategystore.Store
	orderStore    orderstore.Store

	revisionUsage            map[string]*revisionUsage
	revisionGauge            metric.Int64ObservableGauge
	revisionLifecycleMetric  metric.Int64Counter
	uploadValidationFailures metric.Int64Counter
	tagAssignmentCounter     metric.Int64Counter
	tagDeleteCounter         metric.Int64Counter
}

// Option configures manager behaviour.
type Option func(*Manager)

// WithStrategyStore wires a strategy persistence store into the manager.
func WithStrategyStore(store strategystore.Store) Option {
	return func(m *Manager) {
		m.strategyStore = store
	}
}

// WithOrderStore wires an order persistence store into the manager.
func WithOrderStore(store orderstore.Store) Option {
	return func(m *Manager) {
		m.orderStore = store
	}
}

type lambdaInstance struct {
	base   *core.BaseLambda
	cancel context.CancelFunc
	errs   <-chan error
	strat  core.TradingStrategy
	revKey string
}

// NewManager creates a new lambda manager with the specified dependencies.
func NewManager(cfg config.AppConfig, bus eventbus.Bus, pools *pool.PoolManager, providers ProviderCatalog, logger *log.Logger, registrar RouteRegistrar, opts ...Option) (*Manager, error) {
	if logger == nil {
		logger = log.New(os.Stdout, "lambda-manager ", log.LstdFlags|log.Lmicroseconds)
	}

	limits, err := parseRiskConfig(cfg.Risk)
	if err != nil {
		return nil, fmt.Errorf("lambda manager: parse risk limits: %w", err)
	}
	rm := risk.NewManager(limits)

	dir := strings.TrimSpace(cfg.Strategies.Directory)
	if dir == "" {
		dir = "strategies"
	}

	loader, err := js.NewLoader(dir)
	if err != nil {
		return nil, fmt.Errorf("lambda manager: create loader: %w", err)
	}
	if cfg.Strategies.RequireRegistry {
		registryPath := filepath.Join(loader.Root(), "registry.json")
		if _, err := os.Stat(registryPath); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil, fmt.Errorf("lambda manager: registry required but %s missing", registryPath)
			}
			return nil, fmt.Errorf("lambda manager: check registry: %w", err)
		}
	}

	mgr := &Manager{
		mu:                       sync.RWMutex{},
		lifecycleMu:              sync.RWMutex{},
		lifecycleCtx:             context.Background(),
		bus:                      bus,
		pools:                    pools,
		providers:                providers,
		logger:                   logger,
		registrar:                registrar,
		riskManager:              rm,
		jsLoader:                 loader,
		dynamic:                  make(map[string]struct{}),
		baseline:                 make(map[string]struct{}),
		dynamicInstances:         make(map[string]struct{}),
		clock:                    time.Now,
		strategyDir:              loader.Root(),
		base:                     make(map[string]StrategyDefinition),
		strategies:               make(map[string]StrategyDefinition),
		specs:                    make(map[string]config.LambdaSpec),
		instances:                make(map[string]*lambdaInstance),
		strategyStore:            nil,
		orderStore:               nil,
		revisionUsage:            make(map[string]*revisionUsage),
		revisionGauge:            nil,
		revisionLifecycleMetric:  nil,
		uploadValidationFailures: nil,
		tagAssignmentCounter:     nil,
		tagDeleteCounter:         nil,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(mgr)
		}
	}
	mgr.setupMetrics()
	if _, err := mgr.installJavaScriptStrategies(context.Background()); err != nil {
		return nil, fmt.Errorf("lambda manager: install javascript strategies: %w", err)
	}
	return mgr, nil
}

func (m *Manager) setupMetrics() {
	if m == nil {
		return
	}
	meter := otel.Meter("lambda-manager")
	gauge, err := meter.Int64ObservableGauge("strategy_revision_instances",
		metric.WithDescription("Number of running lambda instances per strategy revision"),
		metric.WithUnit("{instance}"),
		metric.WithInt64Callback(m.observeRevisionUsage),
	)
	if err == nil {
		m.revisionGauge = gauge
	} else if m.logger != nil {
		m.logger.Printf("lambda manager: register revision gauge: %v", err)
	}
	counter, err := meter.Int64Counter("strategy_revision_instances_total",
		metric.WithDescription("Lifecycle transitions for strategy revisions"),
		metric.WithUnit("{event}"),
	)
	if err == nil {
		m.revisionLifecycleMetric = counter
	} else if m.logger != nil {
		m.logger.Printf("lambda manager: register revision counter: %v", err)
	}
	failures, err := meter.Int64Counter("strategy.upload.validation_failure_total",
		metric.WithDescription("Strategy upload validation failures by stage"),
		metric.WithUnit("{event}"),
	)
	if err == nil {
		m.uploadValidationFailures = failures
	} else if m.logger != nil {
		m.logger.Printf("lambda manager: register validation failure counter: %v", err)
	}
	assigned, err := meter.Int64Counter("strategy_tag_reassigned_total",
		metric.WithDescription("Tag reassignments per strategy"),
		metric.WithUnit("{event}"),
	)
	if err == nil {
		m.tagAssignmentCounter = assigned
	} else if m.logger != nil {
		m.logger.Printf("lambda manager: register tag reassignment counter: %v", err)
	}
	deleted, err := meter.Int64Counter("strategy_tag_deleted_total",
		metric.WithDescription("Tag deletion events per strategy"),
		metric.WithUnit("{event}"),
	)
	if err == nil {
		m.tagDeleteCounter = deleted
	} else if m.logger != nil {
		m.logger.Printf("lambda manager: register tag deletion counter: %v", err)
	}
}
