package ws

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/errs"
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

// Subscription delivers raw websocket frames for further routing in upper layers.
type Subscription struct {
	conn *websocket.Conn
	raw  chan coreexchange.RawMessage
	err  chan error
}

// Messages returns the channel carrying raw websocket payloads.
func (s *Subscription) Messages() <-chan coreexchange.RawMessage { return s.raw }

// Errors returns the channel carrying websocket errors.
func (s *Subscription) Errors() <-chan error { return s.err }

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

// Connect satisfies the exchange.Connection interface; websocket connections are opened per subscription.
func (c *Client) Connect(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return nil
}

// Close releases resources associated with the client. Binance keeps no shared websocket pool, so this is a no-op.
func (c *Client) Close() error { return nil }

// Subscribe implements the coreexchange.StreamClient contract.
func (c *Client) Subscribe(ctx context.Context, topics ...coreexchange.StreamTopic) (coreexchange.StreamSubscription, error) {
	if len(topics) == 0 {
		return nil, internal.Invalid("wsinfra: no topics provided")
	}
	scope := topics[0].Scope
	if scope == "" {
		scope = coreexchange.StreamScopePublic
	}
	for _, t := range topics {
		currentScope := t.Scope
		if currentScope == "" {
			currentScope = coreexchange.StreamScopePublic
		}
		if currentScope != scope {
			return nil, internal.Invalid("wsinfra: mixed stream scopes not supported")
		}
	}
	switch scope {
	case coreexchange.StreamScopePublic:
		streams := make([]string, len(topics))
		for i, topic := range topics {
			streams[i] = composeStreamName(topic)
		}
		return c.SubscribePublic(ctx, streams)
	case coreexchange.StreamScopePrivate:
		if len(topics) != 1 {
			return nil, internal.Invalid("wsinfra: private streams expect a single listen key")
		}
		listenKey := strings.TrimSpace(topics[0].Name)
		if listenKey == "" {
			return nil, internal.Invalid("wsinfra: empty listen key")
		}
		return c.SubscribePrivate(ctx, listenKey)
	default:
		return nil, internal.Invalid("wsinfra: unsupported scope %q", scope)
	}
}

// Unsubscribe closes the websocket connection associated with the subscription.
func (c *Client) Unsubscribe(ctx context.Context, sub coreexchange.StreamSubscription, topics ...coreexchange.StreamTopic) error {
	if sub == nil {
		return internal.Invalid("wsinfra: nil subscription")
	}
	return sub.Close()
}

// Publish returns a standardised not-supported error because Binance websocket APIs do not accept outbound commands via this client.
func (c *Client) Publish(ctx context.Context, message coreexchange.StreamMessage) error {
	return errs.NotSupported("binance websocket publish not supported")
}

// HandleError normalises websocket transport failures.
func (c *Client) HandleError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	var e *errs.E
	if errors.As(err, &e) {
		return err
	}
	return internal.WrapNetwork(err, "wsinfra: transport failure")
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
		raw:  make(chan coreexchange.RawMessage, 1024),
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
		raw:  make(chan coreexchange.RawMessage, 1024),
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
		msg := coreexchange.RawMessage{Data: data, At: time.Now()}
		select {
		case sub.raw <- msg:
		case <-ctx.Done():
			_ = conn.Close()
			return
		}
	}
}

func composeStreamName(topic coreexchange.StreamTopic) string {
	name := strings.TrimSpace(topic.Name)
	if len(topic.Params) == 0 {
		return name
	}
	values := url.Values{}
	for k, v := range topic.Params {
		if k == "" {
			continue
		}
		values.Set(k, v)
	}
	if encoded := values.Encode(); encoded != "" {
		return fmt.Sprintf("%s?%s", name, encoded)
	}
	return name
}
