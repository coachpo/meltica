package wsinfra

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	publicWSURL        = "wss://ws.kraken.com/v2"
	privateWSURL       = "wss://ws-auth.kraken.com"
	wsHandshakeTimeout = 10 * time.Second
)

// Subscription delivers raw websocket frames for higher layers.
type Subscription struct {
	conn *websocket.Conn
	raw  chan RawMessage
	err  chan error
}

// RawMessage represents a websocket payload with reception timestamp.
type RawMessage struct {
	Data []byte
	At   time.Time
}

// Raw exposes the channel with raw websocket messages.
func (s *Subscription) Raw() <-chan RawMessage { return s.raw }

// Err exposes the channel carrying websocket errors.
func (s *Subscription) Err() <-chan error { return s.err }

// Close terminates the underlying websocket connection.
func (s *Subscription) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Client manages Kraken websocket connections (Level 1 infrastructure).
type Client struct{}

// NewClient constructs a websocket infrastructure client.
func NewClient() *Client { return &Client{} }

// SubscribePublic connects to the Kraken public websocket and sends the provided subscription payloads.
func (c *Client) SubscribePublic(ctx context.Context, payloads []string) (*Subscription, error) {
	if len(payloads) == 0 {
		return nil, fmt.Errorf("kraken/wsinfra: no subscription payloads provided")
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, publicWSURL, nil)
	if err != nil {
		return nil, err
	}
	for _, payload := range payloads {
		payload = strings.TrimSpace(payload)
		if payload == "" {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(payload)); err != nil {
			conn.Close()
			return nil, err
		}
	}
	sub := newSubscription(conn)
	go c.readLoop(ctx, sub)
	return sub, nil
}

// SubscribePrivate connects to the Kraken private websocket and registers default private channels.
func (c *Client) SubscribePrivate(ctx context.Context, token string) (*Subscription, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("kraken/wsinfra: empty token for private subscription")
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, privateWSURL, nil)
	if err != nil {
		return nil, err
	}
	if err := sendSubscribe(conn, token, "ownTrades"); err != nil {
		conn.Close()
		return nil, err
	}
	if err := sendSubscribe(conn, token, "openOrders"); err != nil {
		conn.Close()
		return nil, err
	}
	sub := newSubscription(conn)
	go c.readLoop(ctx, sub)
	return sub, nil
}

func newSubscription(conn *websocket.Conn) *Subscription {
	return &Subscription{
		conn: conn,
		raw:  make(chan RawMessage, 1024),
		err:  make(chan error, 1),
	}
}

func sendSubscribe(conn *websocket.Conn, token, channel string) error {
	if token == "" {
		return fmt.Errorf("kraken/wsinfra: missing token")
	}
	payload := map[string]any{
		"event": "subscribe",
		"subscription": map[string]any{
			"name":  channel,
			"token": token,
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Client) readLoop(ctx context.Context, sub *Subscription) {
	defer close(sub.raw)
	defer close(sub.err)

	for {
		if ctx.Err() != nil {
			return
		}
		_, data, err := sub.conn.ReadMessage()
		if err != nil {
			select {
			case sub.err <- err:
			default:
			}
			return
		}
		msg := RawMessage{Data: data, At: time.Now()}
		select {
		case sub.raw <- msg:
		case <-ctx.Done():
			return
		}
	}
}
