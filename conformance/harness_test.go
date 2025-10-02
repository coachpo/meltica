package conformance_test

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/conformance"
	"github.com/coachpo/meltica/core"
)

func TestRunAll_NoProviders(t *testing.T) {
	if err := conformance.RunAll(context.Background(), nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderCapabilityMismatch(t *testing.T) {
	factory := func(context.Context) (core.Provider, error) {
		return fakeProvider{caps: core.Capabilities(core.CapabilitySpotPublicREST)}, nil
	}
	err := conformance.RunAll(context.Background(), []conformance.ProviderCase{{
		Name:         "fake",
		Capabilities: []core.Capability{core.CapabilitySpotPublicREST, core.CapabilityWebsocketPublic},
		Factory:      factory,
	}})
	if err == nil {
		t.Fatal("expected mismatch error")
	}
}

type fakeProvider struct {
	caps core.ProviderCapabilities
}

func (f fakeProvider) Name() string                                       { return "fake" }
func (f fakeProvider) Capabilities() core.ProviderCapabilities            { return f.caps }
func (f fakeProvider) SupportedProtocolVersion() string                   { return core.ProtocolVersion }
func (f fakeProvider) Spot(ctx context.Context) core.SpotAPI              { return nil }
func (f fakeProvider) LinearFutures(ctx context.Context) core.FuturesAPI  { return nil }
func (f fakeProvider) InverseFutures(ctx context.Context) core.FuturesAPI { return nil }
func (f fakeProvider) WS() core.WS                                        { return nil }
func (f fakeProvider) Close() error                                       { return nil }
