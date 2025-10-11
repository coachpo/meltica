package integration

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/core"
	"github.com/coachpo/meltica/core/layers"
	corestreams "github.com/coachpo/meltica/core/streams"
	binanceRest "github.com/coachpo/meltica/exchanges/binance/infra/rest"
	binanceWS "github.com/coachpo/meltica/exchanges/binance/infra/ws"
	binanceRouting "github.com/coachpo/meltica/exchanges/binance/routing"
)

type dummyWSDeps struct{}

func (dummyWSDeps) BookDepthSnapshot(context.Context, string, int) (corestreams.BookEvent, int64, error) {
	return corestreams.BookEvent{}, 0, nil
}

func (dummyWSDeps) CanonicalSymbol(symbol string) (string, error) { return symbol, nil }
func (dummyWSDeps) NativeSymbol(symbol string) (string, error)    { return symbol, nil }
func (dummyWSDeps) NativeTopic(core.Topic) (string, error)        { return "", nil }
func (dummyWSDeps) CreateListenKey(context.Context) (string, error) {
	return "listen-key", nil
}
func (dummyWSDeps) KeepAliveListenKey(context.Context, string) error { return nil }
func (dummyWSDeps) CloseListenKey(context.Context, string) error     { return nil }

func TestBinanceRoutingAdaptersExposeLayerInterfaces(t *testing.T) {
	wsClient := binanceWS.NewClient()
	wsConn := wsClient.AsLayerInterface()
	require.Implements(t, (*layers.WSConnection)(nil), wsConn)

	wsRouter := binanceRouting.NewWSRouter(wsConn, dummyWSDeps{})
	wsRouting := wsRouter.AsLayerInterface()
	require.Implements(t, (*layers.WSRouting)(nil), wsRouting)

	restClient := binanceRest.NewClient(binanceRest.Config{})
	restConn := restClient.AsLayerInterface()
	require.Implements(t, (*layers.RESTConnection)(nil), restConn)

	restRouter := binanceRouting.NewRESTRouter(restConn)
	restRouting := restRouter.AsLayerInterface()
	require.Implements(t, (*layers.RESTRouting)(nil), restRouting)
}
