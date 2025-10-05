package routing

import (
	"strings"

	coretransport "github.com/coachpo/meltica/core/transport"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	genericrouting "github.com/coachpo/meltica/exchanges/shared/routing"
)

// NewRESTRouter constructs a router backed by the Level 1 REST infrastructure client.
func NewRESTRouter(client coretransport.RESTClient) genericrouting.RESTDispatcher {
	resolver := genericrouting.RESTAPIResolverFunc(func(msg genericrouting.RESTMessage) string {
		return inferAPI(msg.Path)
	})
	return genericrouting.NewDefaultRESTRouter(client, resolver)
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
