package exchanges

import (
	"fmt"
	"sync"

	"github.com/coachpo/meltica/core"
)

// ExchangeName aliases the core exchange identifier type.
type ExchangeName = core.ExchangeName

// Config holds the generic configuration required to construct an exchange provider.
type Config struct {
	APIKey    string
	APISecret string
	Params    map[string]any
}

// Option mutates a Config value used during Resolve.
type Option func(*Config)

// ExchangeFactory builds a concrete exchange provider instance.
type ExchangeFactory func(Config) (core.Exchange, error)

var (
	factoryMu sync.RWMutex
	factories = make(map[ExchangeName]ExchangeFactory)
)

// Register binds a factory to the provided exchange name.
func Register(name ExchangeName, factory ExchangeFactory) {
	if name == "" || factory == nil {
		return
	}
	factoryMu.Lock()
	defer factoryMu.Unlock()
	factories[name] = factory
}

// Resolve constructs a provider for the given exchange name, applying any options.
func Resolve(name ExchangeName, opts ...Option) (core.Exchange, error) {
	cfg := Config{Params: make(map[string]any)}
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	factoryMu.RLock()
	factory, ok := factories[name]
	factoryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("core: exchange %s not registered", name)
	}
	return factory(cfg)
}

// WithAPIKey sets the API key in the configuration.
func WithAPIKey(key string) Option {
	return func(cfg *Config) {
		cfg.APIKey = key
	}
}

// WithAPISecret sets the API secret in the configuration.
func WithAPISecret(secret string) Option {
	return func(cfg *Config) {
		cfg.APISecret = secret
	}
}

// WithParam stores an arbitrary parameter for later use by the factory.
func WithParam(key string, value any) Option {
	return func(cfg *Config) {
		if cfg.Params == nil {
			cfg.Params = make(map[string]any)
		}
		cfg.Params[key] = value
	}
}

// WithParams merges arbitrary parameters into the configuration.
func WithParams(params map[string]any) Option {
	return func(cfg *Config) {
		if len(params) == 0 {
			return
		}
		if cfg.Params == nil {
			cfg.Params = make(map[string]any, len(params))
		}
		for k, v := range params {
			cfg.Params[k] = v
		}
	}
}
