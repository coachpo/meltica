package framework

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/core/stream"
)

func TestConnectionSessionMetadataAndLifecycle(t *testing.T) {
	sess := &ConnectionSession{SessionID: "abc", EndpointAddr: "wss://example", BaseContext: context.Background()}

	errCh := make(chan error, 1)
	sess.SetErrorChannel(errCh)
	if got := sess.Errors(); got != errCh {
		t.Fatal("expected error channel to be set")
	}

	called := false
	closeCtx := context.WithValue(context.Background(), struct{}{}, 1)
	sess.SetCloseFunc(func(ctx context.Context) error {
		if ctx != closeCtx {
			t.Fatal("expected Close to forward context")
		}
		called = true
		return nil
	})
	if err := sess.Close(closeCtx); err != nil || !called {
		t.Fatalf("expected close func to be invoked, err=%v called=%v", err, called)
	}

	// Ensure no-op if close function missing.
	sess.SetCloseFunc(nil)
	if err := sess.Close(context.Background()); err != nil {
		t.Fatalf("expected nil close function to succeed, got %v", err)
	}

	now := time.Now()
	sess.SetLastHeartbeat(now)
	if beat := sess.LastHeartbeat(); !beat.Equal(now) {
		t.Fatalf("expected heartbeat %v, got %v", now, beat)
	}

	sess.SetStatus(stream.SessionActive)
	if sess.Status() != stream.SessionActive {
		t.Fatalf("expected active status")
	}

	newCtx := context.WithValue(context.Background(), "key", "value")
	sess.SetContext(newCtx)
	if sess.Context() != newCtx {
		t.Fatalf("expected updated context")
	}

	sess.SetInvalidCount(7)
	if sess.InvalidCount != 7 {
		t.Fatalf("expected invalid count to update")
	}

	meta := map[string]string{"foo": "bar"}
	sess.SetMetadata(meta)
	returned := sess.Metadata()
	if returned["foo"] != "bar" {
		t.Fatalf("expected metadata copy to include key")
	}
	returned["foo"] = "mutated"
	if sess.Metadata()["foo"] != "bar" {
		t.Fatalf("expected metadata copy to protect internal state")
	}

	sess.SetMetadata(nil)
	if sess.Metadata() != nil {
		t.Fatalf("expected metadata reset to nil")
	}
}

func TestMessageEnvelopeAccessors(t *testing.T) {
	env := &MessageEnvelope{}
	env.SetRaw([]byte("data"))
	env.SetDecoded(map[string]string{"k": "v"})
	now := time.Now()
	env.SetReceivedAt(now)
	env.SetValidated(true)
	env.AppendError(errors.New("boom"))
	env.AppendError(nil)

	if string(env.Raw()) != "data" {
		t.Fatalf("unexpected raw bytes")
	}
	if env.Decoded().(map[string]string)["k"] != "v" {
		t.Fatalf("unexpected decoded value")
	}
	if !env.ReceivedAt().Equal(now) {
		t.Fatalf("expected received timestamp")
	}
	if !env.Validated() {
		t.Fatalf("expected envelope to be marked valid")
	}
	if len(env.Errors()) != 1 {
		t.Fatalf("expected non-nil errors slice")
	}

	env.Reset()
	if env.Validated() || len(env.Errors()) != 0 || len(env.Raw()) != 0 {
		t.Fatalf("expected reset to clear all state")
	}
}

func TestHandlerOutcomeAccessors(t *testing.T) {
	meta := map[string]string{"a": "b"}
	outcome := HandlerOutcome{
		OutcomeType: stream.OutcomeTransform,
		PayloadData: 42,
		LatencyDur:  time.Millisecond,
		Err:         errors.New("noop"),
		Meta:        meta,
	}

	if outcome.Type() != stream.OutcomeTransform {
		t.Fatalf("unexpected outcome type")
	}
	if outcome.Payload() != 42 {
		t.Fatalf("unexpected payload")
	}
	if outcome.Error() == nil {
		t.Fatalf("expected error value")
	}
	if outcome.Metadata()["a"] != "b" {
		t.Fatalf("expected metadata accessor")
	}
	if outcome.Latency() != time.Millisecond {
		t.Fatalf("unexpected latency")
	}
}

func TestMetricsSnapshotAccessors(t *testing.T) {
	snap := MetricsSnapshot{
		Session:       "sess",
		WindowLength:  time.Second,
		MessagesTotal: 10,
		ErrorsTotal:   2,
		P50:           time.Millisecond,
		P95:           5 * time.Millisecond,
		Allocated:     512,
	}

	if snap.SessionID() != "sess" {
		t.Fatalf("unexpected session id")
	}
	if snap.Window() != time.Second {
		t.Fatalf("unexpected window duration")
	}
	if snap.Messages() != 10 || snap.Errors() != 2 {
		t.Fatalf("unexpected message/error counts")
	}
	if snap.P50Latency() != time.Millisecond || snap.P95Latency() != 5*time.Millisecond {
		t.Fatalf("unexpected latency values")
	}
	if snap.AllocBytes() != 512 {
		t.Fatalf("unexpected allocation bytes")
	}
}
