package config

import (
	"os"
	"strings"
	"time"
)

type Environment string

type Exchange string

const (
	EnvDev     Environment = "dev"
	EnvStaging Environment = "staging"
	EnvProd    Environment = "prod"
)

const (
	ExchangeBinance           Exchange = "binance"
	BinanceRESTSurfaceSpot    string   = "spot"
	BinanceRESTSurfaceLinear  string   = "linear"
	BinanceRESTSurfaceInverse string   = "inverse"
)

type Credentials struct {
	APIKey    string
	APISecret string
}

type WebsocketSettings struct {
	PublicURL  string
	PrivateURL string
}

type ExchangeSettings struct {
	REST             map[string]string
	Websocket        WebsocketSettings
	Credentials      Credentials
	HTTPTimeout      time.Duration
	HandshakeTimeout time.Duration
}

type Settings struct {
	Environment Environment
	Exchanges   map[Exchange]ExchangeSettings
}

func Default() Settings {
	return Settings{
		Environment: EnvProd,
		Exchanges: map[Exchange]ExchangeSettings{
			ExchangeBinance: {
				REST: map[string]string{
					BinanceRESTSurfaceSpot:    "https://api.binance.com",
					BinanceRESTSurfaceLinear:  "https://fapi.binance.com",
					BinanceRESTSurfaceInverse: "https://dapi.binance.com",
				},
				Websocket: WebsocketSettings{
					PublicURL:  "wss://stream.binance.com:9443/stream",
					PrivateURL: "wss://stream.binance.com:9443/ws",
				},
				HTTPTimeout:      10 * time.Second,
				HandshakeTimeout: 10 * time.Second,
			},
		},
	}
}

func FromEnv() Settings {
	cfg := Default()
	if env := strings.TrimSpace(os.Getenv("MELTICA_ENV")); env != "" {
		cfg.Environment = Environment(strings.ToLower(env))
	}

	bin := cfg.Exchanges[ExchangeBinance]
	if bin.REST == nil {
		bin.REST = make(map[string]string)
	}

	if v := strings.TrimSpace(os.Getenv("BINANCE_SPOT_BASE_URL")); v != "" {
		bin.REST[BinanceRESTSurfaceSpot] = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_LINEAR_BASE_URL")); v != "" {
		bin.REST[BinanceRESTSurfaceLinear] = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_INVERSE_BASE_URL")); v != "" {
		bin.REST[BinanceRESTSurfaceInverse] = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_PUBLIC_URL")); v != "" {
		bin.Websocket.PublicURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_PRIVATE_URL")); v != "" {
		bin.Websocket.PrivateURL = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_HTTP_TIMEOUT")); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			bin.HTTPTimeout = dur
		}
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_WS_HANDSHAKE_TIMEOUT")); v != "" {
		if dur, err := time.ParseDuration(v); err == nil {
			bin.HandshakeTimeout = dur
		}
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_API_KEY")); v != "" {
		bin.Credentials.APIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("BINANCE_API_SECRET")); v != "" {
		bin.Credentials.APISecret = v
	}

	cfg.Exchanges[ExchangeBinance] = bin
	return cfg
}

type Option func(*Settings)

func Apply(base Settings, opts ...Option) Settings {
	cfg := base.clone()
	for _, opt := range opts {
		if opt != nil {
			opt(&cfg)
		}
	}
	return cfg
}

func (s Settings) Exchange(name Exchange) (ExchangeSettings, bool) {
	if len(s.Exchanges) == 0 {
		return ExchangeSettings{}, false
	}
	key := Exchange(normalizeExchangeName(string(name)))
	cfg, ok := s.Exchanges[key]
	if !ok {
		return ExchangeSettings{}, false
	}
	return cloneExchangeSettings(cfg), true
}

func DefaultExchangeSettings(name Exchange) (ExchangeSettings, bool) {
	def := Default()
	cfg, ok := def.Exchanges[Exchange(normalizeExchangeName(string(name)))]
	if !ok {
		return ExchangeSettings{}, false
	}
	return cloneExchangeSettings(cfg), true
}

func WithEnvironment(env Environment) Option {
	return func(s *Settings) {
		if env != "" {
			s.Environment = env
		}
	}
}

