package wsinfra

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

const (
	publicStreamURL    = "wss://stream.binance.com:9443/stream"
	privateStreamURL   = "wss://stream.binance.com:9443/ws"
	wsHandshakeTimeout = 10 * time.Second
)

// RawMessage represents a raw websocket payload with its receive timestamp.
type RawMessage struct {
	Data []byte
	At   time.Time
}

// Subscription delivers raw websocket frames for further routing in upper layers.
type Subscription struct {
	conn *websocket.Conn
	raw  chan RawMessage
	err  chan error
}

// Raw returns the channel carrying raw websocket payloads.
func (s *Subscription) Raw() <-chan RawMessage { return s.raw }

// Err returns the channel carrying websocket errors.
func (s *Subscription) Err() <-chan error { return s.err }

// Close terminates the websocket connection.
func (s *Subscription) Close() error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

// Client manages Binance websocket connections (Level 1 infrastructure).
type Client struct{}

// NewClient constructs a websocket infrastructure client.
func NewClient() *Client { return &Client{} }

// SubscribePublic dials the Binance combined stream endpoint for the requested raw streams.
func (c *Client) SubscribePublic(ctx context.Context, streams []string) (*Subscription, error) {
	if len(streams) == 0 {
		return nil, fmt.Errorf("binance/wsinfra: no streams provided")
	}
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	endpoint := publicStreamURL + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := &Subscription{
		conn: conn,
		raw:  make(chan RawMessage, 1024),
		err:  make(chan error, 1),
	}
	go c.readLoop(ctx, conn, sub)
	return sub, nil
}

// SubscribePrivate dials the Binance private listen key stream.
func (c *Client) SubscribePrivate(ctx context.Context, listenKey string) (*Subscription, error) {
	if strings.TrimSpace(listenKey) == "" {
		return nil, fmt.Errorf("binance/wsinfra: empty listen key")
	}
	endpoint := fmt.Sprintf("%s/%s", privateStreamURL, listenKey)
	dialer := websocket.Dialer{HandshakeTimeout: wsHandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, err
	}
	sub := &Subscription{
		conn: conn,
		raw:  make(chan RawMessage, 1024),
		err:  make(chan error, 1),
	}
	go c.readLoop(ctx, conn, sub)
	return sub, nil
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn, sub *Subscription) {
	defer close(sub.raw)
	defer close(sub.err)
	for {
		select {
		case <-ctx.Done():
			_ = conn.Close()
			return
		default:
		}
		_, data, err := conn.ReadMessage()
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
			_ = conn.Close()
			return
		}
	}
}
