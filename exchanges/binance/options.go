package binance

import (
	"time"

	"github.com/coachpo/meltica/config"
	"github.com/coachpo/meltica/core/exchanges/bootstrap"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

// Option is a functional option for configuring Binance exchange construction.
type Option = bootstrap.Option

// defaultConstructionParams returns Binance-specific defaults.
func defaultConstructionParams() *bootstrap.ConstructionParams {
	params := bootstrap.NewConstructionParams()

	params.Transports = bootstrap.TransportFactories{
		NewRESTClient: func(cfg interface{}) coretransport.RESTClient {
			return rest.NewClient(cfg.(rest.Config))
		},
		NewWSClient: func(cfg interface{}) coretransport.StreamClient {
			return ws.NewClient(cfg.(ws.Config))
		},
	}

	params.Routers = bootstrap.RouterFactories{
		NewRESTRouter: func(client coretransport.RESTClient) interface{} {
			return bnrouting.NewRESTRouter(client)
		},
		NewWSRouter: func(client coretransport.StreamClient, deps interface{}) interface{} {
			return bnrouting.NewWSRouter(client, deps.(bnrouting.WSDependencies))
		},
	}

	return params
}

// WithRESTClientFactory sets a custom REST client factory.
func WithRESTClientFactory(factory func(rest.Config) coretransport.RESTClient) Option {
	return func(params *bootstrap.ConstructionParams) {
		if factory != nil {
			params.Transports.NewRESTClient = func(cfg interface{}) coretransport.RESTClient {
				return factory(cfg.(rest.Config))
			}
		}
	}
}

// WithRESTClient sets a pre-configured REST client.
func WithRESTClient(client coretransport.RESTClient) Option {
	return WithRESTClientFactory(func(rest.Config) coretransport.RESTClient { return client })
}

// WithRESTRouterFactory sets a custom REST router factory.
func WithRESTRouterFactory(factory func(coretransport.RESTClient) routingrest.RESTDispatcher) Option {
	return func(params *bootstrap.ConstructionParams) {
		if factory != nil {
			params.Routers.NewRESTRouter = func(client coretransport.RESTClient) interface{} {
				return factory(client)
			}
		}
	}
}

// WithRESTRouter sets a pre-configured REST router.
func WithRESTRouter(router routingrest.RESTDispatcher) Option {
	return WithRESTRouterFactory(func(coretransport.RESTClient) routingrest.RESTDispatcher { return router })
}

// WithWSClientFactory sets a custom WebSocket client factory.
func WithWSClientFactory(factory func(ws.Config) coretransport.StreamClient) Option {
	return func(params *bootstrap.ConstructionParams) {
		if factory != nil {
			params.Transports.NewWSClient = func(cfg interface{}) coretransport.StreamClient {
				return factory(cfg.(ws.Config))
			}
		}
	}
}

// WithWSClient sets a pre-configured WebSocket client.
func WithWSClient(client coretransport.StreamClient) Option {
	return WithWSClientFactory(func(ws.Config) coretransport.StreamClient { return client })
}

// WithWSRouterFactory sets a custom WebSocket router factory.
func WithWSRouterFactory(factory func(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter) Option {
	return func(params *bootstrap.ConstructionParams) {
		if factory != nil {
			params.Routers.NewWSRouter = func(client coretransport.StreamClient, deps interface{}) interface{} {
				return factory(client, deps.(bnrouting.WSDependencies))
			}
		}
	}
}

// WithWSRouter sets a pre-configured WebSocket router.
func WithWSRouter(router wsRouter) Option {
	return WithWSRouterFactory(func(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter { return router })
}

// WithSymbolRefreshInterval configures automatic symbol refresh.
func WithSymbolRefreshInterval(interval time.Duration) Option {
	return func(params *bootstrap.ConstructionParams) {
		params.ConfigOpts = append(params.ConfigOpts, config.WithBinanceSymbolRefreshInterval(interval))
	}
}
