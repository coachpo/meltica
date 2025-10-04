package routing

import (
	"context"
	"net/http"

	coreexchange "github.com/coachpo/meltica/core/exchange"
)

// RESTMessage represents a normalized REST request emitted by the business layer.
type RESTMessage struct {
	API    string
	Method string
	Path   string
	Query  map[string]string
	Body   []byte
	Signed bool
	Header http.Header
}

// RESTDispatcher defines the contract for Level 2 REST routers.
type RESTDispatcher interface {
	Dispatch(ctx context.Context, msg RESTMessage, out any) error
}

// RESTAPIResolver resolves exchange-specific API identifiers for REST messages.
type RESTAPIResolver interface {
	ResolveRESTAPI(msg RESTMessage) string
}

// RESTAPIResolverFunc converts a function into a RESTAPIResolver.
type RESTAPIResolverFunc func(msg RESTMessage) string

// ResolveRESTAPI allows RESTAPIResolverFunc to satisfy RESTAPIResolver.
func (f RESTAPIResolverFunc) ResolveRESTAPI(msg RESTMessage) string { return f(msg) }

// DefaultRESTRouter routes REST messages via a coreexchange.RESTClient.
type DefaultRESTRouter struct {
	client   coreexchange.RESTClient
	resolver RESTAPIResolver
}

// NewDefaultRESTRouter constructs a reusable router backed by a REST client and optional resolver.
func NewDefaultRESTRouter(client coreexchange.RESTClient, resolver RESTAPIResolver) *DefaultRESTRouter {
	return &DefaultRESTRouter{client: client, resolver: resolver}
}

// Dispatch implements RESTDispatcher.
func (r *DefaultRESTRouter) Dispatch(ctx context.Context, msg RESTMessage, out any) error {
	if r == nil || r.client == nil {
		return nil
	}
	api := msg.API
	if api == "" && r.resolver != nil {
		api = r.resolver.ResolveRESTAPI(msg)
	}
	req := coreexchange.RESTRequest{
		API:    api,
		Method: msg.Method,
		Path:   msg.Path,
		Query:  msg.Query,
		Body:   msg.Body,
		Signed: msg.Signed,
		Header: msg.Header,
	}
	resp, err := r.client.DoRequest(ctx, req)
	if err != nil {
		return r.client.HandleError(ctx, req, err)
	}
	return r.client.HandleResponse(ctx, req, resp, out)
}
