package rest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"testing"
	"time"

	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/exchanges/binance/internal"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestNewClientAppliesDefaults(t *testing.T) {
	cfg := Config{APIKey: "key", Secret: "sec", Timeout: 5 * time.Second}
	client := NewClient(cfg)

	if got := client.Spot().HTTP.Timeout; got != 5*time.Second {
		t.Fatalf("expected timeout override, got %v", got)
	}
	if client.Spot().BaseURL != defaultSpotBase || client.Linear().BaseURL != defaultLinearBase || client.Inverse().BaseURL != defaultInverseBase {
		t.Fatalf("expected default base urls to be applied")
	}
	hdr, err := client.Spot().Signer("GET", "/api/v3/time", map[string]string{"symbol": "BTCUSDT"}, nil, time.Now().UnixMilli())
	if err != nil {
		t.Fatalf("unexpected signer error: %v", err)
	}
	if hdr.Get("X-MBX-APIKEY") != "key" {
		t.Fatalf("expected api key header to be set")
	}
}

func TestClientDoRequestSignsAndCopiesResponse(t *testing.T) {
	var seen *http.Request
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		seen = req
		if req.URL.Host != "api.binance.com" {
			t.Fatalf("unexpected host %s", req.URL.Host)
		}
		if req.URL.Query().Get("symbol") != "BTCUSDT" {
			t.Fatalf("expected query propagation")
		}
		if req.URL.Query().Get("timestamp") == "" || req.URL.Query().Get("signature") == "" {
			t.Fatalf("expected request to be signed")
		}
		if req.Header.Get("X-MBX-APIKEY") != "key" {
			t.Fatalf("expected api key header")
		}
		body, _ := io.ReadAll(req.Body)
		if string(body) != `{"foo":"bar"}` {
			t.Fatalf("unexpected body: %s", body)
		}
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{"X-Test": []string{"value"}},
			Body:       io.NopCloser(bytes.NewBufferString(`{"ok":true}`)),
		}
		return resp, nil
	})}

	client := NewClient(Config{APIKey: "key", Secret: "sec", HTTPClient: httpClient})
	req := coretransport.RESTRequest{
		Method: "POST",
		API:    string(SpotAPI),
		Path:   "/api/v3/order",
		Query:  map[string]string{"symbol": "BTCUSDT"},
		Body:   []byte(`{"foo":"bar"}`),
		Signed: true,
		Header: http.Header{
			"X-Custom": []string{"value"},
		},
	}

	resp, err := client.DoRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != 200 || string(resp.Body) != `{"ok":true}` {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Header.Get("X-Test") != "value" {
		t.Fatalf("expected header copy")
	}
	if resp.ReceivedAt.IsZero() {
		t.Fatalf("expected received timestamp")
	}
	if seen == nil || seen.Header.Get("X-Custom") != "value" {
		t.Fatalf("expected custom header to propagate")
	}
}

func TestClientDoDecodesAndWrapsErrors(t *testing.T) {
	replies := make(chan *http.Response, 1)
	httpClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("X-Fail") == "network" {
			return nil, errors.New("boom")
		}
		return <-replies, nil
	})}

	client := NewClient(Config{APIKey: "key", Secret: "sec", HTTPClient: httpClient})

	replies <- &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewBufferString(`{"value":1}`))}
	var out struct{ Value int }
	if err := client.Do(context.Background(), coretransport.RESTRequest{Method: "GET", Path: "/api", Signed: true}, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Value != 1 {
		t.Fatalf("expected decoded payload, got %#v", out)
	}

	_, err := client.clientForAPI("bad")
	if err == nil {
		t.Fatalf("expected client selection error")
	}

	err = client.Do(context.Background(), coretransport.RESTRequest{Method: "GET", Path: "/api", Signed: true, Header: http.Header{"X-Fail": []string{"network"}}}, &out)
	var e *errs.E
	if !errors.As(err, &e) || e.Code != errs.CodeNetwork {
		t.Fatalf("expected network error, got %v", err)
	}
}

func TestClientHandleResponseAndError(t *testing.T) {
	client := NewClient(Config{})
	req := coretransport.RESTRequest{Method: "GET", Path: "/api"}

	if err := client.HandleResponse(context.Background(), req, &coretransport.RESTResponse{Body: []byte("{}")}, &struct{}{}); err != nil {
		t.Fatalf("unexpected decode error: %v", err)
	}
	err := client.HandleResponse(context.Background(), req, &coretransport.RESTResponse{Body: []byte("not json")}, &struct{}{})
	var e *errs.E
	if !errors.As(err, &e) || e.Code != errs.CodeExchange {
		t.Fatalf("expected exchange decode error, got %v", err)
	}
	if err := client.HandleResponse(context.Background(), req, &coretransport.RESTResponse{}, nil); err != nil {
		t.Fatalf("expected nil when no output: %v", err)
	}

	if err := client.HandleError(context.Background(), req, nil); err != nil {
		t.Fatalf("expected nil when no error: %v", err)
	}
	if err := client.HandleError(context.Background(), req, context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected passthrough cancellation, got %v", err)
	}
	wrapped := internal.Network("boom")
	if err := client.HandleError(context.Background(), req, wrapped); err != wrapped {
		t.Fatalf("expected canonical errors to pass through")
	}
	err = client.HandleError(context.Background(), req, errors.New("boom"))
	if !errors.As(err, &e) || e.Code != errs.CodeNetwork {
		t.Fatalf("expected wrapped network error, got %v", err)
	}
}

func TestClientConnectHonoursContext(t *testing.T) {
	client := NewClient(Config{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Connect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
	if err := client.Close(); err != nil {
		t.Fatalf("expected close no-op, got %v", err)
	}
}
