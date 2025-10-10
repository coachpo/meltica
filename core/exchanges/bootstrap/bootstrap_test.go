package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/coachpo/meltica/config"
	coretransport "github.com/coachpo/meltica/core/transport"
)

type stubRESTClient struct {
	closeErr error
	closed   bool
}

func (s *stubRESTClient) Connect(context.Context) error { return nil }

func (s *stubRESTClient) Close() error {
	s.closed = true
	return s.closeErr
}

func (s *stubRESTClient) DoRequest(context.Context, coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
	return nil, nil
}

func (s *stubRESTClient) HandleResponse(context.Context, coretransport.RESTRequest, *coretransport.RESTResponse, any) error {
	return nil
}

func (s *stubRESTClient) HandleError(context.Context, coretransport.RESTRequest, error) error {
	return nil
}

type stubStreamClient struct {
	closeErr error
	closed   bool
}

func (s *stubStreamClient) Connect(context.Context) error { return nil }

func (s *stubStreamClient) Close() error {
	s.closed = true
	return s.closeErr
}

func (s *stubStreamClient) Subscribe(context.Context, ...coretransport.StreamTopic) (coretransport.StreamSubscription, error) {
	return nil, nil
}

func (s *stubStreamClient) Unsubscribe(context.Context, coretransport.StreamSubscription, ...coretransport.StreamTopic) error {
	return nil
}

func (s *stubStreamClient) Publish(context.Context, coretransport.StreamMessage) error { return nil }

func (s *stubStreamClient) HandleError(context.Context, error) error { return nil }

type closable struct {
	closed bool
	err    error
}

func (c *closable) Close() error {
	c.closed = true
	return c.err
}

func TestBuildTransportBundleCreatesClients(t *testing.T) {
	rest := &stubRESTClient{}
	stream := &stubStreamClient{}
	bundle := BuildTransportBundle(
		TransportFactories{
			NewRESTClient: func(cfg TransportConfig) coretransport.RESTClient {
				if cfg.APIKey != "key" {
					t.Fatalf("unexpected transport config: %+v", cfg)
				}
				return rest
			},
			NewWSClient: func(cfg TransportConfig) coretransport.StreamClient {
				if cfg.APIKey != "key" {
					t.Fatalf("unexpected stream config: %+v", cfg)
				}
				return stream
			},
		},
		RouterFactories{
			NewRESTRouter: func(client coretransport.RESTClient) interface{} {
				if client != rest {
					t.Fatalf("expected rest client to be passed to router factory")
				}
				return "router"
			},
		},
		TransportConfig{APIKey: "key"},
	)

	if bundle.REST() != rest {
		t.Fatalf("expected rest client to be stored")
	}
	if bundle.Router() != "router" {
		t.Fatalf("expected router to be created")
	}
	if bundle.WSInfra() != stream {
		t.Fatalf("expected stream client to be stored")
	}
}

func TestBuildTransportBundleSkipsRoutersWhenMissing(t *testing.T) {
	bundle := BuildTransportBundle(TransportFactories{}, RouterFactories{NewRESTRouter: func(coretransport.RESTClient) interface{} {
		t.Fatal("router factory should not be invoked without rest client")
		return nil
	}}, TransportConfig{})

	if bundle.REST() != nil || bundle.Router() != nil || bundle.WSInfra() != nil {
		t.Fatalf("expected empty bundle when factories absent")
	}
}

func TestTransportBundleCloseAggregatesErrors(t *testing.T) {
	rest := &stubRESTClient{closeErr: errors.New("rest")}
	stream := &stubStreamClient{closeErr: errors.New("stream")}
	wsRouter := &closable{err: errors.New("ws")}

	bundle := NewTransportBundle(rest, &closable{err: errors.New("router")}, stream)
	bundle.SetWS(wsRouter)

	err := bundle.Close()
	if !rest.closed || !stream.closed || !wsRouter.closed {
		t.Fatalf("expected all close handlers to be invoked")
	}
	if err == nil || !errors.Is(err, rest.closeErr) || !errors.Is(err, stream.closeErr) || !errors.Is(err, wsRouter.err) {
		t.Fatalf("expected aggregated error, got %v", err)
	}
}

func TestApplyOptionsIgnoresNil(t *testing.T) {
	params := &ConstructionParams{}
	invoked := false
	ApplyOptions(params, nil, func(p *ConstructionParams) {
		invoked = true
		p.ConfigOpts = append(p.ConfigOpts, nil)
	})
	if !invoked {
		t.Fatalf("expected non-nil option to run")
	}
	if len(params.ConfigOpts) != 1 {
		t.Fatalf("expected option to mutate params")
	}
}

func TestWithConfigAppendsOptions(t *testing.T) {
	called := false
	opt := WithConfig(func(s *config.Settings) {
		called = true
	})

	params := &ConstructionParams{}
	opt(params)

	if len(params.ConfigOpts) != 1 {
		t.Fatalf("expected option to be appended")
	}
	params.ConfigOpts[0](&config.Settings{})
	if !called {
		t.Fatalf("expected wrapped config option to execute")
	}
}

func TestNewConstructionParamsInitializesSlice(t *testing.T) {
	params := NewConstructionParams()
	if params == nil {
		t.Fatalf("expected params to be allocated")
	}
	if params.ConfigOpts == nil || len(params.ConfigOpts) != 0 {
		t.Fatalf("expected config options slice to be initialized empty")
	}
}
