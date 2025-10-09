package unit

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/stream"
	"github.com/coachpo/meltica/market_data/framework/handler"
)

func TestRegistryRegisterAndResolveLatest(t *testing.T) {
	reg := handler.NewRegistry()
	_, err := reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"btc-usdt"},
		Factory:  func() stream.Handler { return stubHandler{} },
	})
	require.NoError(t, err)

	latest, ok := reg.Resolve("quotes", "")
	require.True(t, ok)
	require.Equal(t, "1.0.0", latest.Version)
	require.Equal(t, []string{"BTC-USDT"}, latest.Channels)
	require.True(t, latest.Active)
	require.WithinDuration(t, time.Now(), latest.RegisteredAt, time.Second)

	instance := latest.Instantiate()
	require.NotNil(t, instance)
}

func TestRegistryVersioningAndActivation(t *testing.T) {
	reg := handler.NewRegistry()
	_, err := reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"btc-usdt"},
		Factory:  func() stream.Handler { return stubHandler{} },
	})
	require.NoError(t, err)

	_, err = reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.1.0",
		Channels: []string{"btc-usdt"},
		Factory:  func() stream.Handler { return stubHandler{} },
	})
	require.NoError(t, err)

	latest, ok := reg.Resolve("quotes", "")
	require.True(t, ok)
	require.Equal(t, "1.1.0", latest.Version)

	legacy, ok := reg.Resolve("quotes", "1.0.0")
	require.True(t, ok)
	require.Equal(t, "1.0.0", legacy.Version)

	err = reg.SetActive("quotes", "1.1.0", false)
	require.NoError(t, err)

	reactivated, ok := reg.Resolve("quotes", "")
	require.True(t, ok)
	require.Equal(t, "1.0.0", reactivated.Version)

	err = reg.SetActive("quotes", "1.0.0", false)
	require.NoError(t, err)

	_, ok = reg.Resolve("quotes", "")
	require.False(t, ok)
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	reg := handler.NewRegistry()
	_, err := reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"btc-usdt"},
		Factory:  func() stream.Handler { return stubHandler{} },
	})
	require.NoError(t, err)

	_, err = reg.Register(stream.HandlerRegistration{
		Name:     "quotes",
		Version:  "1.0.0",
		Channels: []string{"btc-usdt"},
		Factory:  func() stream.Handler { return stubHandler{} },
	})
	require.Error(t, err)
}

type stubHandler struct{}

func (stubHandler) Handle(context.Context, stream.Envelope) (stream.Outcome, error) {
	return nil, nil
}
