package unit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

func TestMiddlewareInjectsSessionMetadataAndBinding(t *testing.T) {
	session := &framework.ConnectionSession{SessionID: "abc"}
	meta := map[string]string{"role": "trader"}
	binding := handler.Binding{Name: "quotes", Channels: []string{"BTC-USDT"}, MaxConcurrency: 2}

	final := handler.Chain(handler.HandlerFunc(func(ctx context.Context, env stream.Envelope) (stream.Outcome, error) {
		sess, ok := handler.SessionFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, session, sess)

		metadata := handler.MetadataFromContext(ctx)
		require.Equal(t, meta, metadata)

		extracted, ok := handler.BindingFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, binding, extracted)

		token, ok := handler.AuthTokenFromContext(ctx)
		require.True(t, ok)
		require.Equal(t, "secret", token)

		return framework.HandlerOutcome{}, nil
	}),
		handler.WithSession(session),
		handler.WithMetadata(meta),
		handler.WithAuthToken(" secret "),
		handler.WithBinding(binding),
	)

	ctx := context.Background()
	_, err := final.Handle(ctx, &framework.MessageEnvelope{})
	require.NoError(t, err)

	// Ensure original metadata map is not mutated.
	require.Equal(t, map[string]string{"role": "trader"}, meta)
}
