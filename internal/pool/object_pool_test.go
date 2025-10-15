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

func TestBorrowCanonicalEvents(t *testing.T) {
	t.Parallel()

	pm := NewPoolManager()
	require.NoError(t, pm.RegisterPool("CanonicalEvent", 4, func() interface{} {
		return new(schema.Event)
	}))

	events, err := pm.BorrowCanonicalEvents(context.Background(), 3)
	require.NoError(t, err)
	require.Len(t, events, 3)

	pm.RecycleCanonicalEvents(events)
}

func TestTryBorrowCanonicalEvents(t *testing.T) {
	t.Parallel()

	pm := NewPoolManager()
	require.NoError(t, pm.RegisterPool("CanonicalEvent", 2, func() interface{} {
		return new(schema.Event)
	}))

	events, ok, err := pm.TryBorrowCanonicalEvents(2)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, events, 2)
	pm.RecycleCanonicalEvents(events)

	events, ok, err = pm.TryBorrowCanonicalEvents(3)
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, events)
}
