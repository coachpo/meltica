package routing

import (
	"context"
	"strings"

	coreexchange "github.com/coachpo/meltica/core/exchange"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
)

// RESTMessage encapsulates information needed to route a REST request through Level 2.
type RESTMessage struct {
	API    rest.API
	Method string
	Path   string
	Query  map[string]string
	Body   []byte
	Signed bool
}

// RESTRouter inspects REST messages and forwards them to the correct Level 1 client.
type RESTRouter struct {
	client *rest.Client
}

// NewRESTRouter constructs a router backed by the Level 1 REST infrastructure client.
func NewRESTRouter(client *rest.Client) *RESTRouter {
	return &RESTRouter{client: client}
}

// Dispatch sends the REST message to the appropriate Binance API surface.
func (r *RESTRouter) Dispatch(ctx context.Context, msg RESTMessage, out any) error {
	api := msg.API
	if api == "" {
		api = inferAPI(msg.Path)
	}
	req := coreexchange.RESTRequest{
		API:    string(api),
		Method: msg.Method,
		Path:   msg.Path,
		Query:  msg.Query,
		Body:   msg.Body,
		Signed: msg.Signed,
	}
	return r.client.Do(ctx, req, out)
}

func inferAPI(path string) rest.API {
	switch {
	case strings.HasPrefix(path, "/fapi/"):
		return rest.LinearAPI
	case strings.HasPrefix(path, "/dapi/"):
		return rest.InverseAPI
	default:
		return rest.SpotAPI
	}
}
