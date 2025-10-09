package infra_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/shared/infra"
	"github.com/coachpo/meltica/exchanges/shared/infra/transport"
)

type stubSymbolTranslator struct {
	canonicalToNative map[string]string
	nativeToCanonical map[string]string
}

func (s *stubSymbolTranslator) Native(canonical string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(canonical))
	if native, ok := s.canonicalToNative[key]; ok {
		return native, nil
	}
	return "", errors.New("unknown canonical symbol")
}

func (s *stubSymbolTranslator) Canonical(native string) (string, error) {
	key := strings.ToUpper(strings.TrimSpace(native))
	if canonical, ok := s.nativeToCanonical[key]; ok {
		return canonical, nil
	}
	return "", errors.New("unknown native symbol")
}

type stubTopicTranslator struct {
	forward map[core.Topic]string
	reverse map[string]core.Topic
}

func (s *stubTopicTranslator) Native(topic core.Topic) (string, error) {
	if value, ok := s.forward[topic]; ok {
		return value, nil
	}
	return "", core.ErrNotSupported
}

func (s *stubTopicTranslator) Canonical(native string) (core.Topic, error) {
	if topic, ok := s.reverse[native]; ok {
		return topic, nil
	}
	return "", core.ErrNotSupported
}

type stubSpotAPI struct{}

func (stubSpotAPI) ServerTime(context.Context) (time.Time, error)               { return time.Unix(0, 0), nil }
func (stubSpotAPI) Instruments(context.Context) ([]core.Instrument, error)      { return nil, nil }
func (stubSpotAPI) Ticker(context.Context, string) (core.Ticker, error)         { return core.Ticker{}, nil }
func (stubSpotAPI) Balances(context.Context) ([]core.Balance, error)            { return nil, nil }
func (stubSpotAPI) Trades(context.Context, string, int64) ([]core.Trade, error) { return nil, nil }
func (stubSpotAPI) PlaceOrder(context.Context, core.OrderRequest) (core.Order, error) {
	return core.Order{ID: "stub"}, nil
}
func (stubSpotAPI) GetOrder(context.Context, string, string, string) (core.Order, error) {
	return core.Order{ID: "stub"}, nil
}
func (stubSpotAPI) CancelOrder(context.Context, string, string, string) error { return nil }

func TestAdapterFactory_ComposeExchangeAndClients(t *testing.T) {
	name := core.ExchangeName("demo-unified")
	factory := infra.NewAdapterFactory(name)

	symbolTranslator := &stubSymbolTranslator{
		canonicalToNative: map[string]string{"BTC-USDT": "BTCUSDT"},
		nativeToCanonical: map[string]string{"BTCUSDT": "BTC-USDT"},
	}
	factory.RegisterSymbolTranslator(symbolTranslator)

	topicTranslator := &stubTopicTranslator{
		forward: map[core.Topic]string{
			core.TopicTrade: "trade",
		},
		reverse: map[string]core.Topic{"trade": core.TopicTrade},
	}
	factory.RegisterTopicTranslator(func() (core.TopicTranslator, error) { return topicTranslator, nil })

	closeCalled := false
	exchange, err := factory.BuildExchange(infra.ExchangeConfig{
		Capabilities: core.Capabilities(
			core.CapabilitySpotPublicREST,
			core.CapabilityMarketTrades,
		),
		Version: core.ProtocolVersion,
		Spot:    stubSpotAPI{},
		OnClose: func() error {
			closeCalled = true
			return nil
		},
	})
	if err != nil {
		t.Fatalf("BuildExchange returned error: %v", err)
	}

	if got := exchange.Name(); got != string(name) {
		t.Fatalf("expected exchange name %q, got %q", name, got)
	}

	if caps := exchange.Capabilities(); !caps.Has(core.CapabilityMarketTrades) {
		t.Fatalf("expected capabilities to include market trades, got %v", caps)
	}

	spotParticipant, ok := exchange.(core.SpotParticipant)
	if !ok {
		t.Fatalf("expected composite exchange to implement SpotParticipant")
	}
	if _, err := spotParticipant.Spot(context.Background()).PlaceOrder(context.Background(), core.OrderRequest{Symbol: "BTC-USDT"}); err != nil {
		t.Fatalf("Spot API place order failed: %v", err)
	}

	if err := exchange.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	if !closeCalled {
		t.Fatal("expected OnClose hook to be invoked")
	}

	if native, err := core.NativeSymbol(name, "BTC-USDT"); err != nil || native != "BTCUSDT" {
		t.Fatalf("expected registered symbol translator to resolve native symbol, got %q err=%v", native, err)
	}

	if topicTranslator, err := core.TopicTranslatorFor(name); err != nil {
		t.Fatalf("TopicTranslatorFor error: %v", err)
	} else {
		if native, err := topicTranslator.Native(core.TopicTrade); err != nil || native != "trade" {
			t.Fatalf("topic translator mismatch: native=%q err=%v", native, err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"code":"429","message":"slow down"}`))
	}))
	t.Cleanup(server.Close)

	client, err := factory.NewRESTClient(infra.RESTClientConfig{
		BaseURL:        server.URL,
		Retry:          transport.RetryPolicy{MaxRetries: 0},
		CanonicalError: errs.CanonicalRateLimited,
	})
	if err != nil {
		t.Fatalf("NewRESTClient returned error: %v", err)
	}

	_, _, err = client.DoWithHeaders(context.Background(), http.MethodGet, "/limits", nil, nil, false, nil, nil)
	if err == nil {
		t.Fatal("expected error but got nil")
	}
	var envelope *errs.E
	if !errors.As(err, &envelope) {
		t.Fatalf("expected errs.E, got %T", err)
	}
	if envelope.Exchange != string(name) {
		t.Fatalf("expected envelope exchange %q, got %q", name, envelope.Exchange)
	}
	if envelope.Canonical != errs.CanonicalRateLimited {
		t.Fatalf("expected canonical rate limited, got %s", envelope.Canonical)
	}
	if envelope.HTTP != http.StatusTooManyRequests {
		t.Fatalf("expected http status 429, got %d", envelope.HTTP)
	}
}

// Order router verification is exercised by the shared routing package tests.
