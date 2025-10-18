package binance

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	json "github.com/goccy/go-json"
)

func TestRESTFetcher_Fetch_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	data, err := fetcher.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	
	if !strings.Contains(string(data), "success") {
		t.Errorf("expected success in response, got %s", string(data))
	}
}

func TestRESTFetcher_Fetch_WithAPIKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-MBX-APIKEY")
		if apiKey != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	data, err := fetcher.Fetch(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	
	if !strings.Contains(string(data), "success") {
		t.Errorf("expected success in response, got %s", string(data))
	}
}

func TestRESTFetcher_Fetch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad request"}`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	_, err := fetcher.Fetch(context.Background(), server.URL)
	if err == nil {
		t.Error("expected error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestRESTFetcher_Fetch_EmptyEndpoint(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	_, err := fetcher.Fetch(context.Background(), "")
	if err == nil {
		t.Error("expected error for empty endpoint")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected 'empty' in error, got: %v", err)
	}
}

func TestRESTFetcher_Fetch_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	
	_, err := fetcher.Fetch(ctx, server.URL)
	if err == nil {
		t.Error("expected error for context cancellation")
	}
}

func TestRESTFetcher_FetchSigned_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if !query.Has("timestamp") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "missing timestamp"}`))
			return
		}
		if !query.Has("signature") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error": "missing signature"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success": true}`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	params := url.Values{}
	params.Set("symbol", "BTCUSDT")
	
	data, err := fetcher.FetchSigned(context.Background(), "", params)
	if err != nil {
		t.Fatalf("fetch signed failed: %v", err)
	}
	
	if !strings.Contains(string(data), "success") {
		t.Errorf("expected success in response, got %s", string(data))
	}
}

func TestRESTFetcher_FetchSigned_NoSecretKey(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test-key", "", false)
	
	_, err := fetcher.FetchSigned(context.Background(), "/test", nil)
	if err == nil {
		t.Error("expected error for missing secret key")
	}
	if !strings.Contains(err.Error(), "secret key required") {
		t.Errorf("expected 'secret key required' error, got: %v", err)
	}
}

func TestRESTFetcher_Sign(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	payload := "symbol=BTCUSDT&timestamp=1234567890"
	signature := fetcher.sign(payload)
	
	if signature == "" {
		t.Error("expected non-empty signature")
	}
	
	signature2 := fetcher.sign(payload)
	if signature != signature2 {
		t.Error("expected consistent signatures for same payload")
	}
	
	signature3 := fetcher.sign("different")
	if signature == signature3 {
		t.Error("expected different signatures for different payloads")
	}
}

func TestRESTFetcher_GetServerTime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := map[string]int64{"serverTime": 1234567890}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	serverTime, err := fetcher.GetServerTime(context.Background())
	if err != nil {
		t.Fatalf("get server time failed: %v", err)
	}
	
	if serverTime != 1234567890 {
		t.Errorf("expected server time 1234567890, got %d", serverTime)
	}
}

func TestRESTFetcher_GetServerTime_ParseError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`invalid json`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	_, err := fetcher.GetServerTime(context.Background())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestRESTFetcher_CreateListenKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		apiKey := r.Header.Get("X-MBX-APIKEY")
		if apiKey != "test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		response := map[string]string{"listenKey": "test-listen-key"}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	listenKey, err := fetcher.CreateListenKey(context.Background())
	if err != nil {
		t.Fatalf("create listen key failed: %v", err)
	}
	
	if listenKey != "test-listen-key" {
		t.Errorf("expected listen key test-listen-key, got %s", listenKey)
	}
}

func TestRESTFetcher_CreateListenKey_NoAPIKey(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("", "test-secret", false)
	
	_, err := fetcher.CreateListenKey(context.Background())
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("expected 'API key required' error, got: %v", err)
	}
}

func TestRESTFetcher_KeepAliveListenKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if !strings.Contains(r.URL.String(), "listenKey=test-key") {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	err := fetcher.KeepAliveListenKey(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("keep alive listen key failed: %v", err)
	}
}

func TestRESTFetcher_KeepAliveListenKey_NoAPIKey(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("", "test-secret", false)
	
	err := fetcher.KeepAliveListenKey(context.Background(), "test-key")
	if err == nil {
		t.Error("expected error for missing API key")
	}
	if !strings.Contains(err.Error(), "API key required") {
		t.Errorf("expected 'API key required' error, got: %v", err)
	}
}

func TestRESTFetcher_KeepAliveListenKey_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid listen key"}`))
	}))
	defer server.Close()
	
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	fetcher.baseURL = server.URL
	
	err := fetcher.KeepAliveListenKey(context.Background(), "test-key")
	if err == nil {
		t.Error("expected error for HTTP 400")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("expected status code in error, got: %v", err)
	}
}

func TestRESTFetcher_Testnet(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", true)
	
	if fetcher.baseURL != binanceTestnetRESTURL {
		t.Errorf("expected testnet URL %s, got %s", binanceTestnetRESTURL, fetcher.baseURL)
	}
	
	if !fetcher.useTestnet {
		t.Error("expected useTestnet to be true")
	}
}

func TestRESTFetcher_Production(t *testing.T) {
	fetcher := NewBinanceRESTFetcher("test-key", "test-secret", false)
	
	if fetcher.baseURL != binanceRESTURL {
		t.Errorf("expected production URL %s, got %s", binanceRESTURL, fetcher.baseURL)
	}
	
	if fetcher.useTestnet {
		t.Error("expected useTestnet to be false")
	}
}
