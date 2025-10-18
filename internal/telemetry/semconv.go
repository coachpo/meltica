// Package telemetry provides semantic conventions for Meltica observability.
package telemetry

import (
	"go.opentelemetry.io/otel/attribute"
)

// Semantic convention attribute keys for Meltica-specific telemetry.
// Following OpenTelemetry naming conventions: namespace.attribute_name
const (
	// Event attributes
	AttrEventType = attribute.Key("event.type")
	AttrProvider  = attribute.Key("provider")
	AttrSymbol    = attribute.Key("symbol")
	
	// Pool attributes
	AttrPoolName   = attribute.Key("pool.name")
	AttrObjectType = attribute.Key("object.type")
	
	// Environment attribute
	AttrEnvironment = attribute.Key("environment")
	
	// Error attributes
	AttrErrorType = attribute.Key("error.type")
	AttrReason    = attribute.Key("reason")
)

// Event type values
const (
	EventTypeBookSnapshot = "book_snapshot"
	EventTypeTrade        = "trade"
	EventTypeTicker       = "ticker"
	EventTypeKline        = "kline"
)

// Provider values
const (
	ProviderBinance = "binance"
	ProviderFake    = "fake"
)

// Helper functions for creating common attribute sets

// EventAttributes returns common attributes for event metrics.
func EventAttributes(environment, eventType, provider, symbol string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrEnvironment.String(environment),
		AttrEventType.String(eventType),
		AttrProvider.String(provider),
		AttrSymbol.String(symbol),
	}
}

// PoolAttributes returns common attributes for pool metrics.
func PoolAttributes(environment, poolName, objectType string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrEnvironment.String(environment),
		AttrPoolName.String(poolName),
		AttrObjectType.String(objectType),
	}
}

// ErrorAttributes returns attributes for error metrics.
func ErrorAttributes(environment, errorType, reason string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrEnvironment.String(environment),
		AttrErrorType.String(errorType),
		AttrReason.String(reason),
	}
}
