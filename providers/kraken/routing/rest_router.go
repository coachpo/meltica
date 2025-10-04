package routing

import (
	"context"
	"strings"

	coreprovider "github.com/coachpo/meltica/core/provider"
	"github.com/coachpo/meltica/providers/kraken/infra/rest"
)

// RESTMessage encapsulates routing information for Kraken REST calls.
type RESTMessage struct {
	API    rest.API
	Method string
	Path   string
	Query  map[string]string
	Body   []byte
	Signed bool
}

// RESTRouter normalizes REST messages into core/provider requests.
type RESTRouter struct {
	client *rest.Client
}

// NewRESTRouter constructs a router backed by the infrastructure client.
func NewRESTRouter(client *rest.Client) *RESTRouter {
	return &RESTRouter{client: client}
}

// Dispatch executes the message through the appropriate Kraken REST surface.
func (r *RESTRouter) Dispatch(ctx context.Context, msg RESTMessage, out any) error {
	api := msg.API
	if api == "" {
		api = inferAPI(msg.Path)
	}
	req := coreprovider.RESTRequest{
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
	if strings.HasPrefix(path, "/api/") {
		return rest.FuturesAPI
	}
	return rest.SpotAPI
}
