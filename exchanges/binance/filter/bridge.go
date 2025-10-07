package filter

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

// RESTBridge implements a Level-4 bridge that maps InteractionRequest to routing.RESTMessage
type RESTBridge struct {
	router routing.RESTDispatcher
}

// NewRESTBridge creates a new bridge that wraps a REST router
func NewRESTBridge(router routing.RESTDispatcher) *RESTBridge {
	return &RESTBridge{router: router}
}

// Dispatch implements the restExecutor interface by bridging InteractionRequest to RESTMessage
func (b *RESTBridge) Dispatch(ctx context.Context, msg interface{}, result interface{}) error {
	if b.router == nil {
		return fmt.Errorf("REST router not available")
	}

	// Convert InteractionRequest to RESTMessage
	interactionReq, ok := msg.(mdfilter.InteractionRequest)
	if !ok {
		return fmt.Errorf("expected InteractionRequest, got %T", msg)
	}

	restMsg, err := b.mapInteractionToREST(interactionReq)
	if err != nil {
		return fmt.Errorf("failed to map interaction to REST: %w", err)
	}

	// Dispatch through the router
	return b.router.Dispatch(ctx, restMsg, result)
}

// mapInteractionToREST converts an InteractionRequest to a routing.RESTMessage
func (b *RESTBridge) mapInteractionToREST(req mdfilter.InteractionRequest) (routing.RESTMessage, error) {
	// Determine API based on path
	api := b.inferAPI(req.Path)

	// Serialize payload
	var body []byte
	if req.Payload != nil {
		var err error
		body, err = json.Marshal(req.Payload)
		if err != nil {
			return routing.RESTMessage{}, fmt.Errorf("failed to serialize payload: %w", err)
		}
	}

	// Extract query parameters from path if present and merge with explicit QueryParams
	query := make(map[string]string)
	if strings.Contains(req.Path, "?") {
		parts := strings.SplitN(req.Path, "?", 2)
		if len(parts) == 2 {
			queryStr := parts[1]
			for _, param := range strings.Split(queryStr, "&") {
				if kv := strings.SplitN(param, "=", 2); len(kv) == 2 {
					query[kv[0]] = kv[1]
				}
			}
		}
	}
	// Merge explicit query parameters (overriding path parameters)
	for k, v := range req.QueryParams {
		query[k] = v
	}

	// Determine if request should be signed based on hint or auto-detection
	signed := b.determineSigning(req)

	// Build headers
	headers := make(http.Header)
	headers.Set("Content-Type", "application/json")

	// Add custom headers
	for k, v := range req.Headers {
		headers.Set(k, v)
	}

	// Add authentication headers if required
	if signed && req.AuthContext != nil {
		// Note: Actual signing would be handled by the underlying REST client
		headers.Set("X-MBX-APIKEY", req.AuthContext.APIKey)
	}

	return routing.RESTMessage{
		API:    api,
		Method: req.Method,
		Path:   b.normalizePath(req.Path),
		Query:  query,
		Body:   body,
		Signed: signed,
		Header: headers,
	}, nil
}

// determineSigning determines if the request should be signed based on hint and auto-detection
func (b *RESTBridge) determineSigning(req mdfilter.InteractionRequest) bool {
	switch mdfilter.SigningHint(req.SigningHint) {
	case mdfilter.SigningHintRequired:
		return true
	case mdfilter.SigningHintNone:
		return false
	case mdfilter.SigningHintAuto, "":
		fallthrough
	default:
		return b.shouldSign(req.Method, req.Path)
	}
}

// inferAPI determines the Binance API endpoint based on the path
func (b *RESTBridge) inferAPI(path string) string {
	switch {
	case strings.HasPrefix(path, "/fapi/"):
		return string(rest.LinearAPI)
	case strings.HasPrefix(path, "/dapi/"):
		return string(rest.InverseAPI)
	default:
		return string(rest.SpotAPI)
	}
}

// shouldSign determines if the request requires signing
func (b *RESTBridge) shouldSign(method, path string) bool {
	// Private endpoints require signing
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

// normalizePath removes query parameters from the path
func (b *RESTBridge) normalizePath(path string) string {
	if idx := strings.Index(path, "?"); idx != -1 {
		return path[:idx]
	}
	return path
}
