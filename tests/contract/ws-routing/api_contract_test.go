package wsrouting_test

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
	"github.com/coachpo/meltica/internal/observability"
	"github.com/coachpo/meltica/lib/ws-routing"
)

func TestInitRequiresSessionID(t *testing.T) {
	ctx := context.Background()
	options := validOptions()
	options.SessionID = ""

	_, err := wsrouting.Init(ctx, options)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)
}

func TestInitConfiguresSessionState(t *testing.T) {
	ctx := context.Background()
	options := validOptions()

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)
	require.NotNil(t, session)
	require.Equal(t, "session-1", session.ID())
	require.Equal(t, wsrouting.StateInitialized, session.State())
	require.Equal(t, options.Backoff, session.Backoff())
}

func TestStartDialerInvokedOnce(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{conn: &stubConnection{}}
	options := validOptions()
	options.Dialer = dialer

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)

	require.Equal(t, wsrouting.StateInitialized, session.State())

	require.NoError(t, wsrouting.Start(ctx, session))
	require.Equal(t, 1, dialer.calls)
	require.Equal(t, wsrouting.StateStreaming, session.State())
}

func TestSubscribeQueuesBeforeStart(t *testing.T) {
	ctx := context.Background()
	conn := &stubConnection{}
	dialer := &stubDialer{conn: conn}
	options := validOptions()
	options.Dialer = dialer

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)

	spec := wsrouting.SubscriptionSpec{Exchange: "binance", Channel: "book", Symbols: []string{"BTC-USDT"}}

	require.NoError(t, wsrouting.Subscribe(ctx, session, spec))
	require.Empty(t, conn.subscriptions)

	require.NoError(t, wsrouting.Start(ctx, session))
	require.Equal(t, []wsrouting.SubscriptionSpec{spec}, conn.subscriptions)
}

func TestSubscribeInvokesConnection(t *testing.T) {
	ctx := context.Background()
	conn := &stubConnection{}
	dialer := &stubDialer{conn: conn}
	options := validOptions()
	options.Dialer = dialer

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)

	require.NoError(t, wsrouting.Start(ctx, session))

	spec := wsrouting.SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"ETH-USDT"}}
	require.NoError(t, wsrouting.Subscribe(ctx, session, spec))
	require.Equal(t, []wsrouting.SubscriptionSpec{spec}, conn.subscriptions)
}

func TestPublishRunsMiddlewareInOrder(t *testing.T) {
	ctx := context.Background()
	conn := &stubConnection{}
	publisher := &stubPublisher{}
	dialer := &stubDialer{conn: conn}
	options := validOptions()
	options.Dialer = dialer
	options.Publish = publisher.Publish

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)
	require.NoError(t, wsrouting.Start(ctx, session))

	first := func(ctx context.Context, msg *wsrouting.Message) (*wsrouting.Message, error) {
		msg.Metadata["first"] = "true"
		return msg, nil
	}
	second := func(ctx context.Context, msg *wsrouting.Message) (*wsrouting.Message, error) {
		msg.Metadata["second"] = msg.Metadata["first"]
		return msg, nil
	}

	require.NoError(t, wsrouting.UseMiddleware(session, first))
	require.NoError(t, wsrouting.UseMiddleware(session, second))

	message := &wsrouting.Message{Type: "trade", Payload: map[string]any{"price": 10}, Metadata: map[string]string{}}
	require.NoError(t, wsrouting.Publish(ctx, session, message))

	require.Len(t, publisher.messages, 1)
	require.Equal(t, "true", publisher.messages[0].Metadata["second"])
}

func TestPublishStopsOnMiddlewareError(t *testing.T) {
	ctx := context.Background()
	publisher := &stubPublisher{}
	dialer := &stubDialer{conn: &stubConnection{}}
	options := validOptions()
	options.Dialer = dialer
	options.Publish = publisher.Publish

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)
	require.NoError(t, wsrouting.Start(ctx, session))

	call := 0
	errMiddleware := func(ctx context.Context, msg *wsrouting.Message) (*wsrouting.Message, error) {
		call++
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("middleware failure"))
	}

	require.NoError(t, wsrouting.UseMiddleware(session, errMiddleware))

	message := &wsrouting.Message{Type: "book", Payload: map[string]any{"depth": 1}, Metadata: map[string]string{}}

	err = wsrouting.Publish(ctx, session, message)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeExchange, e.Code)
	require.Equal(t, 1, call)
	require.Empty(t, publisher.messages)
}

func TestRouteRawParsesAndPublishes(t *testing.T) {
	ctx := context.Background()
	publisher := &stubPublisher{}
	dialer := &stubDialer{conn: &stubConnection{}}
	recorded := &recordingParser{}
	options := validOptions()
	options.Dialer = dialer
	options.Publish = publisher.Publish
	options.Parser = recorded

	session, err := wsrouting.Init(ctx, options)
	require.NoError(t, err)
	require.NoError(t, wsrouting.Start(ctx, session))

	raw := []byte(`{"type":"noop"}`)
	require.NoError(t, wsrouting.RouteRaw(ctx, session, raw))

	require.Equal(t, 1, recorded.calls)
	require.Equal(t, raw, recorded.last)
	require.Len(t, publisher.messages, 1)
}

func TestUseMiddlewareRejectsNil(t *testing.T) {
	ctx := context.Background()
	session, err := wsrouting.Init(ctx, validOptions())
	require.NoError(t, err)

	err = wsrouting.UseMiddleware(session, nil)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)
}

func validOptions() wsrouting.Options {
	return wsrouting.Options{
		SessionID: "session-1",
		Dialer:    &stubDialer{conn: &stubConnection{}},
		Parser:    stubParser{},
		Publish:   (&stubPublisher{}).Publish,
		Backoff: wsrouting.BackoffConfig{
			Initial:    100 * time.Millisecond,
			Max:        time.Second,
			Multiplier: big.NewRat(3, 2),
			Jitter:     20,
		},
		Logger: observability.Log(),
	}
}

type stubDialer struct {
	conn  wsrouting.Connection
	err   error
	calls int
}

func (d *stubDialer) Dial(ctx context.Context, opts wsrouting.DialOptions) (wsrouting.Connection, error) {
	d.calls++
	if d.err != nil {
		return nil, d.err
	}
	if d.conn == nil {
		return nil, errors.New("no connection")
	}
	return d.conn, nil
}

type stubConnection struct {
	subscriptions []wsrouting.SubscriptionSpec
}

func (c *stubConnection) Subscribe(ctx context.Context, spec wsrouting.SubscriptionSpec) error {
	c.subscriptions = append(c.subscriptions, spec)
	return nil
}

func (c *stubConnection) Close(context.Context) error { return nil }

type stubPublisher struct {
	messages []*wsrouting.Message
}

func (p *stubPublisher) Publish(ctx context.Context, msg *wsrouting.Message) error {
	p.messages = append(p.messages, msg)
	return nil
}

type stubParser struct{}

func (stubParser) Parse(ctx context.Context, raw []byte) (*wsrouting.Message, error) {
	return &wsrouting.Message{Type: "noop", Payload: map[string]any{}, Metadata: map[string]string{}}, nil
}

type recordingParser struct {
	calls int
	last  []byte
}

func (r *recordingParser) Parse(ctx context.Context, raw []byte) (*wsrouting.Message, error) {
	r.calls++
	r.last = append(r.last[:0], raw...)
	return &wsrouting.Message{Type: "noop", Payload: map[string]any{}, Metadata: map[string]string{}}, nil
}
