package bootstrap

import "github.com/coachpo/meltica/config"

// WithConfig appends configuration options to be applied when building the exchange.
func WithConfig(options ...config.Option) Option {
	return func(params *ConstructionParams) {
		params.ConfigOpts = append(params.ConfigOpts, options...)
	}
}
