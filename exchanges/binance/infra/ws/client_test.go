package ws

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/errs"
)

func TestConfigWithDefaults(t *testing.T) {
	cfg := Config{}.withDefaults()
	if cfg.PublicURL == "" || cfg.PrivateURL == "" {
		t.Fatalf("expected default URLs to be populated")
	}
	if cfg.HandshakeTimeout <= 0 {
		t.Fatalf("expected default handshake timeout")
	}
}

func TestClientConnectContextCancelled(t *testing.T) {
	client := NewClient()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := client.Connect(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}

func TestClientSubscribeValidation(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	if _, err := client.Subscribe(ctx); err == nil {
		t.Fatalf("expected error when topics missing")
	}
	topics := []coretransport.StreamTopic{{Scope: coretransport.StreamScopePublic, Name: "trade"}, {Scope: coretransport.StreamScopePrivate, Name: "listen"}}
	if _, err := client.Subscribe(ctx, topics...); err == nil {
		t.Fatalf("expected error on mixed scopes")
	}
	if _, err := client.Subscribe(ctx, coretransport.StreamTopic{Scope: coretransport.StreamScopePrivate, Name: ""}); err == nil {
		t.Fatalf("expected error on empty listen key")
	}
}

func TestClientSubscribePublicDeliversMessage(t *testing.T) {
	payload := []byte(`{"stream":"trade"}`)
	srv, wsURL := newWebsocketServer(t, func(conn *websocket.Conn) {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			t.Errorf("write message: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	})
	defer srv.Close()

	client := NewClient(Config{PublicURL: wsURL, PrivateURL: wsURL, HandshakeTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, coretransport.StreamTopic{Scope: coretransport.StreamScopePublic, Name: "btcusdt@trade"})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Close()

	select {
	case msg := <-sub.Messages():
		if !bytes.Equal(msg.Data, payload) {
			t.Fatalf("unexpected payload %q", msg.Data)
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for message")
	}

	select {
	case err := <-sub.Errors():
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	default:
	}
}

func TestClientSubscribePrivateDeliversErrorOnConnectionClose(t *testing.T) {
	srv, wsURL := newWebsocketServer(t, func(conn *websocket.Conn) {})
	defer srv.Close()

	client := NewClient(Config{PublicURL: wsURL, PrivateURL: wsURL, HandshakeTimeout: time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	sub, err := client.Subscribe(ctx, coretransport.StreamTopic{Scope: coretransport.StreamScopePrivate, Name: "listen-key"})
	if err != nil {
		t.Fatalf("subscribe private: %v", err)
	}
	defer sub.Close()

	select {
	case err := <-sub.Errors():
		var e *errs.E
		if err == nil || !errors.As(err, &e) || e.Code != errs.CodeNetwork {
			t.Fatalf("expected network error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("expected error from subscription")
	}
}

func TestClientUnsubscribeValidation(t *testing.T) {
	client := NewClient()
	if err := client.Unsubscribe(context.Background(), nil); err == nil {
		t.Fatalf("expected error on nil subscription")
	}
}

func TestClientPublishNotSupported(t *testing.T) {
	client := NewClient()
	err := client.Publish(context.Background(), coretransport.StreamMessage{})
	if err == nil {
		t.Fatalf("expected not supported error")
	}
	var e *errs.E
	if !errors.As(err, &e) || e.Code != errs.CodeExchange || e.Canonical != errs.CanonicalCapabilityMissing {
		t.Fatalf("expected canonical capability missing error, got %v", err)
	}
}

func TestClientHandleErrorWrapping(t *testing.T) {
	client := NewClient()
	ctx := context.Background()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if err := client.HandleError(cancelled, context.Canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected passthrough cancellation, got %v", err)
	}

	err := client.HandleError(ctx, errors.New("boom"))
	var e *errs.E
	if !errors.As(err, &e) || e.Code != errs.CodeNetwork {
		t.Fatalf("expected wrapped network error, got %v", err)
	}
}

func TestComposeStreamName(t *testing.T) {
	topic := coretransport.StreamTopic{Name: "depth", Params: map[string]string{"limit": "100"}}
	if got := composeStreamName(topic); got != "depth?limit=100" {
		t.Fatalf("unexpected composed stream: %s", got)
	}
	if got := composeStreamName(coretransport.StreamTopic{Name: "trades"}); got != "trades" {
		t.Fatalf("expected bare stream name, got %s", got)
	}
}

func newWebsocketServer(t *testing.T, handler func(*websocket.Conn)) (*httptest.Server, string) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		go func() {
			defer conn.Close()
			handler(conn)
		}()
	}))
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}
