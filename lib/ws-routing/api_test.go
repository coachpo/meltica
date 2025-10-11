package wsrouting

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/errs"
)

func TestInitValidatesInputs(t *testing.T) {
	_, err := Init(nil, Options{})
	require.Error(t, err)

	ctx := context.Background()
	_, err = Init(ctx, Options{SessionID: "", Dialer: &stubDialer{}, Parser: stubParser{}, Publish: func(context.Context, *Message) error { return nil }, Backoff: BackoffConfig{Initial: time.Millisecond}})
	require.Error(t, err)
}

func TestStartDialAndReplaySubscriptions(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{}
	publisher := &capturePublisher{}
	parser := stubParser{}
	options := Options{
		SessionID: "session-1",
		Dialer:    dialer,
		Parser:    parser,
		Publish:   publisher.Publish,
		Backoff: BackoffConfig{
			Initial:    50 * time.Millisecond,
			Max:        200 * time.Millisecond,
			Multiplier: big.NewRat(2, 1),
		},
	}
	session, err := Init(ctx, options)
	require.NoError(t, err)

	spec := SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"BTC-USDT"}}
	require.NoError(t, Subscribe(ctx, session, spec))
	require.Equal(t, StateInitialized, session.State())

	require.NoError(t, Start(ctx, session))
	require.Equal(t, StateStreaming, session.State())
	require.NotNil(t, dialer.lastConn)
	require.Equal(t, options.SessionID, dialer.lastOptions.SessionID)
	require.Len(t, dialer.lastConn.subscriptions, 1)
	require.Equal(t, spec, dialer.lastConn.subscriptions[0])

	msg := &Message{Type: "trade", Payload: map[string]any{"raw": []byte(`{"p":"1"}`)}, Metadata: map[string]string{"origin": "manual"}}
	require.NoError(t, Publish(ctx, session, msg))
	require.Len(t, publisher.messages, 1)
	require.Equal(t, "trade", publisher.messages[0].Type)

	require.NoError(t, RouteRaw(ctx, session, []byte(`{"e":"trade"}`)))
	require.Len(t, publisher.messages, 2)
}

func TestStartPropagatesSubscriptionErrors(t *testing.T) {
	ctx := context.Background()
	failing := &stubDialer{subscriptionErr: errs.New("", errs.CodeNetwork, errs.WithMessage("subscribe failed"))}
	options := Options{
		SessionID: "session-err",
		Dialer:    failing,
		Parser:    stubParser{},
		Publish:   func(context.Context, *Message) error { return nil },
		Backoff:   BackoffConfig{Initial: time.Millisecond},
	}
	session, err := Init(ctx, options)
	require.NoError(t, err)
	require.NoError(t, Subscribe(ctx, session, SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"BTC-USDT"}}))

	err = Start(ctx, session)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeNetwork, e.Code)
}

func TestSubscribeValidations(t *testing.T) {
	ctx := context.Background()
	session, err := Init(ctx, Options{
		SessionID: "session-sub",
		Dialer:    &stubDialer{},
		Parser:    stubParser{},
		Publish:   func(context.Context, *Message) error { return nil },
		Backoff:   BackoffConfig{Initial: time.Millisecond},
	})
	require.NoError(t, err)
	t.Run("nil session", func(t *testing.T) {
		err := Subscribe(ctx, nil, SubscriptionSpec{})
		require.Error(t, err)
	})
	t.Run("invalid spec", func(t *testing.T) {
		err := Subscribe(ctx, session, SubscriptionSpec{})
		require.Error(t, err)
	})

	require.NoError(t, Start(ctx, session))
	require.Equal(t, StateStreaming, session.State())

	spec := SubscriptionSpec{Exchange: "binance", Channel: "trade", Symbols: []string{"ETH-USDT"}}
	require.NoError(t, Subscribe(ctx, session, spec))
	conn := session.getConnection().(*recordConnection)
	require.Equal(t, spec, conn.subscriptions[len(conn.subscriptions)-1])
}

func TestPublishAndRouteRawValidateState(t *testing.T) {
	ctx := context.Background()
	dialer := &stubDialer{}
	options := Options{
		SessionID: "state-check",
		Dialer:    dialer,
		Parser:    stubParser{},
		Publish:   func(context.Context, *Message) error { return nil },
		Backoff:   BackoffConfig{Initial: time.Millisecond},
	}
	session, err := Init(ctx, options)
	require.NoError(t, err)

	msg := &Message{Type: "noop", Metadata: map[string]string{}}
	require.Error(t, Publish(ctx, session, msg))
	require.Error(t, RouteRaw(ctx, session, []byte("{}")))

	require.NoError(t, Start(ctx, session))
	require.NoError(t, Publish(ctx, session, msg))
	require.NoError(t, RouteRaw(ctx, session, []byte("{}")))
}

func TestUseMiddlewareValidation(t *testing.T) {
	require.Error(t, UseMiddleware(nil, func(context.Context, *Message) (*Message, error) { return nil, nil }))

	ctx := context.Background()
	session, err := Init(ctx, Options{
		SessionID: "mw",
		Dialer:    &stubDialer{},
		Parser:    stubParser{},
		Publish:   func(context.Context, *Message) error { return nil },
		Backoff:   BackoffConfig{Initial: time.Millisecond},
	})
	require.NoError(t, err)
	require.Error(t, UseMiddleware(session, nil))
}

type stubDialer struct {
	lastOptions     DialOptions
	lastConn        *recordConnection
	dialErr         error
	subscriptionErr error
}

func (d *stubDialer) Dial(ctx context.Context, opts DialOptions) (Connection, error) {
	if d.dialErr != nil {
		return nil, d.dialErr
	}
	d.lastOptions = opts
	if d.lastConn == nil {
		d.lastConn = &recordConnection{subErr: d.subscriptionErr}
	}
	return d.lastConn, nil
}

type recordConnection struct {
	subscriptions []SubscriptionSpec
	subErr        error
}

func (c *recordConnection) Subscribe(_ context.Context, spec SubscriptionSpec) error {
	if c.subErr != nil {
		return c.subErr
	}
	c.subscriptions = append(c.subscriptions, spec)
	return nil
}

func (c *recordConnection) Close(context.Context) error { return nil }

type stubParser struct{}

func (stubParser) Parse(context.Context, []byte) (*Message, error) {
	return &Message{Type: "parsed", Payload: map[string]any{"raw": []byte("{}")}, Metadata: map[string]string{}}, nil
}

type capturePublisher struct {
	messages []*Message
}

func (c *capturePublisher) Publish(_ context.Context, msg *Message) error {
	clone := *msg
	if msg.Metadata != nil {
		meta := make(map[string]string, len(msg.Metadata))
		for k, v := range msg.Metadata {
			meta[k] = v
		}
		clone.Metadata = meta
	}
	c.messages = append(c.messages, &clone)
	return nil
}