func WithExchangeRESTEndpoint(exchange, surface, baseURL string) Option {
	surface = strings.TrimSpace(surface)
	baseURL = strings.TrimSpace(baseURL)
	return mutateExchangeOption(exchange, func(es *ExchangeSettings) {
		if surface == "" || baseURL == "" {
			return
		}
		es.REST[surface] = baseURL
	})
}

func WithExchangeWebsocketEndpoints(exchange, public, private string, handshake time.Duration) Option {
	public = strings.TrimSpace(public)
	private = strings.TrimSpace(private)
	return mutateExchangeOption(exchange, func(es *ExchangeSettings) {
		if public != "" {
			es.Websocket.PublicURL = public
		}
		if private != "" {
			es.Websocket.PrivateURL = private
		}
		if handshake > 0 {
			es.HandshakeTimeout = handshake
		}
	})
}

func WithExchangeHTTPTimeout(exchange string, timeout time.Duration) Option {
	return mutateExchangeOption(exchange, func(es *ExchangeSettings) {
		if timeout > 0 {
			es.HTTPTimeout = timeout
		}
	})
}

func WithExchangeCredentials(exchange, key, secret string) Option {
	key = strings.TrimSpace(key)
	secret = strings.TrimSpace(secret)
	return mutateExchangeOption(exchange, func(es *ExchangeSettings) {
		if key != "" {
			es.Credentials.APIKey = key
		}
		if secret != "" {
			es.Credentials.APISecret = secret
		}
	})
}

func WithBinanceRESTEndpoints(spot, linear, inverse string) Option {
	spot = strings.TrimSpace(spot)
	linear = strings.TrimSpace(linear)
	inverse = strings.TrimSpace(inverse)
	return mutateExchangeOption(string(ExchangeBinance), func(es *ExchangeSettings) {
		if spot != "" {
			es.REST[BinanceRESTSurfaceSpot] = spot
		}
		if linear != "" {
			es.REST[BinanceRESTSurfaceLinear] = linear
		}
		if inverse != "" {
			es.REST[BinanceRESTSurfaceInverse] = inverse
		}
	})
}

func WithBinanceWebsocketEndpoints(public, private string, handshake time.Duration) Option {
	return WithExchangeWebsocketEndpoints(string(ExchangeBinance), public, private, handshake)
}

func WithBinanceHTTPTimeout(timeout time.Duration) Option {
	return WithExchangeHTTPTimeout(string(ExchangeBinance), timeout)
}

func WithBinanceAPI(key, secret string) Option {
	return WithExchangeCredentials(string(ExchangeBinance), key, secret)
}

func mutateExchangeOption(exchange string, fn func(*ExchangeSettings)) Option {
	key := Exchange(normalizeExchangeName(exchange))
	if string(key) == "" || fn == nil {
		return func(*Settings) {}
	}
	return func(s *Settings) {
		if s.Exchanges == nil {
			s.Exchanges = make(map[Exchange]ExchangeSettings)
		}
		cfg, ok := s.Exchanges[key]
		if !ok {
			cfg = ExchangeSettings{}
		}
		cfg = cloneExchangeSettings(cfg)
		fn(&cfg)
		s.Exchanges[key] = cfg
	}
}

func (s Settings) clone() Settings {
	clone := Settings{
		Environment: s.Environment,
		Exchanges:   cloneExchangeSettingsMap(s.Exchanges),
	}
	return clone
}

func cloneExchangeSettingsMap(src map[Exchange]ExchangeSettings) map[Exchange]ExchangeSettings {
	if len(src) == 0 {
		return make(map[Exchange]ExchangeSettings)
	}
	out := make(map[Exchange]ExchangeSettings, len(src))
	for k, v := range src {
		out[k] = cloneExchangeSettings(v)
	}
	return out
}

func cloneExchangeSettings(cfg ExchangeSettings) ExchangeSettings {
	out := cfg
	if cfg.REST != nil {
		out.REST = make(map[string]string, len(cfg.REST))
		for k, v := range cfg.REST {
			out.REST[k] = v
		}
	} else {
		out.REST = make(map[string]string)
	}
	return out
}

func normalizeExchangeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
