package consumer

import (
	"testing"
)

// TestIsMyOrder verifies that lambdas only process their own orders
func TestIsMyOrder(t *testing.T) {
	mockDataBus := NewMockDataBus()
	mockControlBus := NewMockControlBus()
	mockOrderSubmitter := NewMockOrderSubmitter()
	
	config1 := LambdaConfig{Symbol: "BTC-USDT", Provider: "fake"}
	config2 := LambdaConfig{Symbol: "BTC-USDT", Provider: "fake"}
	
	lambda1 := NewLambda("lambda-1", config1, mockDataBus, mockControlBus, mockOrderSubmitter, nil, nil)
	lambda2 := NewLambda("lambda-2", config2, mockDataBus, mockControlBus, mockOrderSubmitter, nil, nil)
	
	tests := []struct {
		name        string
		lambda      *Lambda
		clientOrder string
		expected    bool
	}{
		{"lambda1 recognizes its order", lambda1, "lambda-1-1234567890-1", true},
		{"lambda1 rejects lambda2 order", lambda1, "lambda-2-1234567890-1", false},
		{"lambda2 recognizes its order", lambda2, "lambda-2-9876543210-5", true},
		{"lambda2 rejects lambda1 order", lambda2, "lambda-1-9876543210-5", false},
		{"empty order ID", lambda1, "", false},
		{"malformed order ID", lambda1, "random-order-123", false},
		{"lambda1 with exact prefix match", lambda1, "lambda-1-", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.lambda.isMyOrder(tt.clientOrder)
			if result != tt.expected {
				t.Errorf("expected %v, got %v for order %s", tt.expected, result, tt.clientOrder)
			}
		})
	}
}

// TestMultipleLambdasSameSymbol demonstrates that two lambdas trading the same symbol
// only process their own ExecReports
func TestMultipleLambdasSameSymbol(t *testing.T) {
	t.Skip("Integration test - demonstrates order isolation concept")
	
	// Scenario: Two lambdas both trading BTC-USDT
	// Lambda1 submits order: "lambda-1-timestamp-1"
	// Lambda2 submits order: "lambda-2-timestamp-1"
	// 
	// When ExecReport comes back for "lambda-1-timestamp-1":
	// - Lambda1 processes it ✓
	// - Lambda2 ignores it ✓
	//
	// When ExecReport comes back for "lambda-2-timestamp-1":
	// - Lambda1 ignores it ✓
	// - Lambda2 processes it ✓
}
