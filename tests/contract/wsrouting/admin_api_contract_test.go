package wsrouting_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coachpo/meltica/lib/ws-routing/api"
)

type inspectorStub struct {
	status api.Status
	items  []api.Subscription
}

func (s inspectorStub) Status() api.Status {
	return s.status
}

func (s inspectorStub) Subscriptions() []api.Subscription {
	return append([]api.Subscription(nil), s.items...)
}

func TestAdminHealthEndpointContract(t *testing.T) {
	inspector := inspectorStub{status: api.StatusRunning}
	handler := api.New(inspector)
	req := httptest.NewRequest(http.MethodGet, "/admin/ws-routing/v1/health", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, "application/json", res.Header().Get("Content-Type"))
	require.JSONEq(t, `{"status":"ok"}`, res.Body.String())
}

func TestAdminSubscriptionsEndpointContract(t *testing.T) {
	inspector := inspectorStub{
		status: api.StatusRunning,
		items: []api.Subscription{
			{Symbol: "BTC-USD", Stream: "trades"},
			{Symbol: "ETH-USD", Stream: "book"},
		},
	}
	handler := api.New(inspector)
	req := httptest.NewRequest(http.MethodGet, "/admin/ws-routing/v1/subscriptions", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	require.Equal(t, http.StatusOK, res.Code)
	require.Equal(t, "application/json", res.Header().Get("Content-Type"))
	require.JSONEq(t, `{"items":[{"symbol":"BTC-USD","stream":"trades"},{"symbol":"ETH-USD","stream":"book"}]}`, res.Body.String())
}
