package binance

import (
	"context"
	"net/http"

	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/binance/internal"
	routingrest "github.com/coachpo/meltica/exchanges/shared/routing"
)

// listenKeyService manages Binance listen-key lifecycle via REST.
type listenKeyService struct {
	router routingrest.RESTDispatcher
}

func newListenKeyService(router routingrest.RESTDispatcher) *listenKeyService {
	return &listenKeyService{router: router}
}

func (s *listenKeyService) Create(ctx context.Context) (string, error) {
	if s.router == nil {
		return "", internal.Invalid("listen key: rest router unavailable")
	}
	var resp struct {
		ListenKey string `json:"listenKey"`
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodPost, Path: "/api/v3/userDataStream"}
	if err := s.router.Dispatch(ctx, msg, &resp); err != nil {
		return "", err
	}
	return resp.ListenKey, nil
}

func (s *listenKeyService) KeepAlive(ctx context.Context, key string) error {
	if s.router == nil {
		return internal.Invalid("listen key: rest router unavailable")
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodPut, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return s.router.Dispatch(ctx, msg, nil)
}

func (s *listenKeyService) Close(ctx context.Context, key string) error {
	if s.router == nil {
		return internal.Invalid("listen key: rest router unavailable")
	}
	msg := routingrest.RESTMessage{API: string(rest.SpotAPI), Method: http.MethodDelete, Path: "/api/v3/userDataStream", Query: map[string]string{"listenKey": key}}
	return s.router.Dispatch(ctx, msg, nil)
}
