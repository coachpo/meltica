package plugin

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/core"
	coreregistry "github.com/coachpo/meltica/core/registry"
)

type stubExchange struct {
	name string
	caps core.ExchangeCapabilities
}

func (s *stubExchange) Name() string                            { return s.name }
func (s *stubExchange) Capabilities() core.ExchangeCapabilities { return s.caps }
func (s *stubExchange) SupportedProtocolVersion() string        { return core.ProtocolVersion }
func (s *stubExchange) Close() error                            { return nil }

func (s *stubExchange) Spot(context.Context) core.SpotAPI { return nil }

type stubSymbolTranslator struct{}

func (stubSymbolTranslator) Native(string) (string, error)    { return "NATIVE", nil }
func (stubSymbolTranslator) Canonical(string) (string, error) { return "CANONICAL", nil }

type stubTopicTranslator struct{}

func (stubTopicTranslator) Native(core.Topic) (string, error)    { return "topic", nil }
func (stubTopicTranslator) Canonical(string) (core.Topic, error) { return core.TopicTrade, nil }

func TestRegisterWiresFactoriesAndTranslators(t *testing.T) {
	Reset()
	name := core.ExchangeName("shared-plugin-demo")
	caps := core.Capabilities(core.CapabilitySpotPublicREST)
	Register(Registration{
		Name: name,
		Build: func(cfg coreregistry.Config) (core.Exchange, error) {
			return &stubExchange{name: string(name), caps: caps}, nil
		},
		SymbolTranslator: func(core.Exchange) (core.SymbolTranslator, error) {
			return stubSymbolTranslator{}, nil
		},
		TopicTranslator: func(core.Exchange) (core.TopicTranslator, error) {
			return stubTopicTranslator{}, nil
		},
		Capabilities:    caps,
		ProtocolVersion: "1.2.0",
		Metadata:        map[string]string{"region": "global"},
	})

	exchange, err := coreregistry.Resolve(name)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if exchange.Name() != string(name) {
		t.Fatalf("expected exchange name %s, got %s", name, exchange.Name())
	}
	if exchange.Capabilities() != caps {
		t.Fatalf("capabilities mismatch: %v", exchange.Capabilities())
	}

	native, err := core.NativeSymbol(name, "BTC-USDT")
	if err != nil {
		t.Fatalf("NativeSymbol returned error: %v", err)
	}
	if native != "NATIVE" {
		t.Fatalf("expected native NATIVE, got %s", native)
	}
	translator, err := core.TopicTranslatorFor(name)
	if err != nil {
		t.Fatalf("TopicTranslatorFor error: %v", err)
	}
	if val, err := translator.Native(core.TopicTrade); err != nil || val != "topic" {
		t.Fatalf("expected topic translator to return 'topic', got %q err=%v", val, err)
	}

	entries := Summaries()
	if len(entries) != 1 {
		t.Fatalf("expected single summary entry, got %d", len(entries))
	}
	if entries[0].Name != name {
		t.Fatalf("summary name mismatch: %s", entries[0].Name)
	}
	if entries[0].Capabilities != caps {
		t.Fatalf("summary capabilities mismatch: %v", entries[0].Capabilities)
	}
	if entries[0].ProtocolVersion != "1.2.0" {
		t.Fatalf("summary protocol mismatch: %s", entries[0].ProtocolVersion)
	}
	if entries[0].Metadata["region"] != "global" {
		t.Fatalf("expected metadata to include region global, got %v", entries[0].Metadata)
	}
}

func TestBuilderRegistersAdapter(t *testing.T) {
	Reset()
	name := core.ExchangeName("builder-adapter")
	caps := core.Capabilities(core.CapabilitySpotPublicREST, core.CapabilityMarketTrades)
	b := NewBuilder(name, func(cfg coreregistry.Config) (core.Exchange, error) {
		return &stubExchange{name: string(name), caps: caps}, nil
	}).
		WithCapabilities(core.CapabilitySpotPublicREST, core.CapabilityMarketTrades).
		WithSymbolTranslator(func(core.Exchange) (core.SymbolTranslator, error) {
			return stubSymbolTranslator{}, nil
		}).
		WithTopicTranslator(func(core.Exchange) (core.TopicTranslator, error) {
			return stubTopicTranslator{}, nil
		}).
		WithMetadata("region", "emea")

	b.Register()

	exchange, err := coreregistry.Resolve(name)
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if exchange.Capabilities() != caps {
		t.Fatalf("capabilities mismatch: %v", exchange.Capabilities())
	}
	if entries := Summaries(); len(entries) != 1 || entries[0].Metadata["region"] != "emea" {
		t.Fatalf("summary metadata mismatch: %v", entries)
	}
}
