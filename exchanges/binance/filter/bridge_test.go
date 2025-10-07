package filter

import (
	"context"
	"testing"

	"github.com/coachpo/meltica/exchanges/binance/infra/rest"
	"github.com/coachpo/meltica/exchanges/shared/routing"
	mdfilter "github.com/coachpo/meltica/pipeline"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRESTDispatcher implements routing.RESTDispatcher for testing
type mockRESTDispatcher struct {
	dispatchFunc func(ctx context.Context, msg routing.RESTMessage, out any) error
}

func (m *mockRESTDispatcher) Dispatch(ctx context.Context, msg routing.RESTMessage, out any) error {
	if m.dispatchFunc != nil {
		return m.dispatchFunc(ctx, msg, out)
	}
	return nil
}

func TestRESTBridge_MapInteractionToREST(t *testing.T) {
	bridge := NewRESTBridge(nil)

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
				Payload:       map[string]interface{}{"symbol": "BTCUSDT", "side": "BUY", "quantity": 0.001},
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
			result, err := bridge.mapInteractionToREST(tt.input)
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

			// Check body separately since JSON marshaling order might vary
			if tt.expected.Body != nil {
				assert.JSONEq(t, string(tt.expected.Body), string(result.Body))
			} else {
				assert.Nil(t, result.Body)
			}

			// Check headers
			assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
			if tt.expected.Signed && tt.input.AuthContext != nil {
				assert.Equal(t, tt.input.AuthContext.APIKey, result.Header.Get("X-MBX-APIKEY"))
			}
		})
	}
}

func TestRESTBridge_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("successful dispatch", func(t *testing.T) {
		var capturedMsg routing.RESTMessage
		mockRouter := &mockRESTDispatcher{
			dispatchFunc: func(ctx context.Context, msg routing.RESTMessage, out any) error {
				capturedMsg = msg
				return nil
			},
		}

		bridge := NewRESTBridge(mockRouter)
		interactionReq := mdfilter.InteractionRequest{
			Method:        "GET",
			Path:          "/api/v3/ticker/price?symbol=BTCUSDT",
			Symbol:        "BTCUSDT",
			CorrelationID: "test-123",
		}

		var result interface{}
		err := bridge.Dispatch(ctx, interactionReq, &result)

		require.NoError(t, err)
		assert.Equal(t, string(rest.SpotAPI), capturedMsg.API)
		assert.Equal(t, "GET", capturedMsg.Method)
		assert.Equal(t, "/api/v3/ticker/price", capturedMsg.Path)
		assert.Equal(t, map[string]string{"symbol": "BTCUSDT"}, capturedMsg.Query)
	})

	t.Run("router not available", func(t *testing.T) {
		bridge := NewRESTBridge(nil)
		interactionReq := mdfilter.InteractionRequest{
			Method: "GET",
			Path:   "/api/v3/ticker/price",
		}

		var result interface{}
		err := bridge.Dispatch(ctx, interactionReq, &result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "REST router not available")
	})

	t.Run("invalid message type", func(t *testing.T) {
		mockRouter := &mockRESTDispatcher{}
		bridge := NewRESTBridge(mockRouter)

		var result interface{}
		err := bridge.Dispatch(ctx, "invalid-type", &result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "expected InteractionRequest")
	})

	t.Run("payload serialization error", func(t *testing.T) {
		mockRouter := &mockRESTDispatcher{}
		bridge := NewRESTBridge(mockRouter)

		// Create an invalid payload that can't be JSON marshaled
		interactionReq := mdfilter.InteractionRequest{
			Method:        "POST",
			Path:          "/api/v3/order",
			Payload:       make(chan int), // Channels can't be JSON marshaled
			CorrelationID: "test-123",
		}

		var result interface{}
		err := bridge.Dispatch(ctx, interactionReq, &result)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to serialize payload")
	})
}

func TestRESTBridge_InferAPI(t *testing.T) {
	bridge := NewRESTBridge(nil)

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
			result := bridge.inferAPI(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRESTBridge_ShouldSign(t *testing.T) {
	bridge := NewRESTBridge(nil)

	tests := []struct {
		method   string
		path     string
		expected bool
	}{
		// Private endpoints that should be signed
		{"GET", "/api/v3/account", true},
		{"POST", "/api/v3/order", true},
		{"GET", "/api/v3/openOrders", true},
		{"GET", "/fapi/v1/account", true},
		{"POST", "/fapi/v1/order", true},
		{"GET", "/dapi/v1/account", true},
		{"POST", "/dapi/v1/order", true},

		// Public endpoints that should not be signed
		{"GET", "/api/v3/ticker/price", false},
		{"GET", "/fapi/v1/ticker/price", false},
		{"GET", "/dapi/v1/ticker/price", false},
		{"GET", "/api/v3/exchangeInfo", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := bridge.shouldSign(tt.method, tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRESTBridge_NormalizePath(t *testing.T) {
	bridge := NewRESTBridge(nil)

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
			result := bridge.normalizePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
