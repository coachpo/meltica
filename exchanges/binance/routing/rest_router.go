package routing

import (
	"context"
	"strings"

	"github.com/coachpo/meltica/core/layers"
	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	genericrouting "github.com/coachpo/meltica/exchanges/shared/routing"
)

type restClientProvider interface {
	LegacyRESTClient() coretransport.RESTClient
}

// RESTRouter wraps the shared REST dispatcher to expose layer interfaces.
type RESTRouter struct {
	dispatcher genericrouting.RESTDispatcher
}

// Dispatch satisfies genericrouting.RESTDispatcher.
func (r *RESTRouter) Dispatch(ctx context.Context, msg genericrouting.RESTMessage, out any) error {
	if r == nil || r.dispatcher == nil {
		return nil
	}
	return r.dispatcher.Dispatch(ctx, msg, out)
}

// LegacyRESTDispatcher exposes the underlying dispatcher for migration scenarios.
func (r *RESTRouter) LegacyRESTDispatcher() genericrouting.RESTDispatcher {
	if r == nil {
		return nil
	}
	return r.dispatcher
}

// NewRESTRouter constructs a router backed by the Level 1 REST infrastructure client.
func NewRESTRouter(conn layers.RESTConnection) *RESTRouter {
	if conn == nil {
		panic("binance routing: nil RESTConnection")
	}
	provider, ok := conn.(restClientProvider)
	if !ok {
		panic("binance routing: RESTConnection missing legacy REST client provider")
	}
	client := provider.LegacyRESTClient()
	if client == nil {
		panic("binance routing: legacy REST client required")
	}
	resolver := genericrouting.RESTAPIResolverFunc(func(msg genericrouting.RESTMessage) string {
		return inferAPI(msg.Path)
	})
	return &RESTRouter{dispatcher: genericrouting.NewDefaultRESTRouter(client, resolver)}
}

func inferAPI(path string) string {
	switch {
	case strings.HasPrefix(path, "/fapi/"):
		return string(rest.LinearAPI)
	case strings.HasPrefix(path, "/dapi/"):
		return string(rest.InverseAPI)
	default:
		return string(rest.SpotAPI)
	}
}

// AsLayerInterface exposes the REST router via the layers.RESTRouting contract.
func (r *RESTRouter) AsLayerInterface() layers.RESTRouting {
	if r == nil {
		return nil
	}
	return &layerRESTRouting{router: r}
}
