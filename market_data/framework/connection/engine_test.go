package connection

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/market_data/framework"
)

type stubNetError struct {
	timeout bool
}

func (s stubNetError) Error() string   { return "net" }
func (s stubNetError) Timeout() bool   { return s.timeout }
func (s stubNetError) Temporary() bool { return true }

func TestNormalizeConfigAppliesDefaults(t *testing.T) {
	parsed, err := normalizeConfig(Config{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.endpoint != "" {
		t.Fatalf("expected empty endpoint, got %q", parsed.endpoint)
	}
	if parsed.dialTimeout != defaultDialTimeout {
		t.Fatalf("expected default dial timeout, got %v", parsed.dialTimeout)
	}
	if parsed.metricsWindow != defaultMetricsWindow {
		t.Fatalf("expected default metrics window, got %v", parsed.metricsWindow)
	}
	if parsed.poolOpts.MaxMessageBytes != defaultMaxMessageBytes {
		t.Fatalf("expected default max message bytes, got %d", parsed.poolOpts.MaxMessageBytes)
	}
	if parsed.heartbeatInterval != 15*time.Second || parsed.heartbeatTimeout != 45*time.Second {
		t.Fatalf("unexpected heartbeat defaults: %+v", parsed)
	}
	if parsed.maxPending != 128 {
		t.Fatalf("expected backpressure default of 128, got %d", parsed.maxPending)
	}
	if parsed.throttleDuration != 50*time.Millisecond {
		t.Fatalf("expected throttle default, got %v", parsed.throttleDuration)
	}
}

func TestNormalizeConfigPreservesValues(t *testing.T) {
	cfg := Config{
		Endpoint:      "wss://example",
		DialTimeout:   2 * time.Second,
		Pool:          PoolOptions{MaxMessageBytes: 2048},
		MetricsWindow: time.Minute,
		Heartbeat:     HeartbeatConfig{Interval: time.Second, Liveness: 2 * time.Second},
		Backpressure:  BackpressureConfig{MaxPending: 10, Throttle: time.Millisecond},
	}
	parsed, err := normalizeConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if parsed.endpoint != "wss://example" {
		t.Fatalf("expected endpoint to persist")
	}
	if parsed.dialTimeout != 2*time.Second || parsed.metricsWindow != time.Minute {
		t.Fatalf("expected custom timings to persist: %+v", parsed)
	}
	if parsed.poolOpts.MaxMessageBytes != 2048 {
		t.Fatalf("expected pool size to persist, got %d", parsed.poolOpts.MaxMessageBytes)
	}
	if parsed.heartbeatInterval != time.Second || parsed.heartbeatTimeout != 2*time.Second {
		t.Fatalf("expected heartbeat configs to persist: %+v", parsed)
	}
	if parsed.maxPending != 10 || parsed.throttleDuration != time.Millisecond {
		t.Fatalf("expected backpressure to persist: %+v", parsed)
	}
}

func TestMergeMetadata(t *testing.T) {
	merged := mergeMetadata(map[string]string{"a": "1"}, map[string]string{"b": "2", "a": "override"}, nil)
	if len(merged) != 2 || merged["a"] != "override" || merged["b"] != "2" {
		t.Fatalf("unexpected merge result: %+v", merged)
	}
	if merge := mergeMetadata(nil); merge != nil {
		t.Fatalf("expected nil merge when no entries")
	}
}

func TestDialOptionsRoundTrip(t *testing.T) {
	opts := DialOptions{Endpoint: "wss://x", Protocols: []string{"p1", "p2"}, Headers: map[string]string{"k": "v"}}
	ctx := WithDialOptions(context.Background(), opts)
	got := dialOptionsFromContext(ctx)
	if got.Endpoint != opts.Endpoint || len(got.Protocols) != len(opts.Protocols) || got.Headers["k"] != "v" {
		t.Fatalf("expected round-trip of dial options, got %+v", got)
	}
	if empty := dialOptionsFromContext(nil); empty.Endpoint != "" || len(empty.Protocols) != 0 {
		t.Fatalf("expected zero options when context missing")
	}
}

func TestWrapDialErrorMappings(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		resp   *http.Response
		expect errs.Code
	}{
		{"unauthorized", errors.New("fail"), &http.Response{StatusCode: http.StatusUnauthorized}, errs.CodeAuth},
		{"rate limited", errors.New("fail"), &http.Response{StatusCode: http.StatusTooManyRequests}, errs.CodeRateLimited},
		{"invalid request", errors.New("fail"), &http.Response{StatusCode: http.StatusBadRequest}, errs.CodeInvalid},
		{"canceled", context.Canceled, nil, errs.CodeNetwork},
		{"deadline", context.DeadlineExceeded, nil, errs.CodeNetwork},
		{"net timeout", stubNetError{timeout: true}, nil, errs.CodeNetwork},
		{"net error", stubNetError{timeout: false}, nil, errs.CodeNetwork},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wrapped := wrapDialError(tc.err, tc.resp)
			var e *errs.E
			if !errors.As(wrapped, &e) {
				t.Fatalf("expected errs.E, got %v", wrapped)
			}
			if e.Code != tc.expect {
				t.Fatalf("expected code %s, got %s", tc.expect, e.Code)
			}
		})
	}
	if wrapDialError(nil, nil) != nil {
		t.Fatalf("nil error should stay nil")
	}
}

func TestMetricsReporterLifecycle(t *testing.T) {
	reporter := newMetricsReporter(0)
	reporter.Track("session", nil)
	if snapshot, ok := reporter.Snapshot("session"); !ok || snapshot.Window() <= 0 {
		t.Fatalf("expected snapshot to exist")
	}
	reporter.Track("session", &framework.MetricsSnapshot{Session: "session", WindowLength: time.Second})
	all := reporter.All()
	if len(all) != 1 || all[0].SessionID() != "session" {
		t.Fatalf("expected single snapshot, got %+v", all)
	}
	reporter.Remove("session")
	if _, ok := reporter.Snapshot("session"); ok {
		t.Fatalf("expected snapshot removal")
	}
}

func TestNewSessionIDGeneratesUniqueValues(t *testing.T) {
	a, b := newSessionID(), newSessionID()
	if len(a) == 0 || len(b) == 0 {
		t.Fatalf("expected non-empty session ids")
	}
	if a == b {
		t.Fatalf("expected session ids to differ")
	}
}
