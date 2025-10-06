package bootstrap

import (
	"github.com/coachpo/meltica/config"
	coretransport "github.com/coachpo/meltica/core/transport"
)

// Option is a functional option for configuring exchange construction parameters.
type Option func(*ConstructionParams)

// ConstructionParams holds all configuration needed to construct an exchange.
type ConstructionParams struct {
	ConfigOpts []config.Option
	Transports TransportFactories
	Routers    RouterFactories
}

// TransportFactories holds factory functions for creating transport clients.
type TransportFactories struct {
	NewRESTClient func(interface{}) coretransport.RESTClient
	NewWSClient   func(interface{}) coretransport.StreamClient
}

// RouterFactories holds factory functions for creating routers.
type RouterFactories struct {
	NewRESTRouter func(coretransport.RESTClient) interface{}
	NewWSRouter   func(coretransport.StreamClient, interface{}) interface{}
}

// NewConstructionParams creates a new ConstructionParams with zero values.
// Exchange implementations should seed this with their specific defaults.
func NewConstructionParams() *ConstructionParams {
	return &ConstructionParams{
		ConfigOpts: make([]config.Option, 0),
	}
}
