package routing

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/coachpo/meltica/core/streams/mocks"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

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
	router := NewRESTRouter(client)
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
	router := NewRESTRouter(client)
	msg := routingrest.RESTMessage{API: string(rest.LinearAPI), Method: http.MethodGet, Path: "/fapi/v1/time"}
	err := router.Dispatch(ctx, msg, nil)
	if !handleErrorCalled {
		t.Fatalf("expected handle error to be called")
	}
	if !errors.Is(err, returnedErr) {
		t.Fatalf("expected error %v, got %v", returnedErr, err)
	}
}
