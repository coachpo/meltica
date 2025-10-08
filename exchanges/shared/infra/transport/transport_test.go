package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type temporaryNetError struct{}

func (temporaryNetError) Error() string   { return "temporary network error" }
func (temporaryNetError) Timeout() bool   { return false }
func (temporaryNetError) Temporary() bool { return true }

func TestDoWithHeaders_RetryAfterRateLimit(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()

	client := &Client{
		HTTP: server.Client(),
		Retry: RetryPolicy{
			MaxRetries: 2,
			BaseDelay:  time.Millisecond,
			MaxDelay:   5 * time.Millisecond,
		},
		BaseURL: server.URL,
	}

	var waits []time.Duration
	client.OnRLWait = func(wait time.Duration) {
		waits = append(waits, wait)
	}

	ctx := context.Background()
	var out struct {
		OK bool `json:"ok"`
	}
	_, status, err := client.DoWithHeaders(ctx, http.MethodGet, "/", nil, nil, false, http.Header{}, &out)
	if err != nil {
		t.Fatalf("DoWithHeaders returned error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	if out.OK != true {
		t.Fatalf("expected decoded body with ok=true")
	}
	if got := atomic.LoadInt32(&attempts); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
	if len(waits) == 0 {
		t.Fatalf("expected OnRLWait to be invoked at least once")
	}
}

func TestDoWithHeaders_RetryOnNetworkError(t *testing.T) {
	var calls int
	mu := &sync.Mutex{}
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		if calls == 1 {
			return nil, temporaryNetError{}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(`{"done":true}`)),
			Header:     make(http.Header),
		}, nil
	})

	client := &Client{
		HTTP: &http.Client{Transport: transport},
		Retry: RetryPolicy{
			MaxRetries: 2,
			BaseDelay:  time.Millisecond,
			MaxDelay:   5 * time.Millisecond,
		},
		BaseURL: "https://example.com",
	}

	_, status, err := client.DoWithHeaders(context.Background(), http.MethodGet, "/resource", nil, nil, false, http.Header{}, nil)
	if err != nil {
		t.Fatalf("DoWithHeaders returned error: %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status 200, got %d", status)
	}
	mu.Lock()
	if calls != 2 {
		mu.Unlock()
		t.Fatalf("expected 2 calls to transport, got %d", calls)
	}
	mu.Unlock()
}

func TestDoWithHeaders_BackoffRespectsContextCancellation(t *testing.T) {
	transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, temporaryNetError{}
	})

	client := &Client{
		HTTP: &http.Client{Transport: transport},
		Retry: RetryPolicy{
			MaxRetries: 5,
			BaseDelay:  20 * time.Millisecond,
			MaxDelay:   20 * time.Millisecond,
		},
		BaseURL: "https://example.com",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, _, err := client.DoWithHeaders(ctx, http.MethodGet, "/resource", nil, nil, false, http.Header{}, nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
