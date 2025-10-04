package routing

import (
	"context"
	"errors"
	"net/http"
	"testing"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/core/exchange/mocks"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
)

func TestRESTRouterDispatchSuccess(t *testing.T) {
	ctx := context.Background()
	client := &mocks.RESTClient{}
	var capturedReq coreexchange.RESTRequest
	var handleCalled bool
	client.DoRequestFunc = func(_ context.Context, req coreexchange.RESTRequest) (*coreexchange.RESTResponse, error) {
		capturedReq = req
		return &coreexchange.RESTResponse{Status: http.StatusOK}, nil
	}
	client.HandleResponseFunc = func(_ context.Context, req coreexchange.RESTRequest, resp *coreexchange.RESTResponse, out any) error {
		handleCalled = true
		if resp.Status != http.StatusOK {
			t.Fatalf("unexpected status: %d", resp.Status)
		}
		return nil
	}
	router := NewRESTRouter(client)
	msg := RESTMessage{API: rest.SpotAPI, Method: http.MethodGet, Path: "/api/v3/time"}
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
	client.DoRequestFunc = func(_ context.Context, req coreexchange.RESTRequest) (*coreexchange.RESTResponse, error) {
		return nil, expectedErr
	}
	var handleErrorCalled bool
	client.HandleErrorFunc = func(_ context.Context, req coreexchange.RESTRequest, err error) error {
		handleErrorCalled = true
		if !errors.Is(err, expectedErr) {
			t.Fatalf("unexpected error passed to handler: %v", err)
		}
		return returnedErr
	}
	router := NewRESTRouter(client)
	msg := RESTMessage{API: rest.LinearAPI, Method: http.MethodGet, Path: "/fapi/v1/time"}
	err := router.Dispatch(ctx, msg, nil)
	if !handleErrorCalled {
		t.Fatalf("expected handle error to be called")
	}
	if !errors.Is(err, returnedErr) {
		t.Fatalf("expected error %v, got %v", returnedErr, err)
	}
}
