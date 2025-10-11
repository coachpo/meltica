package integration

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	wsrouting "github.com/coachpo/meltica/lib/ws-routing"
)

func TestQuickstartSmokeReconnectFlow(t *testing.T) {
	ctx := context.Background()
	dialer := &smokeDialer{failUntil: 1}
	publisher := &smokePublisher{}
	parser := smokeParser{}

	backoff := wsrouting.BackoffConfig{
		Initial:    5 * time.Millisecond,
		Max:        20 * time.Millisecond,
		Multiplier: big.NewRat(3, 2),
		Jitter:     0,
	}

	session, err := wsrouting.Init(ctx, wsrouting.Options{
		SessionID: "quickstart-smoke",
		Dialer:    dialer,
		Parser:    parser,
		Publish:   publisher.Publish,
		Backoff:   backoff,
	})
	require.NoError(t, err)
	require.Equal(t, wsrouting.StateInitialized, session.State())

	spec := wsrouting.SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"BTC-USDT"}}
	require.NoError(t, wsrouting.Subscribe(ctx, session, spec))

	err = wsrouting.Start(ctx, session)
	require.Error(t, err, "first dial must surface failure")
	require.Equal(t, 1, dialer.Attempts())
	// Session should remain reusable after failure.
	require.Equal(t, wsrouting.StateInitialized, session.State())
	require.Equal(t, backoff.Initial, dialer.LastOptions().Backoff.Initial)

	dialer.failUntil = 0
	require.NoError(t, wsrouting.Start(ctx, session))
	require.Equal(t, 2, dialer.Attempts())
	require.Equal(t, wsrouting.StateStreaming, session.State())

	conn := dialer.Connection()
	require.NotNil(t, conn)
	require.Len(t, conn.subscriptions, 1)
	require.Equal(t, spec, conn.subscriptions[0])

	recorder := &smokeMiddleware{}
	require.NoError(t, wsrouting.UseMiddleware(session, recorder.Handler))

	manual := &wsrouting.Message{Type: "manual", Metadata: map[string]string{}}
	require.NoError(t, wsrouting.Publish(ctx, session, manual))

	require.NoError(t, wsrouting.RouteRaw(ctx, session, []byte(`{"payload":"auto"}`)))

	require.Equal(t, 2, publisher.Calls())
	msgs := publisher.Messages()
	require.Equal(t, "manual", msgs[0].Type)
	require.Equal(t, "{\"payload\":\"auto\"}", msgs[1].Metadata["raw"])
	require.Equal(t, 2, recorder.Calls())
	// Smoke connection should remain open for graceful closure.
	require.NoError(t, conn.Close(ctx))
}

type smokeDialer struct {
	mu        sync.Mutex
	failUntil int
	attempts  int
	lastOpts  wsrouting.DialOptions
	conn      *smokeConnection
}

func (d *smokeDialer) Dial(ctx context.Context, opts wsrouting.DialOptions) (wsrouting.Connection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.attempts++
	d.lastOpts = opts
	if d.attempts <= d.failUntil {
		return nil, errs.New("", errs.CodeNetwork, errs.WithMessage("transient dial failure"))
	}
	if d.conn == nil {
		d.conn = &smokeConnection{}
	}
	return d.conn, nil
}

func (d *smokeDialer) Attempts() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.attempts
}

func (d *smokeDialer) LastOptions() wsrouting.DialOptions {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.lastOpts
}

func (d *smokeDialer) Connection() *smokeConnection {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.conn
}

type smokeConnection struct {
	mu            sync.Mutex
	subscriptions []wsrouting.SubscriptionSpec
	closed        bool
}

func (c *smokeConnection) Subscribe(_ context.Context, spec wsrouting.SubscriptionSpec) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscriptions = append(c.subscriptions, spec)
	return nil
}

func (c *smokeConnection) Close(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	return nil
}

type smokeParser struct{}

func (smokeParser) Parse(_ context.Context, raw []byte) (*wsrouting.Message, error) {
	return &wsrouting.Message{Type: "auto", Metadata: map[string]string{"raw": string(raw)}}, nil
}

type smokePublisher struct {
	mu    sync.Mutex
	msgs  []*wsrouting.Message
	calls int
}

func (p *smokePublisher) Publish(_ context.Context, msg *wsrouting.Message) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	clone := *msg
	if clone.Metadata != nil {
		meta := make(map[string]string, len(clone.Metadata))
		for k, v := range clone.Metadata {
			meta[k] = v
		}
		clone.Metadata = meta
	}
	p.msgs = append(p.msgs, &clone)
	return nil
}

func (p *smokePublisher) Calls() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func (p *smokePublisher) Messages() []*wsrouting.Message {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*wsrouting.Message(nil), p.msgs...)
}

type smokeMiddleware struct {
	mu    sync.Mutex
	calls int
}

func (m *smokeMiddleware) Handler(ctx context.Context, msg *wsrouting.Message) (*wsrouting.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if msg.Metadata == nil {
		msg.Metadata = make(map[string]string)
	}
	msg.Metadata["middleware"] = "invoked"
	return msg, nil
}

func (m *smokeMiddleware) Calls() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}
