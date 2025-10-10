package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/shared/routing"
	mdfilter "github.com/coachpo/meltica/pipeline"
)

// Dispatcher defines the contract for Level-3 components that translate filter interactions
// into exchange-specific REST calls.
type Dispatcher interface {
	Dispatch(ctx context.Context, req mdfilter.InteractionRequest, result any) error
}

type routerBridge struct {
	router routing.RESTDispatcher
}

// NewRouterBridge creates a dispatcher that bridges InteractionRequest to routing.RESTMessage.
func NewRouterBridge(router routing.RESTDispatcher) Dispatcher {
	return &routerBridge{router: router}
}

// Dispatch implements the Dispatcher interface by mapping InteractionRequest to RESTMessage.
func (b *routerBridge) Dispatch(ctx context.Context, req mdfilter.InteractionRequest, result any) error {
	if b.router == nil {
		return fmt.Errorf("REST router not available")
	}

	restMsg, err := mapInteractionToREST(req)
	if err != nil {
		return fmt.Errorf("failed to map interaction to REST: %w", err)
	}

	return b.router.Dispatch(ctx, restMsg, result)
}

func mapInteractionToREST(req mdfilter.InteractionRequest) (routing.RESTMessage, error) {
	api := inferAPI(req.Path)

	var body []byte
	if req.Payload != nil {
		data, err := json.Marshal(req.Payload)
		if err != nil {
			return routing.RESTMessage{}, fmt.Errorf("failed to serialize payload: %w", err)
		}
		body = data
	}

	query := make(map[string]string)
	if strings.Contains(req.Path, "?") {
		parts := strings.SplitN(req.Path, "?", 2)
		if len(parts) == 2 {
			for _, param := range strings.Split(parts[1], "&") {
				if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
					query[kv[0]] = kv[1]
				}
			}
		}
	}
	for k, v := range req.QueryParams {
		query[k] = v
	}

	signed := determineSigning(req)

	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")
	for k, v := range req.Headers {
		headers.Set(k, v)
	}
	if signed && req.AuthContext != nil {
		headers.Set("X-MBX-APIKEY", req.AuthContext.APIKey)
	}

	return routing.RESTMessage{
		API:    api,
		Method: req.Method,
		Path:   normalizePath(req.Path),
		Query:  query,
		Body:   body,
		Signed: signed,
		Header: headers,
	}, nil
}

func determineSigning(req mdfilter.InteractionRequest) bool {
	switch mdfilter.SigningHint(req.SigningHint) {
	case mdfilter.SigningHintRequired:
		return true
	case mdfilter.SigningHintNone:
		return false
	case mdfilter.SigningHintAuto, "":
		fallthrough
	default:
		return shouldSign(req.Method, req.Path)
	}
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

func shouldSign(method, path string) bool {
	privatePaths := []string{
		"/api/v3/account",
		"/api/v3/order",
		"/api/v3/openOrders",
		"/api/v3/allOrders",
		"/api/v3/myTrades",
		"/fapi/v1/account",
		"/fapi/v1/order",
		"/fapi/v1/openOrders",
		"/fapi/v1/allOrders",
		"/fapi/v1/userTrades",
		"/dapi/v1/account",
		"/dapi/v1/order",
		"/dapi/v1/openOrders",
		"/dapi/v1/allOrders",
		"/dapi/v1/userTrades",
	}

	for _, privatePath := range privatePaths {
		if strings.HasPrefix(path, privatePath) {
			return true
		}
	}

	return false
}

func normalizePath(path string) string {
	if idx := strings.Index(path, "?"); idx != -1 {
		return path[:idx]
	}
	return path
}
