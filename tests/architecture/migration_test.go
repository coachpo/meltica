package architecture

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core/layers"
	binanceRest "github.com/coachpo/meltica/exchanges/binance/infra/rest"
	binanceWS "github.com/coachpo/meltica/exchanges/binance/infra/ws"
)

func TestBinanceConnectionAdaptersExposeLayerInterfaces(t *testing.T) {
	wsClient := binanceWS.NewClient()
	wsConn := wsClient.AsLayerInterface()
	require.Implements(t, (*layers.WSConnection)(nil), wsConn)

	restClient := binanceRest.NewClient(binanceRest.Config{})
	restConn := restClient.AsLayerInterface()
	require.Implements(t, (*layers.RESTConnection)(nil), restConn)
}
