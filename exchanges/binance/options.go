package binance

import (
	"github.com/coachpo/meltica/config"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/infra/ws"
	bnrouting "github.com/coachpo/meltica/exchanges/binance/routing"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

type Option func(*constructionParams)

type constructionParams struct {
	cfgOpts    []config.Option
	transports transportFactories
	routers    routerFactories
}

type transportFactories struct {
	newRESTClient func(rest.Config) coretransport.RESTClient
	newWSClient   func(ws.Config) coretransport.StreamClient
}

type routerFactories struct {
	newRESTRouter func(coretransport.RESTClient) routingrest.RESTDispatcher
	newWSRouter   func(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter
}

func defaultConstructionParams() constructionParams {
	return constructionParams{
		transports: transportFactories{
			newRESTClient: func(cfg rest.Config) coretransport.RESTClient { return rest.NewClient(cfg) },
			newWSClient:   func(cfg ws.Config) coretransport.StreamClient { return ws.NewClient(cfg) },
		},
		routers: routerFactories{
			newRESTRouter: func(client coretransport.RESTClient) routingrest.RESTDispatcher {
				return bnrouting.NewRESTRouter(client)
			},
			newWSRouter: func(client coretransport.StreamClient, deps bnrouting.WSDependencies) wsRouter {
				return bnrouting.NewWSRouter(client, deps)
			},
		},
	}
}

func WithConfig(options ...config.Option) Option {
	return func(params *constructionParams) {
		params.cfgOpts = append(params.cfgOpts, options...)
	}
}

func WithRESTClientFactory(factory func(rest.Config) coretransport.RESTClient) Option {
	return func(params *constructionParams) {
		if factory != nil {
			params.transports.newRESTClient = factory
		}
	}
}

func WithRESTClient(client coretransport.RESTClient) Option {
	return WithRESTClientFactory(func(rest.Config) coretransport.RESTClient { return client })
}

func WithRESTRouterFactory(factory func(coretransport.RESTClient) routingrest.RESTDispatcher) Option {
	return func(params *constructionParams) {
		if factory != nil {
			params.routers.newRESTRouter = factory
		}
	}
}

func WithRESTRouter(router routingrest.RESTDispatcher) Option {
	return WithRESTRouterFactory(func(coretransport.RESTClient) routingrest.RESTDispatcher { return router })
}

func WithWSClientFactory(factory func(ws.Config) coretransport.StreamClient) Option {
	return func(params *constructionParams) {
		if factory != nil {
			params.transports.newWSClient = factory
		}
	}
}

func WithWSClient(client coretransport.StreamClient) Option {
	return WithWSClientFactory(func(ws.Config) coretransport.StreamClient { return client })
}

func WithWSRouterFactory(factory func(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter) Option {
	return func(params *constructionParams) {
		if factory != nil {
			params.routers.newWSRouter = factory
		}
	}
}

func WithWSRouter(router wsRouter) Option {
	return WithWSRouterFactory(func(coretransport.StreamClient, bnrouting.WSDependencies) wsRouter { return router })
}
