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

func TestFrameworkSessionParseWrapsErrors(t *testing.T) {
	ctx := context.Background()
	options := Options{
		SessionID: "parse-session",
		Dialer:    &nopDialer{},
		Parser:    failingParser{},
		Publish:   nopPublisher,
		Backoff: BackoffConfig{
			Initial:    100 * time.Millisecond,
			Max:        time.Second,
			Multiplier: big.NewRat(2, 1),
			Jitter:     10,
		},
	}

	session, err := Init(ctx, options)
	require.NoError(t, err)

	_, err = session.parse(ctx, []byte("bad"))
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeInvalid, e.Code)
}

func TestFrameworkSessionNextBackoffRespectsMaximum(t *testing.T) {
	ctx := context.Background()
	options := Options{
		SessionID: "backoff-session",
		Dialer:    &nopDialer{},
		Parser:    nopParser{},
		Publish:   nopPublisher,
		Backoff: BackoffConfig{
			Initial:    200 * time.Millisecond,
			Max:        1200 * time.Millisecond,
			Multiplier: big.NewRat(3, 2),
			Jitter:     5,
		},
	}

	session, err := Init(ctx, options)
	require.NoError(t, err)

	durations := []time.Duration{
		session.nextBackoff(0),
		session.nextBackoff(1),
		session.nextBackoff(2),
		session.nextBackoff(5),
	}

	require.Equal(t, []time.Duration{
		200 * time.Millisecond,
		300 * time.Millisecond,
		450 * time.Millisecond,
		1200 * time.Millisecond,
	}, durations)
}

func TestFrameworkSessionMiddlewareStopsOnError(t *testing.T) {
	ctx := context.Background()
	options := Options{
		SessionID: "middleware-session",
		Dialer:    &nopDialer{},
		Parser:    nopParser{},
		Publish:   nopPublisher,
		Backoff: BackoffConfig{
			Initial:    150 * time.Millisecond,
			Max:        2 * time.Second,
			Multiplier: big.NewRat(2, 1),
			Jitter:     10,
		},
	}

	session, err := Init(ctx, options)
	require.NoError(t, err)
	require.NoError(t, UseMiddleware(session, func(ctx context.Context, msg *Message) (*Message, error) {
		msg.Metadata["seen"] = "true"
		return msg, nil
	}))
	require.NoError(t, UseMiddleware(session, func(context.Context, *Message) (*Message, error) {
		return nil, errs.New("", errs.CodeExchange, errs.WithMessage("fail"))
	}))

	msg := &Message{Type: "book", Payload: map[string]any{"depth": 1}, Metadata: map[string]string{}}

	_, err = session.applyMiddleware(ctx, msg)
	require.Error(t, err)
	var e *errs.E
	require.True(t, errors.As(err, &e))
	require.Equal(t, errs.CodeExchange, e.Code)
}

type failingParser struct{}

func (failingParser) Parse(context.Context, []byte) (*Message, error) {
	return nil, errs.New("", errs.CodeInvalid, errs.WithMessage("bad payload"))
}

type nopParser struct{}

func (nopParser) Parse(context.Context, []byte) (*Message, error) {
	return &Message{Type: "noop", Payload: map[string]any{}, Metadata: map[string]string{}}, nil
}

type nopDialer struct{}

func (nopDialer) Dial(context.Context, DialOptions) (Connection, error) {
	return nil, nil
}

func nopPublisher(context.Context, *Message) error { return nil }
