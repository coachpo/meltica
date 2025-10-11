package bridge

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/core/layers"
	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/shared/routing"
	mdfilter "github.com/coachpo/meltica/pipeline"
	archmocks "github.com/coachpo/meltica/tests/architecture/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRESTDispatcher struct {
	dispatchFunc func(ctx context.Context, msg routing.RESTMessage, out any) error
}

func (m *mockRESTDispatcher) Dispatch(ctx context.Context, msg routing.RESTMessage, out any) error {
	if m.dispatchFunc != nil {
		return m.dispatchFunc(ctx, msg, out)
	}
	return nil
}

type noopDispatcher struct{}

func (noopDispatcher) Dispatch(context.Context, mdfilter.InteractionRequest, any) error { return nil }

func TestMapInteractionToREST(t *testing.T) {
	tests := []struct {
		name     string
		input    mdfilter.InteractionRequest
		expected routing.RESTMessage
		hasError bool
	}{
		{
			name: "spot public request",
			input: mdfilter.InteractionRequest{
				Method:        "GET",
				Path:          "/api/v3/ticker/price?symbol=BTCUSDT",
				Symbol:        "BTCUSDT",
				CorrelationID: "test-123",
			},
			expected: routing.RESTMessage{
				API:    string(rest.SpotAPI),
				Method: "GET",
				Path:   "/api/v3/ticker/price",
				Query:  map[string]string{"symbol": "BTCUSDT"},
				Body:   nil,
				Signed: false,
			},
		},
		{
			name: "futures linear private request",
			input: mdfilter.InteractionRequest{
				Method:        "POST",
				Path:          "/fapi/v1/order",
				Symbol:        "BTCUSDT",
				Payload:       map[string]any{"symbol": "BTCUSDT", "side": "BUY", "quantity": 0.001},
				CorrelationID: "order-456",
				AuthContext: &mdfilter.AuthContext{
					APIKey: "test-key",
					Secret: "test-secret",
				},
			},
			expected: routing.RESTMessage{
				API:    string(rest.LinearAPI),
				Method: "POST",
				Path:   "/fapi/v1/order",
				Query:  map[string]string{},
				Body:   []byte(`{"quantity":0.001,"side":"BUY","symbol":"BTCUSDT"}`),
				Signed: true,
			},
		},
		{
			name: "futures inverse request",
			input: mdfilter.InteractionRequest{
				Method:        "GET",
				Path:          "/dapi/v1/ticker/price?symbol=BTCUSD_PERP",
				Symbol:        "BTCUSD_PERP",
				CorrelationID: "test-789",
			},
			expected: routing.RESTMessage{
				API:    string(rest.InverseAPI),
				Method: "GET",
				Path:   "/dapi/v1/ticker/price",
				Query:  map[string]string{"symbol": "BTCUSD_PERP"},
				Body:   nil,
				Signed: false,
			},
		},
		{
			name: "account info request",
			input: mdfilter.InteractionRequest{
				Method:        "GET",
				Path:          "/api/v3/account",
				CorrelationID: "account-999",
				AuthContext: &mdfilter.AuthContext{
					APIKey: "test-key",
					Secret: "test-secret",
				},
			},
			expected: routing.RESTMessage{
				API:    string(rest.SpotAPI),
				Method: "GET",
				Path:   "/api/v3/account",
				Query:  map[string]string{},
				Body:   nil,
				Signed: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := mapInteractionToREST(tt.input)
			if tt.hasError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.API, result.API)
			assert.Equal(t, tt.expected.Method, result.Method)
			assert.Equal(t, tt.expected.Path, result.Path)
			assert.Equal(t, tt.expected.Query, result.Query)
			assert.Equal(t, tt.expected.Signed, result.Signed)

			if tt.expected.Body != nil {
				assert.JSONEq(t, string(tt.expected.Body), string(result.Body))
			} else {
				assert.Nil(t, result.Body)
			}

			assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
			if tt.expected.Signed && tt.input.AuthContext != nil {
				assert.Equal(t, tt.input.AuthContext.APIKey, result.Header.Get("X-MBX-APIKEY"))
			}
		})
	}
}

