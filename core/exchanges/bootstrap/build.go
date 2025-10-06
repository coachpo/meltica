package bootstrap

import "github.com/coachpo/meltica/config"

// ApplyOptions applies the given options to the construction parameters.
func ApplyOptions(params *ConstructionParams, opts ...Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(params)
		}
	}
}

// ConfigAdapter defines how to extract exchange-specific REST and WS configurations
// from the global config.Settings.
type ConfigAdapter interface {
	RESTConfig(settings config.Settings) interface{}
	WSConfig(settings config.Settings) interface{}
}
