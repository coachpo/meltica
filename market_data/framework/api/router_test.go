package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouterParameterizedRoute(t *testing.T) {
	router := NewRouter(RouterConfig{})
	router.Handle(http.MethodGet, "/items/:id", func(ctx context.Context, w http.ResponseWriter, req *http.Request) error {
		id, ok := Param(req, "id")
		require.True(t, ok)
		require.Equal(t, "123", id)
		writeJSON(w, http.StatusOK, map[string]any{"id": id})
		return nil
	})
	req := httptest.NewRequest(http.MethodGet, "/items/123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Contains(t, resp.Body.String(), "123")
	postReq := httptest.NewRequest(http.MethodPost, "/items/123", nil)
	postResp := httptest.NewRecorder()
	router.ServeHTTP(postResp, postReq)
	require.Equal(t, http.StatusMethodNotAllowed, postResp.Code)
	require.Equal(t, "GET", postResp.Header().Get("Allow"))
}