func TestRouterBridge_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("successful dispatch", func(t *testing.T) {
		var captured routing.RESTMessage
		mockRouter := &mockRESTDispatcher{
			dispatchFunc: func(ctx context.Context, msg routing.RESTMessage, out any) error {
				captured = msg
				return nil
			},
		}

		mockRouting := archmocks.NewMockRESTRouting()
		mockRouting.LegacyRESTDispatcherFn = func() routing.RESTDispatcher { return mockRouter }
		dispatcher := NewRouterBridge(mockRouting)
		interactionReq := mdfilter.InteractionRequest{
			Method:        "GET",
			Path:          "/api/v3/ticker/price?symbol=BTCUSDT",
			Symbol:        "BTCUSDT",
			CorrelationID: "test-123",
		}

		var result any
		require.NoError(t, dispatcher.Dispatch(ctx, interactionReq, &result))
		assert.Equal(t, string(rest.SpotAPI), captured.API)
		assert.Equal(t, "GET", captured.Method)
		assert.Equal(t, "/api/v3/ticker/price", captured.Path)
		assert.Equal(t, map[string]string{"symbol": "BTCUSDT"}, captured.Query)
	})

	t.Run("router not available", func(t *testing.T) {
		dispatcher := NewRouterBridge(archmocks.NewMockRESTRouting())
		var result any
		err := dispatcher.Dispatch(ctx, mdfilter.InteractionRequest{Method: "GET", Path: "/api/v3/ticker/price"}, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "REST router not available")
	})

	t.Run("payload serialization error", func(t *testing.T) {
		mockRouting := archmocks.NewMockRESTRouting()
		mockRouting.LegacyRESTDispatcherFn = func() routing.RESTDispatcher { return &mockRESTDispatcher{} }
		dispatcher := NewRouterBridge(mockRouting)
		req := mdfilter.InteractionRequest{Method: "POST", Path: "/api/v3/order", Payload: make(chan int)}
		var result any
		err := dispatcher.Dispatch(ctx, req, &result)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to serialize payload")
	})
}

func TestInferAPI(t *testing.T) {
	tests := []struct {
		path     string
		expected string
	}{
		{"/fapi/v1/ticker/price", string(rest.LinearAPI)},
		{"/dapi/v1/ticker/price", string(rest.InverseAPI)},
		{"/api/v3/ticker/price", string(rest.SpotAPI)},
		{"/api/v3/account", string(rest.SpotAPI)},
		{"/fapi/v1/account", string(rest.LinearAPI)},
		{"/dapi/v1/account", string(rest.InverseAPI)},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, inferAPI(tt.path))
		})
	}
}

func TestShouldSign(t *testing.T) {
	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		{"GET", "/api/v3/account", true},
		{"POST", "/api/v3/order", true},
		{"GET", "/api/v3/openOrders", true},
		{"GET", "/fapi/v1/account", true},
		{"POST", "/fapi/v1/order", true},
		{"GET", "/dapi/v1/account", true},
		{"POST", "/dapi/v1/order", true},
		{"GET", "/api/v3/ticker/price", false},
		{"GET", "/fapi/v1/ticker/price", false},
		{"GET", "/dapi/v1/ticker/price", false},
		{"GET", "/api/v3/exchangeInfo", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.expected, shouldSign(tt.method, tt.path))
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/api/v3/ticker/price?symbol=BTCUSDT", "/api/v3/ticker/price"},
		{"/fapi/v1/order?symbol=BTCUSDT&side=BUY", "/fapi/v1/order"},
		{"/api/v3/account", "/api/v3/account"},
		{"/dapi/v1/ticker/price", "/dapi/v1/ticker/price"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizePath(tt.input))
		})
	}
}

func TestWrapperAsLayerInterface(t *testing.T) {
	wrapper := NewWrapper(noopDispatcher{}, 0, nil)
	business := wrapper.AsLayerInterface()
	require.Implements(t, (*layers.Business)(nil), business)
}
