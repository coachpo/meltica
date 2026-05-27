package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/coachpo/meltica/internal/infra/config"
	"github.com/sourcegraph/conc"
)

func TestResolveConfigPathPrefersFlag(t *testing.T) {
	t.Setenv(configPathEnvVar, "config/app.ci.yaml")
	got := resolveConfigPath("custom/config.yaml")
	want := filepath.Clean("custom/config.yaml")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolveConfigPathFallsBackToEnv(t *testing.T) {
	t.Setenv(configPathEnvVar, "config/app.ci.yaml")
	got := resolveConfigPath("")
	want := filepath.Clean("config/app.ci.yaml")
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestResolveConfigPathDefaults(t *testing.T) {
	t.Setenv(configPathEnvVar, "")
	got := resolveConfigPath("")
	want := filepath.Clean(defaultConfigPath)
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestGatewayBootstrapReturnsStartupFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startupErr := errors.New("startup failed")
	started := false
	shutdown := false

	err := runGateway(ctx, cancel, discardGatewayLogger(), "config/app.yaml", gatewayRuntime{
		composeGateway: func(context.Context, *log.Logger, string, context.CancelFunc) (gatewayComposition, error) {
			return gatewayComposition{}, startupErr
		},
		startAPIServer: func(*conc.WaitGroup, *log.Logger, apiServerStarter) {
			started = true
		},
		performGracefulShutdown: func(context.Context, *log.Logger, gracefulShutdownConfig) {
			shutdown = true
		},
	})

	if !errors.Is(err, startupErr) {
		t.Fatalf("expected startup error, got %v", err)
	}
	if started {
		t.Fatal("API server should not start after composition failure")
	}
	if shutdown {
		t.Fatal("shutdown should not run after composition failure")
	}
}

func TestGatewayBootstrapRunsCompositionStartAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	recorder := newCallRecorder()
	lifecycle := &conc.WaitGroup{}
	server := &http.Server{Addr: "127.0.0.1:0"}

	err := runGateway(ctx, cancel, discardGatewayLogger(), "config/app.yaml", gatewayRuntime{
		composeGateway: func(_ context.Context, _ *log.Logger, cfgPath string, mainCancel context.CancelFunc) (gatewayComposition, error) {
			if cfgPath != "config/app.yaml" {
				t.Fatalf("expected config path to be passed through, got %q", cfgPath)
			}
			if mainCancel == nil {
				t.Fatal("expected main cancel to be passed into composition")
			}
			recorder.record("compose")
			return gatewayComposition{
				apiServer: server,
				lifecycle: lifecycle,
				shutdown:  gracefulShutdownConfig{lifecycle: lifecycle, mainCancel: mainCancel},
			}, nil
		},
		startAPIServer: func(gotLifecycle *conc.WaitGroup, _ *log.Logger, gotServer apiServerStarter) {
			if gotLifecycle != lifecycle {
				t.Fatal("expected composed lifecycle to be used for API startup")
			}
			if gotServer != server {
				t.Fatal("expected composed API server to be started")
			}
			recorder.record("start")
			cancel()
		},
		performGracefulShutdown: func(_ context.Context, _ *log.Logger, cfg gracefulShutdownConfig) {
			if cfg.mainCancel == nil {
				t.Fatal("expected shutdown config to retain main cancel")
			}
			recorder.record("shutdown")
		},
		newShutdownContext: func() (context.Context, context.CancelFunc) {
			return context.Background(), func() {}
		},
	})

	if err != nil {
		t.Fatalf("expected gateway run to complete, got %v", err)
	}
	recorder.requireOrder(t, []string{"compose", "start", "shutdown"})
}

func TestGatewayBootstrapStartAPIServerIgnoresErrServerClosed(t *testing.T) {
	var logs strings.Builder
	lifecycle := &conc.WaitGroup{}
	server := &fakeAPIStarter{
		err:    http.ErrServerClosed,
		called: make(chan struct{}),
	}

	startAPIServer(lifecycle, log.New(&logs, "", 0), server)
	lifecycle.Wait()

	if !server.wasCalled() {
		t.Fatal("expected API server starter to be called")
	}
	if strings.Contains(logs.String(), "control server") {
		t.Fatalf("expected ErrServerClosed to be silent, got logs %q", logs.String())
	}
}

func TestGatewayBootstrapBuildsAPIServerFromConfig(t *testing.T) {
	server := buildAPIServer(config.AppConfig{
		APIServer: config.APIServerConfig{Addr: "127.0.0.1:9999"},
	}, nil, nil, nil)

	if server.Addr != "127.0.0.1:9999" {
		t.Fatalf("expected configured address, got %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("expected API server handler to be configured")
	}
	if server.ReadHeaderTimeout != controlReadHeaderTimeout {
		t.Fatalf("expected read header timeout %v, got %v", controlReadHeaderTimeout, server.ReadHeaderTimeout)
	}
}

func TestPerformGracefulShutdownPreservesOrder(t *testing.T) {
	recorder := newCallRecorder()

	performGracefulShutdown(context.Background(), discardGatewayLogger(), gracefulShutdownConfig{
		server:     recordingGracefulServer{recorder: recorder, name: "server"},
		mainCancel: func() { recorder.record("cancel") },
		lifecycle:  recordingLifecycle{recorder: recorder, name: "lifecycle"},
		dataBus:    recordingDataBus{recorder: recorder, name: "bus"},
		poolMgr:    recordingPoolManager{recorder: recorder, name: "pool"},
		telemetry:  recordingTelemetry{recorder: recorder, name: "telemetry"},
		dbPool:     recordingDatabase{recorder: recorder, name: "db"},
	})

	recorder.requireOrder(t, []string{"server", "cancel", "lifecycle", "bus", "pool", "telemetry", "db"})
}

func discardGatewayLogger() *log.Logger {
	return log.New(io.Discard, "", 0)
}

type fakeAPIStarter struct {
	err    error
	called chan struct{}
}

func (s *fakeAPIStarter) ListenAndServe() error {
	close(s.called)
	return s.err
}

func (s *fakeAPIStarter) wasCalled() bool {
	select {
	case <-s.called:
		return true
	default:
		return false
	}
}

type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func newCallRecorder() *callRecorder {
	return &callRecorder{}
}

func (r *callRecorder) record(call string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, call)
}

func (r *callRecorder) entries() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.calls...)
}

