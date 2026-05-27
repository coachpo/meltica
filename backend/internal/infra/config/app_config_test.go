package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/infra/bus/eventbus"
)

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(context.Background(), filepath.Join(t.TempDir(), "missing.yaml"))
	if err == nil {
		t.Fatalf("expected error when config file missing")
	}
}

func TestLoadDuplicateProviderName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	yaml := `
environment: dev
providers:
  BinanceSpot: {}
  binanceSpot: {}
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	_, err := Load(context.Background(), path)
	if err == nil {
		t.Fatalf("expected error when duplicate provider names supplied")
	}
	if !strings.Contains(err.Error(), `duplicate provider name "binanceSpot"`) {
		t.Fatalf("expected duplicate provider name error, got %v", err)
	}
}

func TestLoadCIConfig(t *testing.T) {
	cfg, err := Load(context.Background(), filepath.Join("..", "..", "..", "config", "app.ci.yaml"))
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Environment != EnvCI {
		t.Fatalf("expected environment %s, got %s", EnvCI, cfg.Environment)
	}
	if cfg.APIServer.AuthToken != "ci-placeholder-token" {
		t.Fatalf("expected CI auth token placeholder to load, got %q", cfg.APIServer.AuthToken)
	}
}

func TestValidateEnvironment(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Environment = EnvCI
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected ci environment to validate, got %v", err)
	}
}

func TestValidateEnvironmentRejectsUnknown(t *testing.T) {
	cfg := validBaseConfig()
	cfg.Environment = Environment("qa")
	if err := cfg.Validate(); err == nil {
		t.Fatalf("expected unknown environment to fail validation")
	}
}

func TestAPIServerAuthTokenValidation(t *testing.T) {
	for _, env := range []Environment{EnvStaging, EnvProd, EnvCI} {
		t.Run(string(env)+" requires token", func(t *testing.T) {
			cfg := validBaseConfig()
			cfg.Environment = env
			cfg.APIServer.AuthToken = ""
			err := cfg.Validate()
			if err == nil {
				t.Fatalf("expected %s to require apiServer authToken", env)
			}
			if !strings.Contains(err.Error(), "apiServer authToken required") {
				t.Fatalf("expected authToken validation error, got %v", err)
			}
		})

		t.Run(string(env)+" accepts token", func(t *testing.T) {
			cfg := validBaseConfig()
			cfg.Environment = env
			cfg.APIServer.AuthToken = "safe-test-token"
			if err := cfg.Validate(); err != nil {
				t.Fatalf("expected %s with token to validate, got %v", env, err)
			}
		})
	}

	cfg := validBaseConfig()
	cfg.Environment = EnvDev
	cfg.APIServer.AuthToken = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected dev without token to validate, got %v", err)
	}
}

func TestLoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	yaml := `
environment: DEV
providers:
  BinanceSpot:
    adapter:
      identifier: binance
      config:
        option: value
eventbus:
  bufferSize: 128
  fanoutWorkers: 4
pools:
  event:
    size: 100
    waitQueueSize: 110
  orderRequest:
    size: 50
    waitQueueSize: 60
risk:
  maxPositionSize: "10"
  maxNotionalValue: "1000"
  notionalCurrency: "USD"
  orderThrottle: 5
  orderBurst: 1

apiServer:
  addr: ":9999"
telemetry:
  otlpEndpoint: http://localhost:4318
  serviceName: test-service
  otlpInsecure: true
  enableMetrics: false
