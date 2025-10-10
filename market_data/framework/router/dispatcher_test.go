package router

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/errs"
)

func TestRouterDispatcherDispatch(t *testing.T) {
	ctx := context.Background()
	metrics := NewRoutingMetrics()
	dispatcher := NewRouterDispatcher(ctx, metrics)
	reg := &ProcessorRegistration{MessageTypeID: "trade", Processor: newMockProcessor("trade"), Status: ProcessorStatusAvailable}
	inbox := dispatcher.Bind("trade", reg)

	done := make(chan struct{})
	go func() {
		if err := dispatcher.Dispatch("trade", []byte("hello")); err != nil {
			t.Errorf("dispatch failed: %v", err)
		}
		close(done)
	}()

	select {
	case payload := <-inbox:
		if string(payload) != "hello" {
			t.Fatalf("unexpected payload %q", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch blocked unexpectedly")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch goroutine did not finish")
	}
}

func TestRouterDispatcherBackpressure(t *testing.T) {
	ctx := context.Background()
	metrics := NewRoutingMetrics()
	dispatcher := NewRouterDispatcher(ctx, metrics)
	reg := &ProcessorRegistration{MessageTypeID: "trade", Processor: newMockProcessor("trade"), Status: ProcessorStatusAvailable}
	inbox := dispatcher.Bind("trade", reg)

	blocked := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(blocked)
		done <- dispatcher.Dispatch("trade", []byte("slow"))
	}()

	<-blocked
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-done:
		t.Fatalf("dispatch should block, got err %v", err)
	default:
	}

	received := make(chan []byte, 1)
	go func() {
		received <- <-inbox
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("dispatch returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not unblock after consumer ready")
	}

	if payload := <-received; string(payload) != "slow" {
		t.Fatalf("unexpected payload %q", payload)
	}

	snapshot := metrics.Snapshot()
	if snapshot.BackpressureEvents == 0 {
		t.Fatalf("expected backpressure events recorded")
	}
	if depth := snapshot.ChannelDepth["trade"]; depth != 0 {
		t.Fatalf("expected trade depth to return to zero, got %d", depth)
	}
}

func TestRouterDispatcherContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	dispatcher := NewRouterDispatcher(ctx, NewRoutingMetrics())
	reg := &ProcessorRegistration{MessageTypeID: "trade", Processor: newMockProcessor("trade"), Status: ProcessorStatusAvailable}
	dispatcher.Bind("trade", reg)

	done := make(chan error, 1)
	go func() {
		done <- dispatcher.Dispatch("trade", []byte("data"))
	}()

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected cancellation error")
		}
		var e *errs.E
		if !errors.As(err, &e) {
			t.Fatalf("expected errs.E, got %T", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("dispatch did not return after cancellation")
	}
}

func TestRouterDispatcherDefaultFallback(t *testing.T) {
	ctx := context.Background()
	dispatcher := NewRouterDispatcher(ctx, NewRoutingMetrics())
	reg := &ProcessorRegistration{MessageTypeID: "default", Processor: newMockProcessor("default"), Status: ProcessorStatusAvailable}
	inbox := dispatcher.BindDefault(reg)

	done := make(chan error, 1)
	go func() {
		done <- dispatcher.Dispatch("unknown", []byte("payload"))
	}()

	select {
	case payload := <-inbox:
		if string(payload) != "payload" {
			t.Fatalf("unexpected payload %q", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("fallback dispatch timed out")
	}

	if err := <-done; err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
}

func TestRouterDispatcherMissingWorker(t *testing.T) {
	ctx := context.Background()
	dispatcher := NewRouterDispatcher(ctx, NewRoutingMetrics())
	if err := dispatcher.Dispatch("trade", []byte("data")); err == nil {
		t.Fatal("expected error for missing worker")
	}
}
