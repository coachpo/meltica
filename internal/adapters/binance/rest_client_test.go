package binance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/coachpo/meltica/internal/schema"
)

type mockFetcher struct {
	data []byte
	err  error
}

func (m *mockFetcher) Fetch(_ context.Context, _ string) ([]byte, error) {
	return m.data, m.err
}

type mockParser struct {
	events []*schema.Event
	err    error
}

func (m *mockParser) ParseSnapshot(_ context.Context, _ string, _ []byte, _ time.Time) ([]*schema.Event, error) {
	return m.events, m.err
}

func TestRESTClient_Poll_EmptyPollers(t *testing.T) {
	fetcher := &mockFetcher{}
	parser := &mockParser{}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	ctx := context.Background()
	events, errs := client.Poll(ctx, nil)
	
	if events == nil {
		t.Error("expected non-nil events channel")
	}
	if errs == nil {
		t.Error("expected non-nil errors channel")
	}
	
	select {
	case _, ok := <-events:
		if ok {
			t.Error("expected closed events channel")
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("timeout waiting for channel close")
	}
}

func TestRESTClient_Poll_FetchError(t *testing.T) {
	fetcher := &mockFetcher{err: errors.New("fetch error")}
	parser := &mockParser{}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{
			Name:     "test",
			Endpoint: "/test",
			Interval: 50 * time.Millisecond,
			Parser:   "orderbook",
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	_, errs := client.Poll(ctx, pollers)
	
	select {
	case err := <-errs:
		if err == nil {
			t.Error("expected error from fetcher")
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for error")
	}
}

func TestRESTClient_Poll_ParseError(t *testing.T) {
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{err: errors.New("parse error")}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{
			Name:     "test",
			Endpoint: "/test",
			Interval: 50 * time.Millisecond,
			Parser:   "orderbook",
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	_, errs := client.Poll(ctx, pollers)
	
	select {
	case err := <-errs:
		if err == nil {
			t.Error("expected error from parser")
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for error")
	}
}

func TestRESTClient_Poll_Success(t *testing.T) {
	evt := &schema.Event{
		EventID:  "test-1",
		Provider: "binance",
		Symbol:   "BTC-USDT",
		Type:     schema.EventTypeBookSnapshot,
	}
	
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{events: []*schema.Event{evt}}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{
			Name:     "test",
			Endpoint: "/test",
			Interval: 50 * time.Millisecond,
			Parser:   "orderbook",
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	events, _ := client.Poll(ctx, pollers)
	
	select {
	case receivedEvt := <-events:
		if receivedEvt == nil {
			t.Error("expected non-nil event")
		}
		if receivedEvt.EventID != "test-1" {
			t.Errorf("expected event ID test-1, got %s", receivedEvt.EventID)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestRESTClient_Poll_DefaultInterval(t *testing.T) {
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{events: []*schema.Event{{EventID: "test"}}}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{
			Name:     "test",
			Endpoint: "/test",
			Interval: 0,
			Parser:   "orderbook",
		},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	events, _ := client.Poll(ctx, pollers)
	
	select {
	case <-events:
	case <-time.After(2 * time.Second):
		t.Error("default interval should be 1 second")
	}
}

func TestRESTClient_Poll_MultiplePollers(t *testing.T) {
	evt1 := &schema.Event{EventID: "test-1"}
	
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{Name: "test1", Endpoint: "/test1", Interval: 50 * time.Millisecond, Parser: "orderbook"},
		{Name: "test2", Endpoint: "/test2", Interval: 50 * time.Millisecond, Parser: "orderbook"},
	}
	
	callCount := 0
	parser.events = []*schema.Event{evt1}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	events, _ := client.Poll(ctx, pollers)
	
	for {
		select {
		case evt := <-events:
			if evt != nil {
				callCount++
			}
		case <-ctx.Done():
			if callCount < 2 {
				t.Errorf("expected at least 2 events from multiple pollers, got %d", callCount)
			}
			return
		case <-time.After(500 * time.Millisecond):
			return
		}
	}
}

func TestRESTClient_Poll_ContextCancellation(t *testing.T) {
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{events: []*schema.Event{{EventID: "test"}}}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{Name: "test", Endpoint: "/test", Interval: 50 * time.Millisecond, Parser: "orderbook"},
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	events, _ := client.Poll(ctx, pollers)
	
	cancel()
	
	select {
	case _, ok := <-events:
		if ok {
			time.Sleep(200 * time.Millisecond)
			if _, stillOpen := <-events; stillOpen {
				t.Error("expected channel to close after context cancellation")
			}
		}
	case <-time.After(500 * time.Millisecond):
	}
}

func TestRESTClient_Poll_NilEvent(t *testing.T) {
	fetcher := &mockFetcher{data: []byte(`{"test": "data"}`)}
	parser := &mockParser{events: []*schema.Event{nil, {EventID: "test"}}}
	client := NewRESTClient(fetcher, parser, time.Now)
	
	pollers := []RESTPoller{
		{Name: "test", Endpoint: "/test", Interval: 50 * time.Millisecond, Parser: "orderbook"},
	}
	
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	
	events, _ := client.Poll(ctx, pollers)
	
	select {
	case evt := <-events:
		if evt == nil {
			t.Error("expected non-nil event (nil events should be filtered)")
		}
		if evt.EventID != "test" {
			t.Errorf("expected event ID test, got %s", evt.EventID)
		}
	case <-time.After(300 * time.Millisecond):
		t.Error("timeout waiting for event")
	}
}

func TestNewRESTClient_NilClock(t *testing.T) {
	fetcher := &mockFetcher{}
	parser := &mockParser{}
	client := NewRESTClient(fetcher, parser, nil)
	
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.clock == nil {
		t.Error("expected clock to be set to default")
	}
}
