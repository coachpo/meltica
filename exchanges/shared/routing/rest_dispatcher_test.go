package routing

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	coretransport "github.com/coachpo/meltica/core/transport"
)

func TestRESTAPIResolverFunc(t *testing.T) {
	resolver := RESTAPIResolverFunc(func(msg RESTMessage) string {
		if msg.Method != "GET" {
			t.Fatalf("unexpected method %s", msg.Method)
		}
		return "spot"
	})
	api := resolver.ResolveRESTAPI(RESTMessage{Method: "GET"})
	if api != "spot" {
		t.Fatalf("expected resolver to return spot, got %s", api)
	}
}

func TestDefaultRESTRouterDispatchUsesResolver(t *testing.T) {
	client := &stubRESTClient{
		response: &coretransport.RESTResponse{Status: 202, Header: http.Header{"X-Test": {"ok"}}},
	}
	router := NewDefaultRESTRouter(client, RESTAPIResolverFunc(func(msg RESTMessage) string {
		if msg.Path != "/api/v3/ping" {
			t.Fatalf("unexpected path %s", msg.Path)
		}
		return "spot"
	}))

	var out struct{ Status int }
	err := router.Dispatch(context.Background(), RESTMessage{
		Method: "GET",
		Path:   "/api/v3/ping",
		Query:  map[string]string{"symbol": "BTCUSDT"},
		Header: http.Header{"X-Test": {"ok"}},
	}, &out)
	if err != nil {
		t.Fatalf("Dispatch returned error: %v", err)
	}
	if client.lastRequest.API != "spot" {
		t.Fatalf("expected resolver API 'spot', got %s", client.lastRequest.API)
	}
	if out.Status != 202 {
		t.Fatalf("expected status 202, got %d", out.Status)
	}
}

func TestDefaultRESTRouterDispatchHandlesError(t *testing.T) {
	sentinel := errors.New("boom")
	client := &stubRESTClient{responseErr: sentinel, handleErr: errors.New("decorated")}
	router := NewDefaultRESTRouter(client, nil)

	err := router.Dispatch(context.Background(), RESTMessage{API: "linear", Method: "POST", Path: "/api/v3/order"}, &struct{}{})
	if !errors.Is(err, client.handleErr) {
		t.Fatalf("expected decorated error, got %v", err)
	}
	if client.lastRequest.API != "linear" {
		t.Fatalf("expected last request API linear, got %s", client.lastRequest.API)
	}
}

func TestDefaultRESTRouterDispatchNoClient(t *testing.T) {
	var router *DefaultRESTRouter
	if err := router.Dispatch(context.Background(), RESTMessage{}, nil); err != nil {
		t.Fatalf("expected nil router to succeed, got %v", err)
	}
	router = &DefaultRESTRouter{}
	if err := router.Dispatch(context.Background(), RESTMessage{}, nil); err != nil {
		t.Fatalf("expected router without client to succeed, got %v", err)
	}
}

type stubRESTClient struct {
	lastRequest       coretransport.RESTRequest
	response          *coretransport.RESTResponse
	responseErr       error
	handleErr         error
	handleResponseErr error
}

func (s *stubRESTClient) Connect(context.Context) error { return nil }

func (s *stubRESTClient) Close() error { return nil }

func (s *stubRESTClient) DoRequest(_ context.Context, req coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
	s.lastRequest = req
	if s.responseErr != nil {
		return nil, s.responseErr
	}
	if s.response != nil {
		return s.response, nil
	}
	return &coretransport.RESTResponse{Status: http.StatusOK, ReceivedAt: time.Now()}, nil
}

func (s *stubRESTClient) HandleResponse(_ context.Context, req coretransport.RESTRequest, resp *coretransport.RESTResponse, out any) error {
	s.lastRequest = req
	if s.handleResponseErr != nil {
		return s.handleResponseErr
	}
	if dst, ok := out.(*struct{ Status int }); ok && resp != nil {
		dst.Status = resp.Status
	}
	return nil
}

func (s *stubRESTClient) HandleError(_ context.Context, req coretransport.RESTRequest, err error) error {
	s.lastRequest = req
	if s.handleErr != nil {
		return s.handleErr
	}
	return err
}