func (r *callRecorder) requireOrder(t *testing.T, want []string) {
	t.Helper()
	got := r.entries()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected order %v, got %v", want, got)
	}
}

type recordingGracefulServer struct {
	recorder *callRecorder
	name     string
}

func (s recordingGracefulServer) Shutdown(context.Context) error {
	s.recorder.record(s.name)
	return nil
}

type recordingLifecycle struct {
	recorder *callRecorder
	name     string
}

func (l recordingLifecycle) Wait() {
	l.recorder.record(l.name)
}

type recordingDataBus struct {
	recorder *callRecorder
	name     string
}

func (b recordingDataBus) Close() {
	b.recorder.record(b.name)
}

type recordingPoolManager struct {
	recorder *callRecorder
	name     string
}

func (m recordingPoolManager) Shutdown(context.Context) error {
	m.recorder.record(m.name)
	return nil
}

type recordingTelemetry struct {
	recorder *callRecorder
	name     string
}

func (p recordingTelemetry) Shutdown(context.Context) error {
	p.recorder.record(p.name)
	return nil
}

type recordingDatabase struct {
	recorder *callRecorder
	name     string
}

func (d recordingDatabase) Close() {
	d.recorder.record(d.name)
}

func TestGatewayRejectsInvalidRiskConfig(t *testing.T) {
	tests := []struct {
		name             string
		maxPositionSize  string
		orderBurst       int
		maxRiskBreaches  int
		breakerThreshold int
		wantErr          string
	}{
		{name: "invalid decimal", maxPositionSize: "not-a-decimal", orderBurst: 1, wantErr: "maxPositionSize"},
		{name: "non-positive order burst", maxPositionSize: "10", orderBurst: 0, wantErr: "risk orderBurst"},
		{name: "negative risk breaches", maxPositionSize: "10", orderBurst: 1, maxRiskBreaches: -1, wantErr: "risk maxRiskBreaches"},
		{name: "negative breaker threshold", maxPositionSize: "10", orderBurst: 1, breakerThreshold: -1, wantErr: "risk circuitBreaker threshold"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			strategyDir := filepath.Join(dir, "strategies")
			if err := os.Mkdir(strategyDir, 0o755); err != nil {
				t.Fatalf("create strategy dir: %v", err)
			}
			cfgPath := filepath.Join(dir, "app.yaml")
			contents := fmt.Sprintf(`environment: dev
eventbus:
  bufferSize: 1
  fanoutWorkers: 1
pools:
  event:
    size: 1
    waitQueueSize: 1
  orderRequest:
    size: 1
    waitQueueSize: 1
risk:
  maxPositionSize: %q
  maxNotionalValue: "1000"
  notionalCurrency: USD
  orderThrottle: 5
  orderBurst: %d
  maxConcurrentOrders: 0
  priceBandPercent: 1
  maxRiskBreaches: %d
  circuitBreaker:
    enabled: false
    threshold: %d
apiServer:
  addr: "127.0.0.1:0"
telemetry:
  serviceName: test-gateway
strategies:
  directory: %q
database:
  runMigrations: false
`, tt.maxPositionSize, tt.orderBurst, tt.maxRiskBreaches, tt.breakerThreshold, strategyDir)
			if err := os.WriteFile(cfgPath, []byte(contents), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			_, err := composeGateway(context.Background(), discardGatewayLogger(), cfgPath, func() {})
			if err == nil {
				t.Fatal("expected gateway composition to reject invalid risk config")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}
