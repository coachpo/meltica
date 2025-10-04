package wsinfra

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	wsEndpoint         = "wss://ws-feed.exchange.coinbase.com"
	wsHandshakeTimeout = 10 * time.Second
)

// Subscription delivers raw websocket frames for upper layers.
type Subscription struct {
	conn *websocket.Conn
	raw  chan RawMessage
	err  chan error
}

// RawMessage carries websocket bytes alongside their receive timestamp.
type RawMessage struct {
	Data []byte
	At   time.Time
}

func newSubscription(conn *websocket.Conn) *Subscription {
	return &Subscription{
		conn: conn,
		raw:  make(chan RawMessage, 1024),
		err:  make(chan error, 1),
	}
}

func (s *Subscription) Raw() <-chan RawMessage { return s.raw }
func (s *Subscription) Err() <-chan error      { return s.err }
func (s *Subscription) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Client manages Coinbase websocket connections at Level 1.
type Client struct{}

// NewClient constructs a websocket infrastructure client.
func NewClient() *Client { return &Client{} }

// SubscribePublic dials the websocket endpoint and sends the provided payloads.
func (c *Client) SubscribePublic(ctx context.Context, payloads []string) (*Subscription, error) {
	if len(payloads) == 0 {
		return nil, fmt.Errorf("coinbase/wsinfra: no payloads provided")
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, wsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	if err := sendPayloads(conn, payloads); err != nil {
		conn.Close()
		return nil, err
	}
	sub := newSubscription(conn)
	go c.readLoop(ctx, sub)
	return sub, nil
}

// SubscribePrivate reuses the same endpoint for authenticated subscriptions.
func (c *Client) SubscribePrivate(ctx context.Context, payloads []string) (*Subscription, error) {
	if len(payloads) == 0 {
		return nil, fmt.Errorf("coinbase/wsinfra: no payloads provided")
	}
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, wsEndpoint, nil)
	if err != nil {
		return nil, err
	}
	if err := sendPayloads(conn, payloads); err != nil {
		conn.Close()
		return nil, err
	}
	sub := newSubscription(conn)
	go c.readLoop(ctx, sub)
	return sub, nil
}

func sendPayloads(conn *websocket.Conn, payloads []string) error {
	for _, payload := range payloads {
		data := strings.TrimSpace(payload)
		if data == "" {
			continue
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(data)); err != nil {
			return err
		}
	}
	return nil
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
		msg := RawMessage{Data: data, At: time.Now().UTC()}
		select {
		case sub.raw <- msg:
		case <-ctx.Done():
			return
		}
	}
}
