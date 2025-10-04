package wsinfra

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/coachpo/meltica/exchanges/binance/internal"
	"github.com/gorilla/websocket"
)

const (
	defaultPublicStreamURL  = "wss://stream.binance.com:9443/stream"
	defaultPrivateStreamURL = "wss://stream.binance.com:9443/ws"
	defaultHandshakeTimeout = 10 * time.Second
)

type Config struct {
	PublicURL        string
	PrivateURL       string
	HandshakeTimeout time.Duration
}

func (c Config) withDefaults() Config {
	if strings.TrimSpace(c.PublicURL) == "" {
		c.PublicURL = defaultPublicStreamURL
	}
	if strings.TrimSpace(c.PrivateURL) == "" {
		c.PrivateURL = defaultPrivateStreamURL
	}
	if c.HandshakeTimeout <= 0 {
		c.HandshakeTimeout = defaultHandshakeTimeout
	}
	return c
}

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
type Client struct {
	cfg Config
}

// NewClient constructs a websocket infrastructure client.
func NewClient(cfgs ...Config) *Client {
	var cfg Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	return &Client{cfg: cfg.withDefaults()}
}

// SubscribePublic dials the Binance combined stream endpoint for the requested raw streams.
func (c *Client) SubscribePublic(ctx context.Context, streams []string) (*Subscription, error) {
	if len(streams) == 0 {
		return nil, internal.Invalid("wsinfra: no streams provided")
	}
	q := url.Values{}
	q.Set("streams", strings.Join(streams, "/"))
	endpoint := c.cfg.PublicURL + "?" + q.Encode()
	dialer := websocket.Dialer{HandshakeTimeout: c.cfg.HandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, internal.WrapNetwork(err, "subscribe public streams")
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
		return nil, internal.Invalid("wsinfra: empty listen key")
	}
	endpoint := fmt.Sprintf("%s/%s", c.cfg.PrivateURL, listenKey)
	dialer := websocket.Dialer{HandshakeTimeout: c.cfg.HandshakeTimeout}
	conn, _, err := dialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return nil, internal.WrapNetwork(err, "subscribe private stream")
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
			wrapped := internal.WrapNetwork(err, "websocket read failure")
			select {
			case sub.err <- wrapped:
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
