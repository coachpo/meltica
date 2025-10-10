package plugin

import (
	"context"
	"sync"
	"testing"

	"github.com/coachpo/meltica/core"
	corestreams "github.com/coachpo/meltica/core/streams"
	sharedplugin "github.com/coachpo/meltica/exchanges/shared/plugin"
	sharedrouting "github.com/coachpo/meltica/exchanges/shared/routing"
)

func TestRegisterRegistersSummaryOnce(t *testing.T) {
	sharedplugin.Reset()
	registerOnce = sync.Once{}

	Register()
	summaries := sharedplugin.Summaries()
	if len(summaries) != 1 {
		t.Fatalf("expected a single registration summary, got %d", len(summaries))
	}
	summary := summaries[0]
	if summary.Name != Name {
		t.Fatalf("expected summary name %q, got %q", Name, summary.Name)
	}
	if summary.ProtocolVersion != core.ProtocolVersion {
		t.Fatalf("expected protocol version %s, got %s", core.ProtocolVersion, summary.ProtocolVersion)
	}

	Register() // should be a no-op because of sync.Once
	if len(sharedplugin.Summaries()) != 1 {
		t.Fatalf("expected registration to remain idempotent")
	}
}

func TestNewFilterAdapterValidation(t *testing.T) {
	if _, _, err := NewFilterAdapter(nil); err == nil {
		t.Fatalf("expected error when exchange is nil")
	}

	wrong := &baseExchange{name: "other"}
	if _, _, err := NewFilterAdapter(wrong); err == nil {
		t.Fatalf("expected error for unsupported exchange")
	}

	empty := &baseExchange{name: string(Name)}
	if _, _, err := NewFilterAdapter(empty); err == nil {
		t.Fatalf("expected error when no feeds are available")
	}
}

func TestNewFilterAdapterWithFeedsAndAuth(t *testing.T) {
	fx := &fullExchange{
		baseExchange: baseExchange{name: string(Name)},
		ws:           stubWS{},
		private:      stubWS{},
		rest:         stubRESTDispatcher{},
		apiKey:       "api-key",
		secret:       "secret",
	}

	adapter, auth, err := NewFilterAdapter(fx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if adapter == nil {
		t.Fatalf("expected adapter instance")
	}
	if auth == nil {
		t.Fatalf("expected auth context from credentials")
	}
	if auth.APIKey != "api-key" || auth.Secret != "secret" {
		t.Fatalf("unexpected auth context: %+v", auth)
	}

	caps := adapter.Capabilities()
	if !caps.Books || !caps.Trades || !caps.RESTEndpoints {
		t.Fatalf("expected capabilities to include books, trades, and rest: %+v", caps)
	}
}

type baseExchange struct {
	name string
}

func (b *baseExchange) Name() string { return b.name }
func (b *baseExchange) Capabilities() core.ExchangeCapabilities {
	return core.Capabilities()
}
func (b *baseExchange) SupportedProtocolVersion() string { return core.ProtocolVersion }
func (b *baseExchange) Close() error                     { return nil }

type fullExchange struct {
	baseExchange
	ws      core.WS
	private core.WS
	rest    sharedrouting.RESTDispatcher
	apiKey  string
	secret  string
}

func (f *fullExchange) BookSnapshots(ctx context.Context, symbol string) (<-chan corestreams.BookEvent, <-chan error, error) {
	books := make(chan corestreams.BookEvent)
	errs := make(chan error)
	close(books)
	close(errs)
	return books, errs, nil
}

func (f *fullExchange) WS() core.WS { return f.ws }

func (f *fullExchange) PrivateWS() core.WS { return f.private }

func (f *fullExchange) RESTRouter() interface{} { return f.rest }

func (f *fullExchange) Credentials() (string, string) { return f.apiKey, f.secret }

type stubWS struct{}

func (stubWS) SubscribePublic(ctx context.Context, topics ...string) (core.Subscription, error) {
	return stubSubscription{}, nil
}

func (stubWS) SubscribePrivate(ctx context.Context, topics ...string) (core.Subscription, error) {
	return stubSubscription{}, nil
}

type stubSubscription struct{}

func (stubSubscription) C() <-chan core.Message {
	ch := make(chan core.Message)
	close(ch)
	return ch
}

func (stubSubscription) Err() <-chan error {
	ch := make(chan error)
	close(ch)
	return ch
}

func (stubSubscription) Close() error { return nil }

type stubRESTDispatcher struct{}

func (stubRESTDispatcher) Dispatch(ctx context.Context, msg sharedrouting.RESTMessage, out any) error {
	return nil
}
