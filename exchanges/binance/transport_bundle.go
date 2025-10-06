package binance

import (
	"errors"

	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type transportBundle struct {
	rest       coretransport.RESTClient
	restRouter routingrest.RESTDispatcher
	ws         wsRouter
	wsInfra    coretransport.StreamClient
}

// buildTransportBundle creates a complete transport bundle using the provided factories.
func buildTransportBundle(transports transportFactories, routers routerFactories, restCfg rest.Config, wsCfg ws.Config) *transportBundle {
	restClient := transports.newRESTClient(restCfg)
	restRouter := routers.newRESTRouter(restClient)
	wsClient := transports.newWSClient(wsCfg)
	return &transportBundle{rest: restClient, restRouter: restRouter, wsInfra: wsClient}
}

func newTransportBundle(rest coretransport.RESTClient, router routingrest.RESTDispatcher, wsClient coretransport.StreamClient) *transportBundle {
	return &transportBundle{rest: rest, restRouter: router, wsInfra: wsClient}
}

func (b *transportBundle) REST() coretransport.RESTClient      { return b.rest }
func (b *transportBundle) Router() routingrest.RESTDispatcher  { return b.restRouter }
func (b *transportBundle) WS() wsRouter                        { return b.ws }
func (b *transportBundle) SetWS(router wsRouter)               { b.ws = router }
func (b *transportBundle) WSInfra() coretransport.StreamClient { return b.wsInfra }

func (b *transportBundle) Close() error {
	var err error
	if b.ws != nil {
		err = errors.Join(err, b.ws.Close())
	}
	if b.wsInfra != nil {
		err = errors.Join(err, b.wsInfra.Close())
	}
	if b.rest != nil {
		err = errors.Join(err, b.rest.Close())
	}
	return err
}
