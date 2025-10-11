package session_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/coachpo/meltica/lib/ws-routing/connection"
	"github.com/coachpo/meltica/lib/ws-routing/handler"
	"github.com/coachpo/meltica/lib/ws-routing/internal"
	"github.com/coachpo/meltica/lib/ws-routing/router"
	"github.com/coachpo/meltica/lib/ws-routing/session"
	"github.com/coachpo/meltica/lib/ws-routing/telemetry"
)

type transportStub struct {
	mu         sync.Mutex
	events     []internal.Event
	closed     bool
	closeCount int
	queue      chan internal.Event
}

func newTransportStub() *transportStub {
	return &transportStub{queue: make(chan internal.Event, 8)}
}

func (t *transportStub) Receive() <-chan internal.Event {
	return t.queue
}

func (t *transportStub) Send(event internal.Event) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return errors.New("transport closed")
	}
	t.events = append(t.events, event)
	return nil
}

func (t *transportStub) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.closeCount++
	if t.closed {
		return nil
	}
	t.closed = true
	close(t.queue)
	return nil
}

func TestNewSessionValidation(t *testing.T) {
	registry := handler.NewRegistry()
	unitRouter, err := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	if err != nil {
		t.Fatalf("create router: %v", err)
	}
	_, err = session.New(session.Options{})
	if err == nil {
		t.Fatal("expected error for missing options")
	}
	_, err = session.New(session.Options{ID: "abc", Transport: newTransportStub()})
	if err == nil {
		t.Fatal("expected error for missing router")
	}
	_, err = session.New(session.Options{ID: "abc", Transport: newTransportStub(), Router: unitRouter})
	if err != nil {
		t.Fatalf("unexpected error for valid options: %v", err)
	}
}

func TestSessionStartLifecycle(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-1", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if got := unit.Status(); got != internal.StatusRunning {
		t.Fatalf("expected running status, got %s", got)
	}
	if unit.StartedAt().IsZero() {
		t.Fatal("expected started timestamp to be set")
	}
	if err := unit.Close(ctx); err != nil {
		t.Fatalf("close session: %v", err)
	}
}

func TestSessionDoubleStartFails(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-2", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := unit.Start(ctx); err == nil {
		t.Fatal("expected error on double start")
	}
	_ = unit.Close(ctx)
}

func TestSessionSubscribeBeforeStart(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-3", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Subscribe(ctx, "BTC-USD"); err != nil {
		t.Fatalf("subscribe before start: %v", err)
	}
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	transport.mu.Lock()
	if len(transport.events) != 1 {
		t.Fatalf("expected subscription event to be sent on start, got %d", len(transport.events))
	}
	if transport.events[0].Type != "routing.subscribe" {
		t.Fatalf("unexpected event type %s", transport.events[0].Type)
	}
	if string(transport.events[0].Payload) != "BTC-USD" {
		t.Fatalf("unexpected payload %s", string(transport.events[0].Payload))
	}
	transport.mu.Unlock()
	_ = unit.Close(ctx)
}

func TestSessionSubscribeAfterStartSendsImmediately(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-4", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := unit.Subscribe(ctx, "ETH-USD"); err != nil {
		t.Fatalf("subscribe after start: %v", err)
	}
	transport.mu.Lock()
	if len(transport.events) != 1 {
		t.Fatalf("expected immediate subscription event, got %d", len(transport.events))
	}
	transport.mu.Unlock()
	_ = unit.Close(ctx)
}

func TestSessionUnsubscribeUnknown(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-5", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Unsubscribe(ctx, "UNKNOWN"); err == nil {
		t.Fatal("expected error when unsubscribing unknown topic")
	}
}

func TestSessionCloseIdempotent(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-6", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	if err := unit.Close(ctx); err != nil {
		t.Fatalf("close session: %v", err)
	}
	if err := unit.Close(ctx); err != nil {
		t.Fatalf("close session again: %v", err)
	}
	transport.mu.Lock()
	if transport.closeCount != 1 {
		t.Fatalf("expected close invoked once, got %d", transport.closeCount)
	}
	transport.mu.Unlock()
}

func TestSessionConcurrentCloses(t *testing.T) {
	transport := newTransportStub()
	registry := handler.NewRegistry()
	unitRouter, _ := router.New(router.Options{Registry: registry, Logger: telemetry.NewNoop(), BufferSize: 0})
	unit, err := session.New(session.Options{ID: "sess-7", Transport: transport, Router: unitRouter})
	if err != nil {
		t.Fatalf("new session: %v", err)
	}
	ctx := context.Background()
	if err := unit.Start(ctx); err != nil {
		t.Fatalf("start session: %v", err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = unit.Close(ctx)
		}()
	}
	wg.Wait()
}

var _ connection.AbstractTransport = (*transportStub)(nil)
