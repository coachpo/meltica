package processors

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/market_data"
)

func TestDefaultProcessorProcess(t *testing.T) {
	processor := NewDefaultProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	raw := []byte(`{"unknown":true}`)
	result, err := processor.Process(context.Background(), raw)
	require.NoError(t, err)
	payload, ok := result.(*market_data.RawPayload)
	require.True(t, ok, "expected *RawPayload")
	require.Equal(t, `{"unknown":true}`, string(payload.Data))
	raw[0] = 'X'
	require.NotEqual(t, raw[0], payload.Data[0])
	require.Equal(t, "default", payload.Metadata["processor"])
}

func TestDefaultProcessorContextCancelled(t *testing.T) {
	processor := NewDefaultProcessor()
	require.NoError(t, processor.Initialize(context.Background()))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := processor.Process(ctx, []byte("test"))
	require.Error(t, err)
	require.True(t, errors.Is(err, context.Canceled))
}
