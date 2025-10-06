package bootstrap

import (
	"errors"

	coretransport "github.com/coachpo/meltica/core/transport"
)

// TransportBundle holds all transport clients and routers for an exchange.
type TransportBundle struct {
	rest       coretransport.RESTClient
	restRouter interface{}
	ws         interface{}
	wsInfra    coretransport.StreamClient
}

// BuildTransportBundle creates a complete transport bundle using the provided factories and configs.
// The restCfg and wsCfg should be exchange-specific configuration structs.
func BuildTransportBundle(
	transports TransportFactories,
	routers RouterFactories,
	restCfg interface{},
	wsCfg interface{},
) *TransportBundle {
	var restClient coretransport.RESTClient
	var wsClient coretransport.StreamClient

	if transports.NewRESTClient != nil {
		restClient = transports.NewRESTClient(restCfg)
	}
	if transports.NewWSClient != nil {
		wsClient = transports.NewWSClient(wsCfg)
	}

	var restRouter interface{}
	if routers.NewRESTRouter != nil && restClient != nil {
		restRouter = routers.NewRESTRouter(restClient)
	}

	return &TransportBundle{
		rest:       restClient,
		restRouter: restRouter,
		wsInfra:    wsClient,
	}
}

// NewTransportBundle creates a transport bundle from existing components.
func NewTransportBundle(
	rest coretransport.RESTClient,
	router interface{},
	wsClient coretransport.StreamClient,
) *TransportBundle {
	return &TransportBundle{
		rest:       rest,
		restRouter: router,
		wsInfra:    wsClient,
	}
}

// REST returns the REST client.
func (b *TransportBundle) REST() coretransport.RESTClient {
	return b.rest
}

// Router returns the REST router.
func (b *TransportBundle) Router() interface{} {
	return b.restRouter
}

// WS returns the websocket router.
func (b *TransportBundle) WS() interface{} {
	return b.ws
}

// SetWS sets the websocket router (used for late initialization).
func (b *TransportBundle) SetWS(router interface{}) {
	b.ws = router
}

// WSInfra returns the underlying websocket client.
func (b *TransportBundle) WSInfra() coretransport.StreamClient {
	return b.wsInfra
}

// Close terminates all transports in the bundle.
func (b *TransportBundle) Close() error {
	var err error
	if b.ws != nil {
		if closer, ok := b.ws.(interface{ Close() error }); ok {
			err = errors.Join(err, closer.Close())
		}
	}
	if b.wsInfra != nil {
		err = errors.Join(err, b.wsInfra.Close())
	}
	if b.rest != nil {
		err = errors.Join(err, b.rest.Close())
	}
	return err
}
