package pool

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/internal/schema"
)

func TestObjectPoolImmediateReturnRace(t *testing.T) {
	t.Parallel()

	pm := NewPoolManager()
	require.NoError(t, pm.RegisterPool("CanonicalEvent", 1, func() interface{} {
		return new(schema.Event)
	}))

	ctx := context.Background()
	const iterations = 128

	var wg sync.WaitGroup
	wg.Add(iterations)
	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			obj, err := pm.Get(ctx, "CanonicalEvent")
			require.NoError(t, err)
			pm.Put("CanonicalEvent", obj)
		}()
	}
	wg.Wait()
}

func TestPoolTryGetAndTryPut(t *testing.T) {
	t.Parallel()

	pm := NewPoolManager()
	require.NoError(t, pm.RegisterPool("CanonicalEvent", 1, func() interface{} {
		return new(schema.Event)
	}))

	obj, ok, err := pm.TryGet("CanonicalEvent")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, obj)

	next, nextOK, nextErr := pm.TryGet("CanonicalEvent")
	require.NoError(t, nextErr)
	require.False(t, nextOK)
	require.Nil(t, next)

	putOK, putErr := pm.TryPut("CanonicalEvent", obj)
	require.NoError(t, putErr)
	require.True(t, putOK)

	objAgain, errAgain := pm.Get(context.Background(), "CanonicalEvent")
	require.NoError(t, errAgain)
	require.NotNil(t, objAgain)

	pm.Put("CanonicalEvent", objAgain)
}
