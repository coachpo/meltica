package registry

import (
	"errors"
	"testing"

	"github.com/coachpo/meltica/core"
)

type stubExchange struct{}

func (stubExchange) Name() string { return "stub" }

func (stubExchange) Capabilities() core.ExchangeCapabilities { return core.Capabilities() }

func (stubExchange) SupportedProtocolVersion() string { return core.ProtocolVersion }

func (stubExchange) Close() error { return nil }

func withFactorySnapshot(t *testing.T, fn func()) {
	t.Helper()
	factoryMu.Lock()
	original := make(map[ExchangeName]ExchangeFactory, len(factories))
	for k, v := range factories {
		original[k] = v
	}
	factories = make(map[ExchangeName]ExchangeFactory)
	factoryMu.Unlock()

	t.Cleanup(func() {
		factoryMu.Lock()
		factories = make(map[ExchangeName]ExchangeFactory, len(original))
		for k, v := range original {
			factories[k] = v
		}
		factoryMu.Unlock()
	})

	fn()
}

func TestRegisterAndResolveAppliesOptions(t *testing.T) {
	withFactorySnapshot(t, func() {
		var captured Config
		Register("demo", func(cfg Config) (core.Exchange, error) {
			captured = cfg
			return stubExchange{}, nil
		})

		exch, err := Resolve("demo",
			WithAPIKey("key"),
			WithAPISecret("secret"),
			WithParam("foo", 123),
			WithParams(map[string]any{"bar": "baz"}),
		)
		if err != nil {
			t.Fatalf("resolve returned error: %v", err)
		}
		if exch == nil {
			t.Fatal("expected exchange instance")
		}
		if captured.APIKey != "key" || captured.APISecret != "secret" {
			t.Fatalf("unexpected credentials: %+v", captured)
		}
		if got := captured.Params["foo"]; got != 123 {
			t.Fatalf("expected foo param = 123, got %v", got)
		}
		if got := captured.Params["bar"]; got != "baz" {
			t.Fatalf("expected bar param = baz, got %v", got)
		}
	})
}

func TestResolveUnknownExchangeReturnsError(t *testing.T) {
	withFactorySnapshot(t, func() {
		_, err := Resolve("missing")
		if err == nil {
			t.Fatal("expected error for unknown exchange")
		}
	})
}

func TestWithParamsHandlesNilAndEmpty(t *testing.T) {
	withFactorySnapshot(t, func() {
		Register("noop", func(cfg Config) (core.Exchange, error) {
			if cfg.Params == nil {
				return nil, errors.New("params not initialized")
			}
			if len(cfg.Params) != 0 {
				return nil, errors.New("unexpected params")
			}
			return stubExchange{}, nil
		})

		if _, err := Resolve("noop", WithParams(nil)); err != nil {
			t.Fatalf("resolve failed: %v", err)
		}
	})
}
