package mocks

import (
	"context"
	"errors"
	"testing"

	coretransport "github.com/coachpo/meltica/core/transport"
)

func TestRESTClientDelegatesToInjectedFunctions(t *testing.T) {
	req := coretransport.RESTRequest{Method: "GET"}
	resp := &coretransport.RESTResponse{}
	ctx := context.Background()

	client := &RESTClient{
		ConnectFunc: func(c context.Context) error {
			if c != ctx {
				t.Fatalf("unexpected context: %v", c)
			}
			return errors.New("connect")
		},
		CloseFunc: func() error { return errors.New("close") },
		DoRequestFunc: func(c context.Context, r coretransport.RESTRequest) (*coretransport.RESTResponse, error) {
			if r.Method != req.Method {
				t.Fatalf("unexpected request: %+v", r)
			}
			return resp, errors.New("request")
		},
		HandleResponseFunc: func(c context.Context, r coretransport.RESTRequest, received *coretransport.RESTResponse, out any) error {
			if received != resp {
				t.Fatalf("unexpected response pointer")
			}
			if out == nil {
				t.Fatalf("expected output receiver")
			}
			return errors.New("response")
		},
		HandleErrorFunc: func(c context.Context, r coretransport.RESTRequest, err error) error {
			if err == nil {
				t.Fatalf("expected error input")
			}
			return errors.New("handled")
		},
	}

	if err := client.Connect(ctx); err == nil || err.Error() != "connect" {
		t.Fatalf("expected connect delegation, got %v", err)
	}
	if err := client.Close(); err == nil || err.Error() != "close" {
		t.Fatalf("expected close delegation, got %v", err)
	}
	gotResp, err := client.DoRequest(ctx, req)
	if err == nil || err.Error() != "request" || gotResp != resp {
		t.Fatalf("expected DoRequest delegation, got resp=%v err=%v", gotResp, err)
	}
	var out any
	if err := client.HandleResponse(ctx, req, resp, &out); err == nil || err.Error() != "response" {
		t.Fatalf("expected HandleResponse delegation, got %v", err)
	}
	if err := client.HandleError(ctx, req, errors.New("boom")); err == nil || err.Error() != "handled" {
		t.Fatalf("expected HandleError delegation, got %v", err)
	}
}

func TestRESTClientFallbackBehaviour(t *testing.T) {
	var nilClient *RESTClient
	if err := nilClient.Connect(context.Background()); err != nil {
		t.Fatalf("nil client connect should be no-op, got %v", err)
	}
	if err := nilClient.Close(); err != nil {
		t.Fatalf("nil client close should be no-op, got %v", err)
	}
	if resp, err := nilClient.DoRequest(context.Background(), coretransport.RESTRequest{}); err != nil || resp != nil {
		t.Fatalf("expected nil response and error, got resp=%v err=%v", resp, err)
	}

	client := &RESTClient{}
	sentinel := errors.New("boom")
	if err := client.HandleResponse(context.Background(), coretransport.RESTRequest{}, nil, nil); err != nil {
		t.Fatalf("expected nil when handler missing, got %v", err)
	}
	if err := client.HandleError(context.Background(), coretransport.RESTRequest{}, sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("expected passthrough error, got %v", err)
	}
}
