package binance

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/config"
	corestreams "github.com/coachpo/meltica/core/streams"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type resourceTracker struct {
	restOpens    int
	restCloses   int
	streamOpens  int
	streamCloses int
	routerOpens  int
	routerCloses int
}

func (rt *resourceTracker) newRESTClient(rest.Config) coretransport.RESTClient {
	rt.restOpens++
	return &mockRESTClient{tracker: rt}
}

func (rt *resourceTracker) newStreamClient(ws.Config) coretransport.StreamClient {
	rt.streamOpens++
	return &mockStreamClient{tracker: rt}
}

func (rt *resourceTracker) newRouter(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter {
	rt.routerOpens++
	return &mockWSRouter{tracker: rt}
}

type mockRESTClient struct {
	tracker *resourceTracker
}

func (c *mockRESTClient) Connect(context.Context) error { return nil }
func (c *mockRESTClient) Close() error {
	c.tracker.restCloses++
	return nil
}
func (c *mockRESTClient) DoRequest(ctx context.Context, req coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
	return nil, nil
}
func (c *mockRESTClient) HandleResponse(ctx context.Context, req coretransport.RESTRequest, resp *coretransport.RESTResponse, out any) error {
	return nil
}
func (c *mockRESTClient) HandleError(ctx context.Context, req coretransport.RESTRequest, err error) error {
	return err
}

type mockStreamClient struct {
	tracker *resourceTracker
}

func (c *mockStreamClient) Connect(context.Context) error { return nil }
func (c *mockStreamClient) Close() error {
	c.tracker.streamCloses++
	return nil
}
func (c *mockStreamClient) Subscribe(context.Context, ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
	return nil, errs.NotSupported("not implemented")
}
func (c *mockStreamClient) Unsubscribe(context.Context, coretransport.StreamSubscription, ...coretransport.StreamTopic) error {
	return errs.NotSupported("not implemented")
}
func (c *mockStreamClient) Publish(context.Context, coretransport.StreamMessage) error {
	return errs.NotSupported("not implemented")
}
func (c *mockStreamClient) HandleError(context.Context, error) error { return nil }

type mockWSRouter struct {
	tracker *resourceTracker
}

func (r *mockWSRouter) SubscribePublic(context.Context, ...string) (bnrouting.Subscription, error) {
	return nil, errs.NotSupported("not implemented")
}

func (r *mockWSRouter) SubscribePrivate(context.Context) (bnrouting.Subscription, error) {
	return nil, errs.NotSupported("not implemented")
}

func (r *mockWSRouter) Close() error {
	r.tracker.routerCloses++
	return nil
}

func (r *mockWSRouter) WSNativeSymbol(string) (string, error) { return "", nil }

func (r *mockWSRouter) WSCanonicalSymbol(string) (string, error) { return "", nil }

func (r *mockWSRouter) OrderBookSnapshot(string) (corestreams.BookEvent, bool) {
	return corestreams.BookEvent{}, false
}

func (r *mockWSRouter) InitializeOrderBook(context.Context, string) error { return nil }

func TestUpdateConfigClosesPreviousClients(t *testing.T) {
	tracker := &resourceTracker{}

	ex, err := New("", "",
		WithRESTClientFactory(tracker.newRESTClient),
		WithRESTRouterFactory(func(coretransport.RESTClient) routingrest.RESTDispatcher { return noopDispatcher{} }),
		WithWSClientFactory(tracker.newStreamClient),
		WithWSRouterFactory(tracker.newRouter),
	)
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}

	if tracker.restOpens != 1 || tracker.routerOpens != 1 || tracker.streamOpens != 1 {
		t.Fatalf("unexpected opens: rest=%d router=%d stream=%d", tracker.restOpens, tracker.routerOpens, tracker.streamOpens)
	}

	if err := ex.UpdateConfig(config.WithBinanceAPI("key", "secret")); err != nil {
		t.Fatalf("UpdateConfig returned error: %v", err)
	}

	if tracker.restOpens != 2 {
		t.Fatalf("expected rest opens=2, got %d", tracker.restOpens)
	}
	if tracker.routerOpens != 2 {
		t.Fatalf("expected router opens=2, got %d", tracker.routerOpens)
	}
	if tracker.streamOpens != 2 {
		t.Fatalf("expected stream opens=2, got %d", tracker.streamOpens)
	}

	if tracker.restCloses != 1 {
		t.Fatalf("expected rest closes=1, got %d", tracker.restCloses)
	}
	if tracker.routerCloses != 1 {
		t.Fatalf("expected router closes=1, got %d", tracker.routerCloses)
	}
	if tracker.streamCloses != 1 {
		t.Fatalf("expected stream closes=1, got %d", tracker.streamCloses)
	}

	if err := ex.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if tracker.restCloses != 2 {
		t.Fatalf("expected rest closes=2, got %d", tracker.restCloses)
	}
	if tracker.routerCloses != 2 {
		t.Fatalf("expected router closes=2, got %d", tracker.routerCloses)
	}
	if tracker.streamCloses != 2 {
		t.Fatalf("expected stream closes=2, got %d", tracker.streamCloses)
	}
}

type noopDispatcher struct{}

func (noopDispatcher) Dispatch(context.Context, routingrest.RESTMessage, any) error { return nil }