database:
  dsn: postgresql://localhost:5432/meltica?sslmode=disable
  maxConns: 32
  minConns: 4
  maxConnLifetime: 45m
  maxConnIdleTime: 10m
  healthCheckPeriod: 1m
  runMigrations: true
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Environment != EnvDev {
		t.Fatalf("expected environment %s, got %s", EnvDev, cfg.Environment)
	}

	ex, ok := cfg.Providers[Provider("binanceSpot")]
	if !ok {
		t.Fatalf("expected binance adapter config")
	}
	rawAdapter := ex["adapter"]
	adapterCfg, ok := rawAdapter.(map[string]any)
	if !ok {
		t.Fatalf("expected adapter config map, got %T", rawAdapter)
	}
	if id := adapterCfg["identifier"]; id != "binance" {
		t.Fatalf("expected identifier binance, got %v", id)
	}
	rawNested := adapterCfg["config"]
	nestedCfg, ok := rawNested.(map[string]any)
	if !ok {
		t.Fatalf("expected nested config map, got %T", rawNested)
	}
	if got := nestedCfg["option"]; got != "value" {
		t.Fatalf("expected adapter option value, got %v", got)
	}

	if cfg.Eventbus.BufferSize != 128 {
		t.Fatalf("expected buffer size 128, got %d", cfg.Eventbus.BufferSize)
	}
	if workers := cfg.Eventbus.FanoutWorkerCount(); workers != 4 {
		t.Fatalf("expected fanout workers 4, got %d", workers)
	}
	if cfg.Eventbus.ExtensionPayloadCapBytes != eventbus.DefaultExtensionPayloadCapBytes {
		t.Fatalf("expected default extension payload cap %d, got %d", eventbus.DefaultExtensionPayloadCapBytes, cfg.Eventbus.ExtensionPayloadCapBytes)
	}

	if cfg.APIServer.Addr != ":9999" {
		t.Fatalf("expected api server addr :9999, got %s", cfg.APIServer.Addr)
	}

	if cfg.Telemetry.ServiceName != "test-service" {
		t.Fatalf("expected telemetry service name test-service, got %s", cfg.Telemetry.ServiceName)
	}
	if cfg.Telemetry.EnableMetrics {
		t.Fatalf("expected telemetry metrics disabled")
	}

	if cfg.Strategies.Directory != "strategies" {
		t.Fatalf("expected default strategies directory, got %q", cfg.Strategies.Directory)
	}

	if cfg.Pools.Event.Size != 100 {
		t.Fatalf("expected pool event size 100, got %d", cfg.Pools.Event.Size)
	}
	if cfg.Pools.Event.WaitQueueSize != 110 {
		t.Fatalf("expected pool event queue size 110, got %d", cfg.Pools.Event.WaitQueueSize)
	}
	if cfg.Pools.OrderRequest.Size != 50 {
		t.Fatalf("expected pool order request size 50, got %d", cfg.Pools.OrderRequest.Size)
	}
	if cfg.Pools.OrderRequest.WaitQueueSize != 60 {
		t.Fatalf("expected pool order request queue size 60, got %d", cfg.Pools.OrderRequest.WaitQueueSize)
	}

	if cfg.Risk.OrderBurst != 1 {
		t.Fatalf("expected order burst 1, got %d", cfg.Risk.OrderBurst)
	}
	if cfg.Risk.MaxConcurrentOrders != 0 {
		t.Fatalf("expected default max concurrent orders 0, got %d", cfg.Risk.MaxConcurrentOrders)
	}

	if cfg.Database.DSN != "postgresql://localhost:5432/meltica?sslmode=disable" {
		t.Fatalf("unexpected database DSN %q", cfg.Database.DSN)
	}
	if cfg.Database.MaxConns != 32 {
		t.Fatalf("expected database maxConns 32, got %d", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 4 {
		t.Fatalf("expected database minConns 4, got %d", cfg.Database.MinConns)
	}
	if cfg.Database.MaxConnLifetime != 45*time.Minute {
		t.Fatalf("expected database maxConnLifetime 45m, got %s", cfg.Database.MaxConnLifetime)
	}
	if cfg.Database.MaxConnIdleTime != 10*time.Minute {
		t.Fatalf("expected database maxConnIdleTime 10m, got %s", cfg.Database.MaxConnIdleTime)
	}
	if cfg.Database.HealthCheckPeriod != time.Minute {
		t.Fatalf("expected database healthCheckPeriod 1m, got %s", cfg.Database.HealthCheckPeriod)
	}
	if !cfg.Database.RunMigrations {
		t.Fatalf("expected database runMigrations to be true")
	}
}

func TestFanoutWorkersAuto(t *testing.T) {
	cfg := loadConfigWithFanout(t, "  fanoutWorkers: auto\n")
	expected := runtime.NumCPU()
	if expected <= 0 {
		expected = 4
	}
	if workers := cfg.Eventbus.FanoutWorkerCount(); workers != expected {
		t.Fatalf("expected fanout workers %d, got %d", expected, workers)
	}
}

func TestFanoutWorkersDefaultString(t *testing.T) {
	cfg := loadConfigWithFanout(t, "  fanoutWorkers: default\n")
	if workers := cfg.Eventbus.FanoutWorkerCount(); workers != 4 {
		t.Fatalf("expected default fanout workers 4, got %d", workers)
	}
}

func TestFanoutWorkersMissing(t *testing.T) {
	cfg := loadConfigWithFanout(t, "")
	if workers := cfg.Eventbus.FanoutWorkerCount(); workers != 4 {
		t.Fatalf("expected missing fanout workers to default to 4, got %d", workers)
	}
}

func TestDatabaseDefaultsApplied(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	yaml := `
environment: dev
eventbus:
  bufferSize: 16
  fanoutWorkers: 2
pools:
  event:
    size: 32
  orderRequest:
    size: 16
risk:
  maxPositionSize: "1"
  maxNotionalValue: "10"
  notionalCurrency: USD
  orderThrottle: 1
  orderBurst: 1
apiServer:
  addr: ":1234"
telemetry:
  otlpEndpoint: http://localhost:4318
  serviceName: svc
  otlpInsecure: true
  enableMetrics: true
lambdaManifest: {}
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Database.DSN != "postgresql://localhost:5432/meltica" {
		t.Fatalf("expected default DSN, got %q", cfg.Database.DSN)
	}
	if cfg.Database.MaxConns != 16 {
		t.Fatalf("expected default maxConns 16, got %d", cfg.Database.MaxConns)
	}
	if cfg.Database.MinConns != 1 {
		t.Fatalf("expected default minConns 1, got %d", cfg.Database.MinConns)
	}
	if cfg.Database.MaxConnLifetime != 30*time.Minute {
		t.Fatalf("expected default maxConnLifetime 30m, got %s", cfg.Database.MaxConnLifetime)
	}
	if cfg.Database.MaxConnIdleTime != 5*time.Minute {
		t.Fatalf("expected default maxConnIdleTime 5m, got %s", cfg.Database.MaxConnIdleTime)
	}
	if cfg.Database.HealthCheckPeriod != 30*time.Second {
		t.Fatalf("expected default healthCheckPeriod 30s, got %s", cfg.Database.HealthCheckPeriod)
	}
}

func TestLoadAppliesRiskDefaultsWhenMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	yaml := `
environment: dev
eventbus:
  bufferSize: 256
  fanoutWorkers: 4
pools:
  event:
    size: 128
    waitQueueSize: 128
  orderRequest:
    size: 64
    waitQueueSize: 64
apiServer:
  addr: ":8080"
telemetry:
  otlpEndpoint: ""
  serviceName: default-service
  otlpInsecure: false
  enableMetrics: true
`
	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if !reflect.DeepEqual(cfg.Risk, defaultRiskConfig()) {
		t.Fatalf("expected risk config defaults: %#v", cfg.Risk)
	}
}

func TestEventbusExtensionPayloadCapOverrides(t *testing.T) {
	dir := t.TempDir()
	validPath := filepath.Join(dir, "valid.yaml")
	validYAML := `
environment: dev
eventbus:
  bufferSize: 64
  fanoutWorkers: 2
  extensionPayloadCapBytes: 204800
pools:
  event:
    size: 10
  orderRequest:
    size: 5
risk:
  maxPositionSize: "1"
  maxNotionalValue: "10"
  notionalCurrency: USD
  orderThrottle: 1
  orderBurst: 1
apiServer:
  addr: ":8080"
telemetry:
  otlpEndpoint: http://localhost:4318
  serviceName: svc
  otlpInsecure: true
  enableMetrics: true
`
	if err := os.WriteFile(validPath, []byte(validYAML), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(context.Background(), validPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Eventbus.ExtensionPayloadCapBytes != 204800 {
		t.Fatalf("expected override cap 204800, got %d", cfg.Eventbus.ExtensionPayloadCapBytes)
	}

	invalidPath := filepath.Join(dir, "invalid.yaml")
	invalidYAML := `
environment: dev
eventbus:
  bufferSize: 64
  fanoutWorkers: 2
  extensionPayloadCapBytes: -1
pools:
  event:
    size: 10
  orderRequest:
    size: 5
risk:
  maxPositionSize: "1"
  maxNotionalValue: "10"
  notionalCurrency: USD
  orderThrottle: 1
  orderBurst: 1
apiServer:
  addr: ":8080"
telemetry:
  otlpEndpoint: http://localhost:4318
  serviceName: svc
  otlpInsecure: true
  enableMetrics: true
`
	if err := os.WriteFile(invalidPath, []byte(invalidYAML), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	_, err = Load(context.Background(), invalidPath)
	if err == nil {
		t.Fatalf("expected error for negative extension payload cap")
	}
	if !strings.Contains(err.Error(), "extensionPayloadCapBytes") {
		t.Fatalf("expected extension payload cap validation error, got %v", err)
	}
}

func validBaseConfig() AppConfig {
	return AppConfig{
		Environment: EnvDev,
		Providers:   map[Provider]map[string]any{},
		Eventbus: EventbusConfig{
			BufferSize:               128,
			FanoutWorkers:            FanoutWorkerSetting{kind: fanoutWorkerExplicit, value: 4},
			ExtensionPayloadCapBytes: eventbus.DefaultExtensionPayloadCapBytes,
		},
		Pools: PoolConfig{
			Event:        ObjectPoolConfig{Size: 100},
			OrderRequest: ObjectPoolConfig{Size: 50},
		},
		Risk:       defaultRiskConfig(),
		APIServer:  APIServerConfig{Addr: ":9999", AuthToken: "safe-test-token"},
		Telemetry:  TelemetryConfig{ServiceName: "test-service"},
		Strategies: StrategiesConfig{Directory: "strategies"},
		Database: DatabaseConfig{
			DSN:               "postgresql://localhost:5432/meltica",
			MaxConns:          16,
			MinConns:          1,
			MaxConnLifetime:   30 * time.Minute,
			MaxConnIdleTime:   5 * time.Minute,
			HealthCheckPeriod: 30 * time.Second,
		},
	}
}

func loadConfigWithFanout(t *testing.T, fanoutLine string) AppConfig {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	yaml := fmt.Sprintf(`
environment: dev
providers:
  binance-spot:
    adapter:
      identifier: binance
      config:
        option: value
eventbus:
  bufferSize: 128
%spools:
  event:
    size: 100
  orderRequest:
    size: 50
risk:
  maxPositionSize: "10"
  maxNotionalValue: "1000"
  notionalCurrency: "USD"
  orderThrottle: 5
  orderBurst: 1

apiServer:
  addr: ":9999"
telemetry:
  otlpEndpoint: http://localhost:4318
  serviceName: test-service
  otlpInsecure: true
  enableMetrics: false
lambdaManifest:
  lambdas:
    - id: test-lambda
      scope:
        binance-spot:
          symbols:
            - BTC-USDT
      strategy:
        identifier: delay
`, fanoutLine)

	if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := Load(context.Background(), path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	return cfg
}

func TestGatewayRejectsInvalidRiskConfigFromLoad(t *testing.T) {
	tests := []struct {
		name             string
		orderBurst       int
		maxRiskBreaches  int
		breakerThreshold int
		wantErr          string
	}{
		{
			name:             "non-positive order burst",
			orderBurst:       0,
			maxRiskBreaches:  0,
			breakerThreshold: 0,
			wantErr:          "risk orderBurst",
		},
		{
			name:             "negative risk breaches",
			orderBurst:       1,
			maxRiskBreaches:  -1,
			breakerThreshold: 0,
			wantErr:          "risk maxRiskBreaches",
		},
		{
			name:             "negative circuit breaker threshold",
			orderBurst:       1,
			maxRiskBreaches:  0,
			breakerThreshold: -1,
			wantErr:          "risk circuitBreaker threshold",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "app.yaml")
			yaml := fmt.Sprintf(`environment: dev
eventbus:
  bufferSize: 16
  fanoutWorkers: 1
pools:
  event:
    size: 1
  orderRequest:
    size: 1
risk:
  maxPositionSize: "10"
  maxNotionalValue: "1000"
  notionalCurrency: USD
  orderThrottle: 5
  orderBurst: %d
  maxRiskBreaches: %d
  circuitBreaker:
    enabled: false
    threshold: %d
apiServer:
  addr: ":0"
telemetry:
  serviceName: test-service
`, tt.orderBurst, tt.maxRiskBreaches, tt.breakerThreshold)
			if err := os.WriteFile(path, []byte(yaml), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := Load(context.Background(), path)
			if err == nil {
				t.Fatal("expected Load to reject invalid risk config")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
