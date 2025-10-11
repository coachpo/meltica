package routing

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/layers"
	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type stubRESTConnection struct {
	client coretransport.RESTClient
}

var _ layers.RESTConnection = (*stubRESTConnection)(nil)

func newStubRESTConnection(client coretransport.RESTClient) *stubRESTConnection {
	return &stubRESTConnection{client: client}
}

func (s *stubRESTConnection) Connect(context.Context) error    { return nil }
func (s *stubRESTConnection) Close() error                     { return nil }
func (s *stubRESTConnection) IsConnected() bool                { return true }
func (s *stubRESTConnection) SetReadDeadline(time.Time) error  { return nil }
func (s *stubRESTConnection) SetWriteDeadline(time.Time) error { return nil }
func (s *stubRESTConnection) Do(context.Context, *layers.HTTPRequest) (*layers.HTTPResponse, error) {
	return nil, nil
}
func (s *stubRESTConnection) SetRateLimit(int) {}
func (s *stubRESTConnection) LegacyRESTClient() coretransport.RESTClient {
	return s.client
}

func TestRESTRouterDispatchSuccess(t *testing.T) {
	ctx := context.Background()
	client := &mocks.RESTClient{}
	var capturedReq coretransport.RESTRequest
	var handleCalled bool
	client.DoRequestFunc = func(_ context.Context, req coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
		capturedReq = req
		return &coretransport.RESTResponse{Status: http.StatusOK}, nil
	}
	client.HandleResponseFunc = func(_ context.Context, req coretransport.RESTRequest, resp *coretransport.RESTResponse, out any) error {
		handleCalled = true
		if resp.Status != http.StatusOK {
			t.Fatalf("unexpected status: %d", resp.Status)
		}
		return nil
	}
	router := NewRESTRouter(newStubRESTConnection(client))
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodGet, Path: "/api/v3/time"}
	if err := router.Dispatch(ctx, msg, nil); err != nil {
		t.Fatalf("dispatch returned error: %v", err)
	}
	if !handleCalled {
		t.Fatalf("expected handle response to be called")
	}
	if capturedReq.API != string(rest.SpotAPI) {
		t.Fatalf("unexpected api forwarded: %s", capturedReq.API)
	}
}

func TestRESTRouterDispatchError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("boom")
	returnedErr := errors.New("handled")
	client := &mocks.RESTClient{}
	client.DoRequestFunc = func(_ context.Context, req coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
		return nil, expectedErr
	}
	var handleErrorCalled bool
	client.HandleErrorFunc = func(_ context.Context, req coretransport.RESTRequest, err error) error {
		handleErrorCalled = true
		if !errors.Is(err, expectedErr) {
			t.Fatalf("unexpected error passed to handler: %v", err)
		}
		return returnedErr
	}
	router := NewRESTRouter(newStubRESTConnection(client))
	msg := routingrest.RESTMessage{API: string(rest.LinearAPI), Method: http.MethodGet, Path: "/fapi/v1/time"}
	err := router.Dispatch(ctx, msg, nil)
	if !handleErrorCalled {
		t.Fatalf("expected handle error to be called")
	}
	if !errors.Is(err, returnedErr) {
		t.Fatalf("expected error %v, got %v", returnedErr, err)
	}
}
